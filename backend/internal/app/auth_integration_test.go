//go:build integration

package app_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	ts "github.com/reihnxx/go-react-boilerplate/backend/internal/testsupport"
)

func TestHealthEndpoints(t *testing.T) {
	e := ts.NewEnv(t, nil)
	for _, path := range []string{"/healthz", "/readyz"} {
		res, err := http.Get(e.Server.URL + path)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("%s: %v %v", path, res, err)
		}
		res.Body.Close()
	}
}

func TestLoginByUsernameOrEmail(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("budi", "user")

	for _, login := range []string{"budi", "BUDI", "budi@example.com", "Budi@Example.com"} {
		c := e.Client()
		res := c.Login(login, ts.Password("budi")).Expect(t, http.StatusOK)
		if res.String("data.user.username") != "budi" || res.String("data.csrf_token") == "" {
			t.Fatalf("%s: unexpected body %s", login, res.Raw)
		}
		cookie := res.Header.Get("Set-Cookie")
		for _, attr := range []string{"sid=", "HttpOnly", "SameSite=Lax", "Path=/"} {
			if !strings.Contains(cookie, attr) {
				t.Fatalf("cookie %q missing %s", cookie, attr)
			}
		}
		// Single role → auto-selected.
		if res.String("data.active_role.slug") != "user" || res.Bool("data.requires_role_selection") {
			t.Fatalf("expected auto-selected role: %s", res.Raw)
		}
		if res.Len("data.permissions.dashboard") != 5 {
			t.Fatalf("expected dashboard permissions: %s", res.Raw)
		}
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("aktif", "user")
	inactive := e.CreateUser("nonaktif", "user")
	deleted := e.CreateUser("terhapus", "user")
	e.Exec(`UPDATE users SET is_active = false WHERE id = $1`, inactive)
	e.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, deleted)

	cases := []struct{ login, password string }{
		{"aktif", "wrong-pass1"},
		{"tidakada", "whatever123"},
		{"nonaktif", ts.Password("nonaktif")},
		{"terhapus", ts.Password("terhapus")},
	}
	var message string
	for _, c := range cases {
		res := e.Client().Login(c.login, c.password).ExpectError(t, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		if message == "" {
			message = res.String("error.message")
		} else if res.String("error.message") != message {
			t.Fatalf("messages differ: %q vs %q", message, res.String("error.message"))
		}
		if strings.Contains(res.Header.Get("Set-Cookie"), "sid=") && !strings.Contains(res.Header.Get("Set-Cookie"), "Max-Age=0") {
			t.Fatal("failed login must not set a session cookie")
		}
	}

	e.Client().Login("x", "").ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
}

func TestLoginThrottle(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("sasaran", "user")
	c := e.Client()
	for i := 1; i <= 4; i++ {
		c.Login("sasaran", "salah-salah1").ExpectError(t, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	}
	res := c.Login("sasaran", "salah-salah1").ExpectError(t, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
	if res.Header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	// Correct password is still blocked while locked.
	c.Login("sasaran", ts.Password("sasaran")).ExpectError(t, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
	// Other accounts are not affected.
	e.CreateUser("lain", "user")
	e.Client().MustLogin("lain")

	// After the lock expires the correct password works and clears the counter.
	e.Exec(`UPDATE login_attempts SET locked_until = now() - interval '1 second', window_started_at = now() - interval '2 minutes'`)
	c.MustLogin("sasaran")
	if n := e.Count(`SELECT count(*) FROM login_attempts WHERE key LIKE 'login|sasaran|%'`); n != 0 {
		t.Fatalf("attempts not cleared: %d", n)
	}
}

func TestLogoutAndSessionExpiry(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("sesi", "user")

	c := e.Client()
	c.MustLogin("sesi")
	c.Do(http.MethodGet, "/auth/me", nil).Expect(t, http.StatusOK)
	c.Do(http.MethodPost, "/auth/logout", nil).Expect(t, http.StatusNoContent)
	c.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED")
	if n := e.Count(`SELECT count(*) FROM sessions`); n != 0 {
		t.Fatalf("session not deleted: %d", n)
	}

	// Expired session is rejected.
	c.MustLogin("sesi")
	e.Exec(`UPDATE sessions SET expires_at = now() - interval '1 second'`)
	c.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED")

	// Activity slides the idle expiry forward.
	c.MustLogin("sesi")
	e.Exec(`UPDATE sessions SET last_seen_at = now() - interval '5 minutes', expires_at = now() + interval '1 minute'`)
	c.Do(http.MethodGet, "/auth/me", nil).Expect(t, http.StatusOK)
	if n := e.Count(`SELECT count(*) FROM sessions WHERE expires_at > now() + interval '1 hour'`); n != 1 {
		t.Fatal("idle expiry was not extended")
	}

	// Absolute lifetime caps the sliding window.
	e.Exec(`UPDATE sessions SET created_at = now() - interval '23 hours 59 minutes', last_seen_at = now() - interval '5 minutes'`)
	c.Do(http.MethodGet, "/auth/me", nil).Expect(t, http.StatusOK)
	if n := e.Count(`SELECT count(*) FROM sessions WHERE expires_at < now() + interval '2 minutes'`); n != 1 {
		t.Fatal("absolute lifetime not enforced")
	}
}

func TestLoginRotatesSession(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("rotasi", "user")
	c := e.Client()
	c.MustLogin("rotasi")
	c.MustLogin("rotasi") // CSRF header carries the token of the first session
	if n := e.Count(`SELECT count(*) FROM sessions`); n != 1 {
		t.Fatalf("previous session not revoked on re-login: %d sessions", n)
	}
}

func TestSessionTokenIsHashedAtRest(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("hash", "user")
	res := e.Client().MustLogin("hash")
	cookie := res.Header.Get("Set-Cookie")
	token := strings.TrimPrefix(strings.Split(cookie, ";")[0], "sid=")
	if n := e.Count(`SELECT count(*) FROM sessions WHERE token_hash = convert_to($1, 'UTF8')`, token); n != 0 {
		t.Fatal("raw token stored in database")
	}
	if n := e.Count(`SELECT count(*) FROM sessions WHERE token_hash = sha256(convert_to($1, 'UTF8'))`, token); n != 1 {
		t.Fatal("session hash does not match sha256(token)")
	}
}

func TestCSRFProtection(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	c := e.Client()
	c.MustLogin("admin")
	token := c.CSRF

	c.CSRF = ""
	c.Do(http.MethodPost, "/roles", map[string]string{"name": "A"}).ExpectError(t, http.StatusForbidden, "CSRF_FAILED")
	c.CSRF = "wrong"
	c.Do(http.MethodPost, "/roles", map[string]string{"name": "A"}).ExpectError(t, http.StatusForbidden, "CSRF_FAILED")
	c.CSRF = token
	c.Origin = "https://evil.example"
	c.Do(http.MethodPost, "/roles", map[string]string{"name": "A"}).ExpectError(t, http.StatusForbidden, "CSRF_FAILED")
	c.Origin = "null"
	c.Do(http.MethodPost, "/roles", map[string]string{"name": "A"}).ExpectError(t, http.StatusForbidden, "CSRF_FAILED")
	c.Origin = ts.Origin
	c.Do(http.MethodPost, "/roles", map[string]string{"name": "A"}).Expect(t, http.StatusCreated)
	// Safe methods need no token.
	c.CSRF = ""
	c.Do(http.MethodGet, "/roles", nil).Expect(t, http.StatusOK)

	// Login from a foreign origin is rejected too.
	evil := e.Client()
	evil.Origin = "https://evil.example"
	evil.Login("admin", ts.Password("admin")).ExpectError(t, http.StatusForbidden, "CSRF_FAILED")
}

func TestProtectedEndpointsRequireAuthentication(t *testing.T) {
	e := ts.NewEnv(t, nil)
	c := e.Client()
	for _, ep := range []struct{ method, path string }{
		{"GET", "/auth/me"}, {"POST", "/auth/logout"}, {"PUT", "/auth/active-role"},
		{"GET", "/navigation"}, {"GET", "/users"}, {"POST", "/users"},
		{"GET", "/users/00000000-0000-7000-8000-000000000001"}, {"DELETE", "/users/00000000-0000-7000-8000-000000000001"},
		{"POST", "/users/00000000-0000-7000-8000-000000000001/impersonate"},
		{"GET", "/roles"}, {"PUT", "/roles/00000000-0000-7000-8000-000000000002/permissions"},
		{"GET", "/menus"}, {"POST", "/menus"}, {"GET", "/menu-groups"}, {"GET", "/actions"},
		{"GET", "/profile"}, {"PUT", "/profile/password"},
	} {
		res := c.Do(ep.method, ep.path, map[string]string{})
		if res.Status != http.StatusUnauthorized {
			t.Errorf("%s %s: status %d, want 401", ep.method, ep.path, res.Status)
		}
	}
}

func TestActiveRoleSelection(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateRole("operator", map[string][]string{"users": {"view"}})
	e.CreateUser("multi", "user", "operator")

	c := e.Client()
	res := c.MustLogin("multi")
	if !res.Bool("data.requires_role_selection") || res.Get("data.active_role") != nil || res.Len("data.roles") != 2 {
		t.Fatalf("expected role selection: %s", res.Raw)
	}
	c.Do(http.MethodGet, "/navigation", nil).ExpectError(t, http.StatusForbidden, "ROLE_SELECTION_REQUIRED")
	c.Do(http.MethodGet, "/users", nil).ExpectError(t, http.StatusForbidden, "ROLE_SELECTION_REQUIRED")

	// Laravel defect D1: selecting a role the user does not hold must fail.
	superID := e.RoleID("super-admin").String()
	c.Do(http.MethodPut, "/auth/active-role", map[string]string{"role_id": superID}).
		ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	c.Do(http.MethodGet, "/users", nil).ExpectError(t, http.StatusForbidden, "ROLE_SELECTION_REQUIRED")

	res = c.Do(http.MethodPut, "/auth/active-role", map[string]string{"role_id": e.RoleID("operator").String()}).Expect(t, http.StatusOK)
	if res.String("data.active_role.slug") != "operator" {
		t.Fatalf("role not switched: %s", res.Raw)
	}
	c.Do(http.MethodGet, "/users", nil).Expect(t, http.StatusOK)
	c.Do(http.MethodGet, "/profile", nil).Expect(t, http.StatusOK)

	// Switching to "user" removes access to /users.
	c.Do(http.MethodPut, "/auth/active-role", map[string]string{"role_id": e.RoleID("user").String()}).Expect(t, http.StatusOK)
	c.Do(http.MethodGet, "/users", nil).ExpectError(t, http.StatusForbidden, "FORBIDDEN")

	// Revoking the active role takes effect immediately.
	e.Exec(`DELETE FROM user_roles WHERE role_id = $1`, e.RoleID("user"))
	c.Do(http.MethodGet, "/navigation", nil).ExpectError(t, http.StatusForbidden, "ROLE_SELECTION_REQUIRED")
}

func TestPasswordResetFlow(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("lupa", "user")
	other := e.Client()
	other.MustLogin("lupa")

	anon := e.Client()
	res := anon.Do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "tidak@ada.com"}).Expect(t, http.StatusOK)
	generic := res.String("data.message")
	time.Sleep(100 * time.Millisecond)
	if e.Mail.Count() != 0 {
		t.Fatal("mail sent for unknown address")
	}
	res = anon.Do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "LUPA@example.com"}).Expect(t, http.StatusOK)
	if res.String("data.message") != generic {
		t.Fatal("response reveals whether the email exists")
	}
	msg := e.Mail.WaitFor(t, 1)
	if msg.To != "lupa@example.com" {
		t.Fatalf("reset mail not sent: %+v", msg)
	}
	i := strings.Index(msg.Text, ts.Origin+"/reset-password?token=")
	if i < 0 {
		t.Fatalf("no link in mail: %s", msg.Text)
	}
	link := strings.Fields(msg.Text[i:])[0]
	u, _ := url.Parse(link)
	token := u.Query().Get("token")

	anon.Do(http.MethodPost, "/auth/reset-password", map[string]string{
		"token": token, "password": "pendek", "password_confirmation": "pendek",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	anon.Do(http.MethodPost, "/auth/reset-password", map[string]string{
		"token": token, "password": "BaruSekali123", "password_confirmation": "BaruSekali123",
	}).Expect(t, http.StatusOK)

	// Token is single-use; other sessions were revoked; new password works.
	anon.Do(http.MethodPost, "/auth/reset-password", map[string]string{
		"token": token, "password": "LainLagi123", "password_confirmation": "LainLagi123",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	other.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED")
	e.Client().Login("lupa", ts.Password("lupa")).ExpectError(t, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	e.Client().Login("lupa", "BaruSekali123").Expect(t, http.StatusOK)

	// Expired tokens are rejected.
	anon.Do(http.MethodPost, "/auth/forgot-password", map[string]string{"email": "lupa@example.com"}).Expect(t, http.StatusOK)
	msg = e.Mail.WaitFor(t, 2)
	u, _ = url.Parse(strings.Fields(msg.Text[strings.Index(msg.Text, ts.Origin):])[0])
	e.Exec(`UPDATE password_reset_tokens SET expires_at = now() - interval '1 second'`)
	anon.Do(http.MethodPost, "/auth/reset-password", map[string]string{
		"token": u.Query().Get("token"), "password": "LainLagi123", "password_confirmation": "LainLagi123",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
}

func TestRegistration(t *testing.T) {
	disabled := ts.NewEnv(t, nil)
	if disabled.Client().Do(http.MethodGet, "/auth/options", nil).Expect(t, http.StatusOK).Bool("data.registration_enabled") {
		t.Fatal("registration should be reported as disabled")
	}
	disabled.Client().Do(http.MethodPost, "/auth/register", map[string]string{"name": "x"}).ExpectError(t, http.StatusNotFound, "NOT_FOUND")

	e := ts.NewEnv(t, func(c *config.Config) { c.RegistrationEnabled = true })
	c := e.Client()
	if !c.Do(http.MethodGet, "/auth/options", nil).Expect(t, http.StatusOK).Bool("data.registration_enabled") {
		t.Fatal("registration should be reported as enabled")
	}
	body := map[string]string{
		"name": "Siti", "username": "siti", "email": "siti@example.com",
		"password": "Rahasia123", "password_confirmation": "Rahasia123",
	}
	c.Do(http.MethodPost, "/auth/register", body).Expect(t, http.StatusCreated)
	c.Do(http.MethodPost, "/auth/register", body).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	res := c.Login("siti", "Rahasia123").Expect(t, http.StatusOK)
	if res.String("data.active_role.slug") != "user" {
		t.Fatalf("registered user should get the default role: %s", res.Raw)
	}
}

func TestImpersonation(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	e.CreateUser("admin2", "super-admin")
	target := e.CreateUser("target", "user")
	e.CreateRole("operator", map[string][]string{"users": {"view", "update"}})
	e.CreateUser("oper", "operator")

	// Non-super-admins cannot impersonate.
	op := e.Client()
	op.MustLogin("oper")
	op.Do(http.MethodPost, "/users/"+target.String()+"/impersonate", nil).ExpectError(t, http.StatusForbidden, "FORBIDDEN")

	c := e.Client()
	res := c.MustLogin("admin")
	var admin2ID string
	e.Pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE username = 'admin2'`).Scan(&admin2ID)
	c.Do(http.MethodPost, "/users/"+admin2ID+"/impersonate", nil).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	c.Do(http.MethodPost, "/users/"+res.String("data.user.id")+"/impersonate", nil).ExpectError(t, http.StatusConflict, "CONFLICT")

	res = c.Do(http.MethodPost, "/users/"+target.String()+"/impersonate", nil).Expect(t, http.StatusOK)
	if res.String("data.user.username") != "target" || res.String("data.impersonator.username") != "admin" {
		t.Fatalf("unexpected impersonation payload: %s", res.Raw)
	}
	c.CSRF = res.String("data.csrf_token")
	c.Do(http.MethodGet, "/users", nil).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	// Nested impersonation is not possible (target is not a super-admin).
	c.Do(http.MethodPost, "/users/"+target.String()+"/impersonate", nil).ExpectError(t, http.StatusForbidden, "FORBIDDEN")

	res = c.Do(http.MethodPost, "/auth/impersonation/stop", nil).Expect(t, http.StatusOK)
	if res.String("data.user.username") != "admin" || res.String("data.active_role.slug") != "super-admin" || res.Get("data.impersonator") != nil {
		t.Fatalf("not restored to admin: %s", res.Raw)
	}
	c.CSRF = res.String("data.csrf_token")
	c.Do(http.MethodGet, "/users", nil).Expect(t, http.StatusOK)
	c.Do(http.MethodPost, "/auth/impersonation/stop", nil).ExpectError(t, http.StatusConflict, "CONFLICT")

	if n := e.Count(`SELECT count(*) FROM audit_logs WHERE action IN ('auth.impersonation_start', 'auth.impersonation_stop')`); n != 2 {
		t.Fatalf("impersonation not audited: %d", n)
	}
}

func TestMigrationsRollBackAndReapply(t *testing.T) {
	pool := ts.NewDB(t)
	ctx := context.Background()
	for range 5 {
		if err := database.Migrate(ctx, pool, "down", io.Discard); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	var tables int
	pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name <> 'goose_db_version'`).Scan(&tables)
	if tables != 0 {
		t.Fatalf("%d tables left after rolling back everything", tables)
	}
	if err := database.Migrate(ctx, pool, "up", io.Discard); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
