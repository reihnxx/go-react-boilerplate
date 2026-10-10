package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("APP_ENV", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != EnvDevelopment || c.CookieSecure || c.SessionIdle != 120*time.Minute || c.AuthMaxAttempts != 5 || c.RegistrationEnabled {
		t.Fatalf("unexpected defaults %+v", c)
	}
}

func TestLoadProductionDefaultsSecureCookie(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("APP_ENV", "production")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.CookieSecure || c.LogFormat != "json" {
		t.Fatalf("production must default to secure cookies and json logs: %+v", c)
	}
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("insecure cookies must be rejected in production")
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("AUTH_MAX_ATTEMPTS", "many")
	t.Setenv("SESSION_IDLE_TIMEOUT", "1x")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"DATABASE_URL", "APP_ENV", "AUTH_MAX_ATTEMPTS", "SESSION_IDLE_TIMEOUT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\nDOTENV_A=from-file\nexport DOTENV_B=\"quoted value\"\nDOTENV_C=file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTENV_C", "from-env")
	os.Unsetenv("DOTENV_A")
	os.Unsetenv("DOTENV_B")
	t.Cleanup(func() { os.Unsetenv("DOTENV_A"); os.Unsetenv("DOTENV_B") })

	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("DOTENV_A") != "from-file" || os.Getenv("DOTENV_B") != "quoted value" || os.Getenv("DOTENV_C") != "from-env" {
		t.Fatalf("got A=%q B=%q C=%q", os.Getenv("DOTENV_A"), os.Getenv("DOTENV_B"), os.Getenv("DOTENV_C"))
	}
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
}
