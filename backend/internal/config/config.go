// Package config loads application configuration from environment variables.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

type Config struct {
	Env            string
	AppName        string
	HTTPAddr       string
	BaseURL        string
	AllowedOrigins []string
	TrustProxy     bool

	DatabaseURL    string
	DBMaxConns     int32
	MigrateOnStart bool

	SessionIdle     time.Duration
	SessionMaxLife  time.Duration
	CookieSecure    bool
	AuthMaxAttempts int
	// AuthIPMaxAttempts limits failed logins per IP across all usernames
	// (password spraying); counted in the same AUTH_LOCKOUT window.
	AuthIPMaxAttempts int
	AuthLockout       time.Duration

	RegistrationEnabled bool
	DefaultRoleSlug     string

	RequestTimeout time.Duration
	LogLevel       slog.Level
	LogFormat      string

	Mail MailConfig
}

type MailConfig struct {
	Driver   string // "log" or "smtp"
	Host     string
	Port     int
	User     string
	Password string
	From     string
	// RedactLog hides message bodies (e.g. reset links) in the "log" driver.
	// Set automatically in production.
	RedactLog bool
}

func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Load reads the configuration from the process environment and validates it.
func Load() (Config, error) {
	e := &reader{}
	c := Config{
		Env:            e.str("APP_ENV", EnvDevelopment),
		AppName:        e.str("APP_NAME", "Admin Panel"),
		HTTPAddr:       e.str("HTTP_ADDR", ":8080"),
		BaseURL:        strings.TrimRight(e.str("APP_BASE_URL", "http://localhost:5173"), "/"),
		AllowedOrigins: e.list("APP_ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustProxy:     e.bool("TRUST_PROXY", false),

		DatabaseURL:    e.required("DATABASE_URL"),
		DBMaxConns:     int32(e.int("DB_MAX_CONNS", 10)),
		MigrateOnStart: e.bool("MIGRATE_ON_START", false),

		SessionIdle:       e.duration("SESSION_IDLE_TIMEOUT", 120*time.Minute),
		SessionMaxLife:    e.duration("SESSION_MAX_LIFETIME", 7*24*time.Hour),
		AuthMaxAttempts:   e.int("AUTH_MAX_ATTEMPTS", 5),
		AuthIPMaxAttempts: e.int("AUTH_IP_MAX_ATTEMPTS", 20),
		AuthLockout:       e.duration("AUTH_LOCKOUT", 60*time.Second),

		RegistrationEnabled: e.bool("AUTH_REGISTRATION_ENABLED", false),
		DefaultRoleSlug:     e.str("AUTH_DEFAULT_ROLE", "user"),

		RequestTimeout: e.duration("REQUEST_TIMEOUT", 30*time.Second),

		Mail: MailConfig{
			Driver:   e.str("MAIL_DRIVER", "log"),
			Host:     e.str("SMTP_HOST", ""),
			Port:     e.int("SMTP_PORT", 587),
			User:     e.str("SMTP_USER", ""),
			Password: e.str("SMTP_PASSWORD", ""),
			From:     e.str("MAIL_FROM", ""),
		},
	}

	c.CookieSecure = e.bool("COOKIE_SECURE", c.Env == EnvProduction)
	c.Mail.RedactLog = c.Env == EnvProduction

	defaultFormat := "text"
	if c.Env == EnvProduction {
		defaultFormat = "json"
	}
	c.LogFormat = e.str("LOG_FORMAT", defaultFormat)
	c.LogLevel = e.level("LOG_LEVEL", slog.LevelInfo)

	switch c.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		e.fail("APP_ENV must be one of development, test, production")
	}
	if c.LogFormat != "text" && c.LogFormat != "json" {
		e.fail("LOG_FORMAT must be text or json")
	}
	if c.DBMaxConns < 1 {
		e.fail("DB_MAX_CONNS must be >= 1")
	}
	if c.AuthMaxAttempts < 1 {
		e.fail("AUTH_MAX_ATTEMPTS must be >= 1")
	}
	if c.AuthIPMaxAttempts < c.AuthMaxAttempts {
		e.fail("AUTH_IP_MAX_ATTEMPTS must be >= AUTH_MAX_ATTEMPTS")
	}
	if c.SessionIdle <= 0 || c.SessionMaxLife <= 0 || c.SessionIdle > c.SessionMaxLife {
		e.fail("SESSION_IDLE_TIMEOUT must be > 0 and <= SESSION_MAX_LIFETIME")
	}
	switch c.Mail.Driver {
	case "log":
	case "smtp":
		if c.Mail.Host == "" || c.Mail.From == "" {
			e.fail("SMTP_HOST and MAIL_FROM are required when MAIL_DRIVER=smtp")
		}
	default:
		e.fail("MAIL_DRIVER must be log or smtp")
	}
	if c.Env == EnvProduction && !c.CookieSecure {
		e.fail("COOKIE_SECURE must not be false in production")
	}

	if len(e.errs) > 0 {
		return Config{}, errors.New("invalid configuration:\n  - " + strings.Join(e.errs, "\n  - "))
	}
	return c, nil
}

// LoadDotEnv sets variables from a KEY=VALUE file without overriding variables
// that are already present in the environment. A missing file is not an error.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")
		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

type reader struct{ errs []string }

func (r *reader) fail(msg string) { r.errs = append(r.errs, msg) }

func (r *reader) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func (r *reader) required(key string) string {
	v := r.str(key, "")
	if v == "" {
		r.fail(key + " is required")
	}
	return v
}

func (r *reader) int(key string, def int) int {
	v := r.str(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		r.fail(key + " must be an integer")
		return def
	}
	return n
}

func (r *reader) bool(key string, def bool) bool {
	v := r.str(key, "")
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.fail(key + " must be true or false")
		return def
	}
	return b
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v := r.str(key, "")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key + " must be a duration such as 30s, 15m, 2h")
		return def
	}
	return d
}

func (r *reader) list(key, def string) []string {
	var out []string
	for _, part := range strings.Split(r.str(key, def), ",") {
		if p := strings.TrimRight(strings.TrimSpace(part), "/"); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (r *reader) level(key string, def slog.Level) slog.Level {
	v := r.str(key, "")
	if v == "" {
		return def
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(v)); err != nil {
		r.fail(key + " must be debug, info, warn or error")
		return def
	}
	return l
}
