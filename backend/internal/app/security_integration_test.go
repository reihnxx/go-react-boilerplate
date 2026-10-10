//go:build integration

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
	ts "github.com/reihnxx/go-react-boilerplate/backend/internal/testsupport"
)

func TestAPISecurityHeaders(t *testing.T) {
	e := ts.NewEnv(t, nil)
	req, _ := http.NewRequest(http.MethodGet, e.Server.URL+"/api/v1/auth/options", nil)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"Cache-Control":           "no-store",
	}
	for h, v := range want {
		if got := res.Header.Get(h); got != v {
			t.Errorf("%s = %q, want %q", h, got, v)
		}
	}
	if res.Header.Get("X-Request-Id") == "" {
		t.Error("missing X-Request-Id")
	}
	// No CORS: browsers must not let other origins read API responses.
	if res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("unexpected CORS header")
	}
}

func TestRequestBodyLimit(t *testing.T) {
	e := ts.NewEnv(t, nil)
	big := `{"login":"x","password":"` + strings.Repeat("a", 2<<20) + `"}`
	res, err := http.Post(e.Server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(big))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", res.StatusCode)
	}
}

func TestOversizedPasswordIsRejectedAndCounted(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("panjang", "user")
	c := e.Client()
	c.Login("panjang", ts.Password("panjang")+strings.Repeat("x", 2000)).ExpectError(t, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	if n := e.Count(`SELECT attempts FROM login_attempts WHERE key LIKE 'login|panjang|%'`); n != 1 {
		t.Fatalf("oversized attempt not counted: %d", n)
	}
}

func TestPasswordSprayingIsLimitedPerIP(t *testing.T) {
	e := ts.NewEnv(t, func(c *config.Config) { c.AuthIPMaxAttempts = 6 })
	e.CreateUser("korban", "user")
	c := e.Client()
	for i, name := range []string{"a1", "b2", "c3", "d4", "e5"} {
		res := c.Login(name+"-tidak-ada", "Tebakan123")
		if res.Status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d", i+1, res.Status)
		}
	}
	// Sixth failure from the same IP (different username again) locks the IP.
	c.Login("f6-tidak-ada", "Tebakan123").ExpectError(t, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
	// Even correct credentials for another account are refused while locked.
	c.Login("korban", ts.Password("korban")).ExpectError(t, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
}

func TestSuccessfulLoginDoesNotResetIPCounter(t *testing.T) {
	e := ts.NewEnv(t, func(c *config.Config) { c.AuthIPMaxAttempts = 6 })
	e.CreateUser("milik", "user")
	c := e.Client()
	for _, name := range []string{"x1", "x2", "x3"} {
		c.Login(name, "Tebakan123").Expect(t, http.StatusUnauthorized)
	}
	e.Client().MustLogin("milik") // attacker's own valid account
	for _, name := range []string{"x4", "x5"} {
		c.Login(name, "Tebakan123").Expect(t, http.StatusUnauthorized)
	}
	c.Login("x6", "Tebakan123").ExpectError(t, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
}

func TestForgotPasswordThrottleAndHashedToken(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("lupa2", "user")
	c := e.Client()
	for i := 1; i <= 5; i++ {
		c.Do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "lupa2@example.com"}).Expect(t, http.StatusOK)
	}
	c.Do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "lupa2@example.com"}).
		ExpectError(t, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")

	msg := e.Mail.WaitFor(t, 5)
	link := strings.Fields(msg.Text[strings.Index(msg.Text, ts.Origin):])[0]
	u, _ := url.Parse(link)
	token := u.Query().Get("token")
	sum := sha256.Sum256([]byte(token))
	if n := e.Count(`SELECT count(*) FROM password_reset_tokens WHERE token_hash = convert_to($1, 'UTF8')`, token); n != 0 {
		t.Fatal("raw reset token stored")
	}
	if n := e.Count(`SELECT count(*) FROM password_reset_tokens WHERE token_hash = $1`, sum[:]); n != 1 {
		t.Fatal("reset token hash not found")
	}
	// Only the latest token is valid.
	if n := e.Count(`SELECT count(*) FROM password_reset_tokens`); n != 1 {
		t.Fatalf("old reset tokens not replaced: %d", n)
	}
}

func TestForwardingHeadersAreTrustedOnlyFromProxy(t *testing.T) {
	failWith := func(e *ts.Env, headers map[string]string) {
		body := bytes.NewBufferString(`{"login":"spoof","password":"Tebakan123"}`)
		req, _ := http.NewRequest(http.MethodPost, e.Server.URL+"/api/v1/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	keys := func(e *ts.Env) []string {
		rows, err := e.Pool.Query(context.Background(), `SELECT key FROM login_attempts WHERE key LIKE 'login|spoof|%' ORDER BY key`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var k string
			_ = rows.Scan(&k)
			out = append(out, k)
		}
		return out
	}

	// Without TRUST_PROXY every forwarding header is ignored.
	direct := ts.NewEnv(t, nil)
	failWith(direct, map[string]string{"X-Real-IP": "203.0.113.7", "X-Forwarded-For": "198.51.100.1", "True-Client-IP": "192.0.2.1"})
	if k := keys(direct); len(k) != 1 || !strings.HasSuffix(k[0], "|127.0.0.1") {
		t.Fatalf("direct: keys = %v", k)
	}

	// Behind the proxy only X-Real-IP (overwritten by nginx) is used.
	proxied := ts.NewEnv(t, func(c *config.Config) { c.TrustProxy = true })
	failWith(proxied, map[string]string{"X-Real-IP": "203.0.113.7", "X-Forwarded-For": "198.51.100.1", "True-Client-IP": "192.0.2.1"})
	failWith(proxied, map[string]string{"X-Forwarded-For": "198.51.100.2", "True-Client-IP": "192.0.2.2"})
	failWith(proxied, map[string]string{"X-Real-IP": "not-an-ip"})
	got := keys(proxied)
	want := []string{"login|spoof|127.0.0.1", "login|spoof|203.0.113.7"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("proxied: keys = %v, want %v", got, want)
	}
}

func TestConcurrentDeletionCannotRemoveAllSuperAdmins(t *testing.T) {
	for round := 0; round < 5; round++ {
		e := ts.NewEnv(t, nil)
		idA := e.CreateUser("admina", "super-admin")
		idB := e.CreateUser("adminb", "super-admin")
		a, b := e.Client(), e.Client()
		a.MustLogin("admina")
		b.MustLogin("adminb")

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); a.Do(http.MethodDelete, "/users/"+idB.String(), nil) }()
		go func() { defer wg.Done(); b.Do(http.MethodDelete, "/users/"+idA.String(), nil) }()
		wg.Wait()

		n := e.Count(`SELECT count(*) FROM users u JOIN user_roles ur ON ur.user_id = u.id
			JOIN roles r ON r.id = ur.role_id WHERE r.is_super_admin AND u.is_active AND u.deleted_at IS NULL`)
		if n < 1 {
			t.Fatalf("round %d: no active super-admin left", round)
		}
	}
}

func TestDeletedUserSessionsAreRevoked(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	victimID := e.CreateUser("hapusaku", "user")
	admin, victim := e.Client(), e.Client()
	admin.MustLogin("admin")
	victim.MustLogin("hapusaku")
	admin.Do(http.MethodDelete, "/users/"+victimID.String(), nil).Expect(t, http.StatusNoContent)
	victim.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func TestAuditLogContainsNoSecrets(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	c := e.Client()
	c.MustLogin("admin")
	res := c.Do(http.MethodPost, "/users", map[string]any{
		"name": "Rahasia", "username": "rahasia", "email": "rahasia@example.com",
		"password": "SangatRahasia123", "password_confirmation": "SangatRahasia123",
	}).Expect(t, http.StatusCreated)
	id := res.String("data.id")
	c.Do(http.MethodPut, "/users/"+id+"/password", map[string]any{"password": "LebihRahasia456", "password_confirmation": "LebihRahasia456"}).Expect(t, http.StatusNoContent)
	c.Do(http.MethodPut, "/profile/password", map[string]any{"current_password": ts.Password("admin"), "password": "AdminBaru789", "password_confirmation": "AdminBaru789"}).Expect(t, http.StatusNoContent)

	for _, secret := range []string{"SangatRahasia123", "LebihRahasia456", "AdminBaru789", ts.Password("admin"), "argon2id", "csrf"} {
		if n := e.Count(`SELECT count(*) FROM audit_logs WHERE metadata::text ILIKE '%' || $1 || '%'`, secret); n != 0 {
			t.Errorf("audit log contains %q", secret)
		}
	}
}

func TestInjectionAttemptsAreInert(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	c := e.Client()
	c.MustLogin("admin")
	for _, q := range []string{"' OR 1=1 --", "%", "_", `\`, "admin'; DROP TABLE users; --"} {
		res := c.Do(http.MethodGet, "/users?q="+url.QueryEscape(q), nil).Expect(t, http.StatusOK)
		if res.Len("data") != 0 {
			t.Errorf("q=%q matched %d users", q, res.Len("data"))
		}
	}
	c.Do(http.MethodGet, "/users?sort="+url.QueryEscape("name; DROP TABLE users"), nil).ExpectError(t, http.StatusBadRequest, "BAD_REQUEST")
	c.Do(http.MethodGet, "/users?order="+url.QueryEscape("desc, (SELECT 1)"), nil).ExpectError(t, http.StatusBadRequest, "BAD_REQUEST")
	if n := e.Count(`SELECT count(*) FROM users`); n != 1 {
		t.Fatalf("users table changed: %d", n)
	}
}

func TestPurgeExpiredRemovesStaleRows(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("lama", "user")
	e.Client().MustLogin("lama")
	e.Client().MustLogin("lama")
	e.Exec(`UPDATE sessions SET expires_at = now() - interval '1 minute' WHERE token_hash = (SELECT token_hash FROM sessions LIMIT 1)`)
	e.Exec(`INSERT INTO login_attempts (key, attempts, window_started_at) VALUES ('login|old|1.1.1.1', 3, now() - interval '2 days'), ('login|new|1.1.1.1', 1, now())`)
	e.Exec(`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at)
		SELECT '\x01', id, now() - interval '2 days' FROM users WHERE username = 'lama'`)

	if err := e.App.Auth.Store().PurgeExpired(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := e.Count(`SELECT count(*) FROM sessions`); n != 1 {
		t.Fatalf("sessions left = %d, want 1", n)
	}
	if n := e.Count(`SELECT count(*) FROM login_attempts`); n != 1 {
		t.Fatalf("attempts left = %d, want 1", n)
	}
	if n := e.Count(`SELECT count(*) FROM password_reset_tokens`); n != 0 {
		t.Fatalf("reset tokens left = %d, want 0", n)
	}
}
