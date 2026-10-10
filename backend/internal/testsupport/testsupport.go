//go:build integration

// Package testsupport provides helpers for integration tests that run the
// full HTTP stack against a real PostgreSQL database. Each test gets its own
// schema, migrated from scratch and dropped afterwards.
package testsupport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/app"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/mailer"
)

// Origin is the allowed SPA origin in test configuration.
const Origin = "http://app.test"

// NewDB returns a pool bound to a fresh, fully migrated schema. The test is
// skipped when TEST_DATABASE_URL is not set.
func NewDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	admin, err := database.Connect(ctx, url, 2, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	schema := "test_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	pool, err := database.Connect(ctx, url, 10, map[string]string{"search_path": schema})
	if err != nil {
		t.Fatalf("connect schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	if err := database.Migrate(ctx, pool, "up", io.Discard); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func Config() config.Config {
	return config.Config{
		Env: config.EnvTest, AppName: "Test", HTTPAddr: ":0", BaseURL: Origin,
		AllowedOrigins: []string{Origin}, DBMaxConns: 10,
		SessionIdle: 2 * time.Hour, SessionMaxLife: 24 * time.Hour,
		AuthMaxAttempts: 5, AuthIPMaxAttempts: 20, AuthLockout: time.Minute,
		DefaultRoleSlug: "user", RequestTimeout: 10 * time.Second,
		LogLevel: slog.LevelError, LogFormat: "text",
		Mail: config.MailConfig{Driver: "log"},
	}
}

// CaptureMailer stores sent messages.
type CaptureMailer struct {
	mu   sync.Mutex
	Sent []mailer.Message
}

func (m *CaptureMailer) Send(_ context.Context, msg mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, msg)
	return nil
}

// WaitFor blocks until at least n messages were sent (mail is sent
// asynchronously) or fails the test after two seconds.
func (m *CaptureMailer) WaitFor(t *testing.T, n int) mailer.Message {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		if len(m.Sent) >= n {
			msg := m.Sent[n-1]
			m.mu.Unlock()
			return msg
		}
		m.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d mail(s), got %d", n, m.Count())
	return mailer.Message{}
}

func (m *CaptureMailer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Sent)
}

func (m *CaptureMailer) Last() (mailer.Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Sent) == 0 {
		return mailer.Message{}, false
	}
	return m.Sent[len(m.Sent)-1], true
}

type Env struct {
	T      *testing.T
	Pool   *pgxpool.Pool
	App    *app.App
	Server *httptest.Server
	Mail   *CaptureMailer
}

// NewEnv starts the full application on a test server. mutate may adjust
// the configuration.
func NewEnv(t *testing.T, mutate func(*config.Config)) *Env {
	t.Helper()
	pool := NewDB(t)
	cfg := Config()
	if mutate != nil {
		mutate(&cfg)
	}
	mail := &CaptureMailer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := app.New(cfg, pool, logger, mail)
	srv := httptest.NewServer(a.Handler)
	t.Cleanup(srv.Close)
	return &Env{T: t, Pool: pool, App: a, Server: srv, Mail: mail}
}

// Password returns the deterministic test password for username.
func Password(username string) string { return "Passw0rd-" + username }

// CreateUser creates an active user with the given role slugs.
func (e *Env) CreateUser(username string, roleSlugs ...string) uuid.UUID {
	e.T.Helper()
	id, err := e.App.Users.CreateWithRoleSlugs(context.Background(), "User "+username, username,
		username+"@example.com", Password(username), true, roleSlugs...)
	if err != nil {
		e.T.Fatalf("create user %s: %v", username, err)
	}
	return id
}

// CreateRole creates a role with explicit grants: menu slug → actions.
func (e *Env) CreateRole(slug string, grants map[string][]string) uuid.UUID {
	e.T.Helper()
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7())
	e.Exec(`INSERT INTO roles (id, name, slug) VALUES ($1, $2, $3)`, id, strings.ToUpper(slug[:1])+slug[1:], slug)
	for menu, actions := range grants {
		for _, a := range actions {
			if _, err := e.Pool.Exec(ctx, `
				INSERT INTO role_permissions (role_id, menu_id, action)
				SELECT $1, id, $3 FROM menus WHERE slug = $2 AND deleted_at IS NULL`, id, menu, a); err != nil {
				e.T.Fatalf("grant %s %s: %v", menu, a, err)
			}
		}
	}
	return id
}

func (e *Env) RoleID(slug string) uuid.UUID {
	e.T.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(), `SELECT id FROM roles WHERE slug = $1 AND deleted_at IS NULL`, slug).Scan(&id); err != nil {
		e.T.Fatalf("role %s: %v", slug, err)
	}
	return id
}

func (e *Env) MenuID(slug string) uuid.UUID {
	e.T.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(), `SELECT id FROM menus WHERE slug = $1 AND deleted_at IS NULL`, slug).Scan(&id); err != nil {
		e.T.Fatalf("menu %s: %v", slug, err)
	}
	return id
}

func (e *Env) Exec(sql string, args ...any) {
	e.T.Helper()
	if _, err := e.Pool.Exec(context.Background(), sql, args...); err != nil {
		e.T.Fatalf("exec %q: %v", sql, err)
	}
}

func (e *Env) Count(sql string, args ...any) int {
	e.T.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.T.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// Client is a browser-like client with a cookie jar and CSRF handling.
type Client struct {
	e    *Env
	http *http.Client
	CSRF string
	// Origin header sent with every request ("" = none).
	Origin string
}

func (e *Env) Client() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{e: e, http: &http.Client{Jar: jar}, Origin: Origin}
}

type Response struct {
	Status int
	Header http.Header
	Body   map[string]any
	Raw    []byte
}

// Do sends a JSON request. body may be nil.
func (c *Client) Do(method, path string, body any) *Response {
	c.e.T.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.e.T.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.e.Server.URL+"/api/v1"+path, r)
	if err != nil {
		c.e.T.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Origin != "" {
		req.Header.Set("Origin", c.Origin)
	}
	if c.CSRF != "" {
		req.Header.Set("X-CSRF-Token", c.CSRF)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.T.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := &Response{Status: res.StatusCode, Header: res.Header, Raw: raw}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

// Login posts credentials and stores the CSRF token on success.
func (c *Client) Login(login, password string) *Response {
	c.e.T.Helper()
	res := c.Do(http.MethodPost, "/auth/login", map[string]string{"login": login, "password": password})
	if res.Status == http.StatusOK {
		c.CSRF = res.String("data.csrf_token")
	}
	return res
}

// MustLogin logs in as username (with the test password) or fails the test.
func (c *Client) MustLogin(username string) *Response {
	c.e.T.Helper()
	res := c.Login(username, Password(username))
	if res.Status != http.StatusOK {
		c.e.T.Fatalf("login %s: %d %s", username, res.Status, res.Raw)
	}
	return res
}

// Get walks a dotted path ("data.user.id", "data.items.0.slug").
func (r *Response) Get(path string) any {
	var cur any = r.Body
	for _, part := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[part]
		case []any:
			i := 0
			for _, ch := range part {
				i = i*10 + int(ch-'0')
			}
			if i >= len(v) {
				return nil
			}
			cur = v[i]
		default:
			return nil
		}
	}
	return cur
}

func (r *Response) String(path string) string {
	s, _ := r.Get(path).(string)
	return s
}

func (r *Response) Bool(path string) bool {
	b, _ := r.Get(path).(bool)
	return b
}

func (r *Response) Len(path string) int {
	switch v := r.Get(path).(type) {
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	}
	return -1
}

func (r *Response) ErrorCode() string { return r.String("error.code") }

// Expect fails the test when the status differs.
func (r *Response) Expect(t *testing.T, status int) *Response {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, status, r.Raw)
	}
	return r
}

// ExpectError checks status and error code.
func (r *Response) ExpectError(t *testing.T, status int, code string) *Response {
	t.Helper()
	r.Expect(t, status)
	if r.ErrorCode() != code {
		t.Fatalf("error code = %q, want %q; body: %s", r.ErrorCode(), code, r.Raw)
	}
	return r
}
