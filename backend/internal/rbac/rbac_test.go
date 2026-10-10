package rbac

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
)

func TestPermissionsCan(t *testing.T) {
	p := NewPermissions(uuid.New(), false, map[string][]Action{
		"users": {View, Update},
	})
	cases := []struct {
		menu string
		a    Action
		want bool
	}{
		{"users", View, true},
		{"users", Update, true},
		{"users", Delete, false},
		{"roles", View, false},
	}
	for _, c := range cases {
		if got := p.Can(c.menu, c.a); got != c.want {
			t.Errorf("Can(%s,%s)=%v want %v", c.menu, c.a, got, c.want)
		}
	}
	var nilPerms *Permissions
	if nilPerms.Can("users", View) {
		t.Error("nil permissions must deny")
	}
}

func TestSuperAdminCanEverything(t *testing.T) {
	p := NewPermissions(uuid.New(), true, nil)
	if !p.Can("anything", Delete) {
		t.Fatal("super-admin must be allowed")
	}
}

func TestMapCanonicalOrder(t *testing.T) {
	p := NewPermissions(uuid.New(), false, map[string][]Action{"menus": {Delete, View, Create}})
	if got := p.Map()["menus"]; !reflect.DeepEqual(got, []Action{View, Create, Delete}) {
		t.Fatalf("Map order = %v", got)
	}
}

func TestValidAction(t *testing.T) {
	if !ValidAction("download") || ValidAction("admin") || ValidAction("") {
		t.Fatal("ValidAction misbehaves")
	}
}

func TestGuardWithoutPrincipalOrRole(t *testing.T) {
	g := NewGuard(nil) // store is not reached in these cases
	h := g.Require("users", View)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(identity.WithPrincipal(req.Context(), &identity.Principal{UserID: uuid.New()}))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("no active role: status %d", rec.Code)
	}
}

func TestGuardUsesPermissionsFromContext(t *testing.T) {
	g := NewGuard(nil)
	ran := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ran = true })
	perms := NewPermissions(uuid.New(), false, map[string][]Action{"users": {View}})

	for _, tc := range []struct {
		action Action
		want   int
	}{{View, http.StatusOK}, {Delete, http.StatusForbidden}} {
		ran = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(WithPermissions(req.Context(), perms))
		g.Require("users", tc.action)(next).ServeHTTP(rec, req)
		if rec.Code != tc.want || ran != (tc.want == http.StatusOK) {
			t.Errorf("%s: status %d ran %v", tc.action, rec.Code, ran)
		}
	}
}
