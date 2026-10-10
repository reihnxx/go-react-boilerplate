//go:build integration

package app_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"

	ts "github.com/reihnxx/go-react-boilerplate/backend/internal/testsupport"
)

func TestUserRoleCannotReachAdminEndpoints(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("biasa", "user")
	c := e.Client()
	c.MustLogin("biasa")

	for _, ep := range []struct{ method, path string }{
		{"GET", "/users"}, {"POST", "/users"}, {"GET", "/roles"}, {"POST", "/roles"},
		{"GET", "/menus"}, {"POST", "/menus"}, {"GET", "/menu-groups"}, {"GET", "/actions"},
		{"PUT", "/roles/" + e.RoleID("user").String() + "/permissions"},
	} {
		c.Do(ep.method, ep.path, map[string]any{}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	}

	res := c.Do(http.MethodGet, "/navigation", nil).Expect(t, http.StatusOK)
	if res.Len("data") != 1 || res.String("data.0.name") != "Beranda" || res.Len("data.0.items") != 2 {
		t.Fatalf("user navigation should only contain Beranda + Profil: %s", res.Raw)
	}
	c.Do(http.MethodGet, "/profile", nil).Expect(t, http.StatusOK)
}

func TestUserCRUD(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	c := e.Client()
	c.MustLogin("admin")
	userRole := e.RoleID("user").String()

	res := c.Do(http.MethodPost, "/users", map[string]any{
		"name": "Ani", "username": "ani", "email": "Ani@Example.com",
		"password": "Rahasia123", "password_confirmation": "Rahasia123", "role_ids": []string{userRole},
	}).Expect(t, http.StatusCreated)
	id := res.String("data.id")
	if res.String("data.email") != "ani@example.com" || res.String("data.roles.0.slug") != "user" {
		t.Fatalf("unexpected create payload: %s", res.Raw)
	}

	// Validation + case-insensitive uniqueness.
	res = c.Do(http.MethodPost, "/users", map[string]any{
		"name": "", "username": "a", "email": "bukan-email", "password": "x", "password_confirmation": "y",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	for _, f := range []string{"name", "username", "email", "password", "password_confirmation"} {
		if res.String("error.fields."+f) == "" {
			t.Errorf("missing field error for %s: %s", f, res.Raw)
		}
	}
	res = c.Do(http.MethodPost, "/users", map[string]any{
		"name": "Ani 2", "username": "ani2", "email": "ANI@example.com", "password": "Rahasia123", "password_confirmation": "Rahasia123",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	if res.String("error.fields.email") == "" {
		t.Fatalf("expected email uniqueness error: %s", res.Raw)
	}
	c.Do(http.MethodPost, "/users", map[string]any{
		"name": "Ani 3", "username": "ANI", "email": "ani3@example.com", "password": "Rahasia123", "password_confirmation": "Rahasia123",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	c.Do(http.MethodPost, "/users", map[string]any{
		"name": "X", "username": "xyz", "email": "xyz@example.com", "password": "Rahasia123", "password_confirmation": "Rahasia123",
		"role_ids": []string{uuid.NewString()},
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")

	// List: search, filter, sort, pagination.
	res = c.Do(http.MethodGet, "/users?q=ani&per_page=1&sort=username&order=asc", nil).Expect(t, http.StatusOK)
	if res.Get("data.0.username") != "ani" || res.Get("meta.total") != float64(1) {
		t.Fatalf("unexpected list: %s", res.Raw)
	}
	res = c.Do(http.MethodGet, "/users?role_id="+e.RoleID("super-admin").String(), nil).Expect(t, http.StatusOK)
	if res.Len("data") != 1 || res.String("data.0.username") != "admin" {
		t.Fatalf("role filter failed: %s", res.Raw)
	}
	c.Do(http.MethodGet, "/users?sort=password_hash", nil).ExpectError(t, http.StatusBadRequest, "BAD_REQUEST")

	// Update, status, password, delete.
	c.Do(http.MethodPatch, "/users/"+id, map[string]any{"name": "Ani Baru", "username": "ani", "email": "ani.baru@example.com"}).Expect(t, http.StatusOK)
	res = c.Do(http.MethodGet, "/users/"+id, nil).Expect(t, http.StatusOK)
	if res.String("data.email") != "ani.baru@example.com" || res.String("data.name") != "Ani Baru" {
		t.Fatalf("update not applied: %s", res.Raw)
	}
	res = c.Do(http.MethodPatch, "/users/"+id+"/status", map[string]any{"is_active": false}).Expect(t, http.StatusOK)
	if res.Bool("data.is_active") {
		t.Fatal("status not changed")
	}
	c.Do(http.MethodPatch, "/users/"+id+"/status", map[string]any{"is_active": true}).Expect(t, http.StatusOK)
	c.Do(http.MethodPut, "/users/"+id+"/password", map[string]any{"password": "GantiBaru123", "password_confirmation": "GantiBaru123"}).Expect(t, http.StatusNoContent)
	e.Client().Login("ani", "GantiBaru123").Expect(t, http.StatusOK)

	c.Do(http.MethodDelete, "/users/"+id, nil).Expect(t, http.StatusNoContent)
	c.Do(http.MethodGet, "/users/"+id, nil).ExpectError(t, http.StatusNotFound, "NOT_FOUND")
	c.Do(http.MethodGet, "/users/not-a-uuid", nil).ExpectError(t, http.StatusNotFound, "NOT_FOUND")
	e.Client().Login("ani", "GantiBaru123").ExpectError(t, http.StatusUnauthorized, "INVALID_CREDENTIALS")

	// Username/email of a soft-deleted user can be reused.
	c.Do(http.MethodPost, "/users", map[string]any{
		"name": "Ani Lagi", "username": "ani", "email": "ani.baru@example.com", "password": "Rahasia123", "password_confirmation": "Rahasia123",
	}).Expect(t, http.StatusCreated)

	if n := e.Count(`SELECT count(*) FROM audit_logs WHERE entity_type = 'user' AND action LIKE 'user.%'`); n < 6 {
		t.Fatalf("expected user changes to be audited, got %d rows", n)
	}
}

func TestDeactivationAndPasswordChangeRevokeSessions(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	target := e.CreateUser("korban", "user")
	admin := e.Client()
	admin.MustLogin("admin")
	victim := e.Client()
	victim.MustLogin("korban")

	admin.Do(http.MethodPatch, "/users/"+target.String()+"/status", map[string]any{"is_active": false}).Expect(t, http.StatusOK)
	victim.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED")
	victim.Login("korban", ts.Password("korban")).ExpectError(t, http.StatusUnauthorized, "INVALID_CREDENTIALS")

	admin.Do(http.MethodPatch, "/users/"+target.String()+"/status", map[string]any{"is_active": true}).Expect(t, http.StatusOK)
	victim.MustLogin("korban")
	admin.Do(http.MethodPut, "/users/"+target.String()+"/password", map[string]any{"password": "Reset12345", "password_confirmation": "Reset12345"}).Expect(t, http.StatusNoContent)
	victim.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func TestLastSuperAdminIsProtected(t *testing.T) {
	e := ts.NewEnv(t, nil)
	adminID := e.CreateUser("admin", "super-admin").String()
	c := e.Client()
	c.MustLogin("admin")
	userRole := e.RoleID("user").String()

	c.Do(http.MethodPatch, "/users/"+adminID+"/status", map[string]any{"is_active": false}).ExpectError(t, http.StatusConflict, "CONFLICT")
	c.Do(http.MethodDelete, "/users/"+adminID, nil).ExpectError(t, http.StatusConflict, "CONFLICT")
	c.Do(http.MethodPut, "/users/"+adminID+"/roles", map[string]any{"role_ids": []string{userRole}}).ExpectError(t, http.StatusConflict, "CONFLICT")

	// An inactive second admin does not count.
	second := e.CreateUser("admin2", "super-admin").String()
	e.Exec(`UPDATE users SET is_active = false WHERE username = 'admin2'`)
	c.Do(http.MethodPut, "/users/"+adminID+"/roles", map[string]any{"role_ids": []string{userRole}}).ExpectError(t, http.StatusConflict, "CONFLICT")

	// With another active admin the change is allowed.
	c.Do(http.MethodPatch, "/users/"+second+"/status", map[string]any{"is_active": true}).Expect(t, http.StatusOK)
	c.Do(http.MethodPut, "/users/"+adminID+"/roles", map[string]any{"role_ids": []string{userRole}}).Expect(t, http.StatusOK)

	// admin is no longer super-admin, so admin2 is now the last one.
	c2 := e.Client()
	c2.MustLogin("admin2")
	c2.Do(http.MethodDelete, "/users/"+second, nil).ExpectError(t, http.StatusConflict, "CONFLICT")
	c2.Do(http.MethodDelete, "/users/"+adminID, nil).Expect(t, http.StatusNoContent)
}

func TestPrivilegeEscalationIsBlocked(t *testing.T) {
	e := ts.NewEnv(t, nil)
	adminID := e.CreateUser("admin", "super-admin")
	e.CreateRole("operator", map[string][]string{
		"users": {"view", "create", "update"},
		"roles": {"view", "update"},
	})
	e.CreateRole("manager", map[string][]string{"users": {"view", "delete"}, "menus": {"view"}})
	e.CreateRole("viewer", map[string][]string{"users": {"view"}})
	opID := e.CreateUser("oper", "operator")
	staff := e.CreateUser("staff", "viewer")

	c := e.Client()
	c.MustLogin("oper")
	superRole := e.RoleID("super-admin").String()
	managerRole := e.RoleID("manager").String()
	viewerRole := e.RoleID("viewer").String()

	// Cannot grant the super-admin role or roles exceeding own permissions.
	c.Do(http.MethodPut, "/users/"+staff.String()+"/roles", map[string]any{"role_ids": []string{superRole}}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	c.Do(http.MethodPut, "/users/"+staff.String()+"/roles", map[string]any{"role_ids": []string{managerRole}}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	c.Do(http.MethodPost, "/users", map[string]any{
		"name": "Baru", "username": "baru", "email": "baru@example.com", "password": "Rahasia123", "password_confirmation": "Rahasia123",
		"role_ids": []string{managerRole},
	}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	// A role within own permissions is fine.
	c.Do(http.MethodPut, "/users/"+staff.String()+"/roles", map[string]any{"role_ids": []string{viewerRole}}).Expect(t, http.StatusOK)

	// Cannot change own roles.
	c.Do(http.MethodPut, "/users/"+opID.String()+"/roles", map[string]any{"role_ids": []string{viewerRole}}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")

	// Cannot touch super-admin accounts.
	c.Do(http.MethodPatch, "/users/"+adminID.String(), map[string]any{"name": "Hacked", "username": "admin", "email": "admin@example.com"}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	c.Do(http.MethodPut, "/users/"+adminID.String()+"/password", map[string]any{"password": "Hacked1234", "password_confirmation": "Hacked1234"}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	c.Do(http.MethodPatch, "/users/"+adminID.String()+"/status", map[string]any{"is_active": false}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")

	// Cannot grant a permission they do not hold via the matrix (own role included).
	var usersMenu string
	res := c.Do(http.MethodGet, "/roles/"+e.RoleID("operator").String()+"/permissions", nil).Expect(t, http.StatusOK)
	for i := 0; i < res.Len("data.menus"); i++ {
		if res.String("data.menus."+strconv.Itoa(i)+".slug") == "users" {
			usersMenu = res.String("data.menus." + strconv.Itoa(i) + ".id")
		}
	}
	if usersMenu == "" {
		t.Fatalf("users menu not in matrix: %s", res.Raw)
	}
	c.Do(http.MethodPut, "/roles/"+e.RoleID("operator").String()+"/permissions", map[string]any{
		"permissions": []map[string]any{{"menu_id": usersMenu, "actions": []string{"view", "create", "update", "delete"}}},
	}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
	// Keeping or reducing existing grants is allowed.
	c.Do(http.MethodPut, "/roles/"+e.RoleID("viewer").String()+"/permissions", map[string]any{
		"permissions": []map[string]any{{"menu_id": usersMenu, "actions": []string{"view"}}},
	}).Expect(t, http.StatusOK)
	// Cannot use impersonation.
	c.Do(http.MethodPost, "/users/"+staff.String()+"/impersonate", nil).ExpectError(t, http.StatusForbidden, "FORBIDDEN")
}

func TestRoleCRUDAndPermissionMatrix(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	e.CreateUser("biasa", "user")
	c := e.Client()
	c.MustLogin("admin")

	res := c.Do(http.MethodPost, "/roles", map[string]any{"name": "Kepala Sekolah"}).Expect(t, http.StatusCreated)
	id := res.String("data.id")
	if res.String("data.slug") != "kepala-sekolah" {
		t.Fatalf("unexpected slug: %s", res.Raw)
	}
	c.Do(http.MethodPost, "/roles", map[string]any{"name": "kepala sekolah"}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	c.Do(http.MethodPost, "/roles", map[string]any{"name": ""}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")

	// Rename keeps the slug.
	res = c.Do(http.MethodPatch, "/roles/"+id, map[string]any{"name": "Kepsek"}).Expect(t, http.StatusOK)
	if res.String("data.slug") != "kepala-sekolah" || res.String("data.name") != "Kepsek" {
		t.Fatalf("rename changed slug: %s", res.Raw)
	}

	// System roles are protected.
	userRole := e.RoleID("user").String()
	superRole := e.RoleID("super-admin").String()
	c.Do(http.MethodDelete, "/roles/"+userRole, nil).ExpectError(t, http.StatusConflict, "CONFLICT")
	c.Do(http.MethodDelete, "/roles/"+superRole, nil).ExpectError(t, http.StatusConflict, "CONFLICT")
	c.Do(http.MethodPatch, "/roles/"+superRole, map[string]any{"name": "Super Admin", "is_active": false}).ExpectError(t, http.StatusConflict, "CONFLICT")

	// Matrix: super-admin is read-only and fully granted.
	res = c.Do(http.MethodGet, "/roles/"+superRole+"/permissions", nil).Expect(t, http.StatusOK)
	if !res.Bool("data.read_only") || res.Len("data.menus.0.granted") != 5 || res.Len("data.actions") != 5 {
		t.Fatalf("unexpected super-admin matrix: %s", res.Raw)
	}
	c.Do(http.MethodPut, "/roles/"+superRole+"/permissions", map[string]any{"permissions": []any{}}).ExpectError(t, http.StatusConflict, "CONFLICT")

	// Grant "users:view" to the user role → a user can list users.
	res = c.Do(http.MethodGet, "/roles/"+userRole+"/permissions", nil).Expect(t, http.StatusOK)
	ids := map[string]string{}
	for i := 0; i < res.Len("data.menus"); i++ {
		ids[res.String("data.menus."+strconv.Itoa(i)+".slug")] = res.String("data.menus." + strconv.Itoa(i) + ".id")
	}
	c.Do(http.MethodPut, "/roles/"+userRole+"/permissions", map[string]any{"permissions": []map[string]any{
		{"menu_id": ids["dashboard"], "actions": []string{"view"}},
		{"menu_id": ids["profile"], "actions": []string{"view", "update", "view"}},
		{"menu_id": ids["users"], "actions": []string{"view"}},
	}}).Expect(t, http.StatusOK)
	if n := e.Count(`SELECT count(*) FROM role_permissions WHERE role_id = $1`, e.RoleID("user")); n != 4 {
		t.Fatalf("expected 4 grants after replace, got %d", n)
	}
	u := e.Client()
	u.MustLogin("biasa")
	u.Do(http.MethodGet, "/users", nil).Expect(t, http.StatusOK)
	u.Do(http.MethodPost, "/users", map[string]any{}).ExpectError(t, http.StatusForbidden, "FORBIDDEN")

	c.Do(http.MethodPut, "/roles/"+userRole+"/permissions", map[string]any{"permissions": []map[string]any{
		{"menu_id": ids["users"], "actions": []string{"approve"}},
	}}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	c.Do(http.MethodPut, "/roles/"+userRole+"/permissions", map[string]any{"permissions": []map[string]any{
		{"menu_id": uuid.NewString(), "actions": []string{"view"}},
	}}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")

	// Deleting a role removes assignments; holders lose it immediately.
	e.Exec(`INSERT INTO user_roles (user_id, role_id) SELECT id, $1 FROM users WHERE username = 'biasa'`, uuid.MustParse(id))
	c.Do(http.MethodDelete, "/roles/"+id, nil).Expect(t, http.StatusNoContent)
	if n := e.Count(`SELECT count(*) FROM user_roles WHERE role_id = $1`, uuid.MustParse(id)); n != 0 {
		t.Fatal("assignments not removed")
	}
	res = c.Do(http.MethodGet, "/roles?q=kep", nil).Expect(t, http.StatusOK)
	if res.Len("data") != 0 {
		t.Fatalf("deleted role still listed: %s", res.Raw)
	}
}

func TestMenuManagementAndNavigation(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("admin", "super-admin")
	e.CreateUser("biasa", "user")
	c := e.Client()
	c.MustLogin("admin")

	res := c.Do(http.MethodGet, "/menu-groups", nil).Expect(t, http.StatusOK)
	var manajemen string
	for i := 0; i < res.Len("data"); i++ {
		if res.String("data."+strconv.Itoa(i)+".name") == "Manajemen" {
			manajemen = res.String("data." + strconv.Itoa(i) + ".id")
		}
	}

	// Validation rules from Laravel: main needs group + icon, child needs parent + path.
	res = c.Do(http.MethodPost, "/menus", map[string]any{"name": "Laporan", "type": "main"}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	if res.String("error.fields.group_id") == "" || res.String("error.fields.icon") == "" {
		t.Fatalf("missing main-menu errors: %s", res.Raw)
	}
	res = c.Do(http.MethodPost, "/menus", map[string]any{"name": "Sub", "type": "child"}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	if res.String("error.fields.parent_id") == "" || res.String("error.fields.path") == "" {
		t.Fatalf("missing child-menu errors: %s", res.Raw)
	}
	c.Do(http.MethodPost, "/menus", map[string]any{"name": "X", "type": "main", "group_id": manajemen, "icon": "<script>", "path": "javascript:alert(1)"}).
		ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")

	res = c.Do(http.MethodPost, "/menus", map[string]any{"name": "Laporan Bulanan", "type": "main", "group_id": manajemen, "icon": "file-text"}).Expect(t, http.StatusCreated)
	parent := res.String("data.id")
	if res.String("data.slug") != "laporan_bulanan" || res.Get("data.sort_order") != float64(40) {
		t.Fatalf("unexpected menu: %s", res.Raw)
	}
	c.Do(http.MethodPost, "/menus", map[string]any{"name": "Laporan Bulanan", "type": "main", "group_id": manajemen, "icon": "file-text"}).
		ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR") // duplicate slug
	res = c.Do(http.MethodPost, "/menus", map[string]any{"name": "Rekap", "slug": "rekap", "type": "child", "parent_id": parent, "path": "/laporan/rekap"}).Expect(t, http.StatusCreated)
	child := res.String("data.id")
	if res.String("data.group_name") != "Manajemen" || res.String("data.parent_name") != "Laporan Bulanan" {
		t.Fatalf("unexpected child: %s", res.Raw)
	}
	// Max two levels.
	c.Do(http.MethodPost, "/menus", map[string]any{"name": "Cucu", "type": "child", "parent_id": child, "path": "/x"}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	// A parent cannot become a child.
	c.Do(http.MethodPatch, "/menus/"+parent, map[string]any{"name": "Laporan Bulanan", "type": "child", "parent_id": e.MenuID("dashboard").String(), "path": "/l"}).
		ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")

	// Grant view on both to the user role; user sees the new section.
	e.Exec(`INSERT INTO role_permissions (role_id, menu_id, action) VALUES ($1, $2, 'view'), ($1, $3, 'view')`,
		e.RoleID("user"), uuid.MustParse(parent), uuid.MustParse(child))
	u := e.Client()
	u.MustLogin("biasa")
	res = u.Do(http.MethodGet, "/navigation", nil).Expect(t, http.StatusOK)
	if res.Len("data") != 2 || res.String("data.1.items.0.slug") != "laporan_bulanan" || res.String("data.1.items.0.children.0.path") != "/laporan/rekap" {
		t.Fatalf("navigation missing new menu: %s", res.Raw)
	}

	// Deactivating the parent hides it.
	c.Do(http.MethodPatch, "/menus/"+parent, map[string]any{"name": "Laporan Bulanan", "type": "main", "group_id": manajemen, "icon": "file-text", "is_active": false}).Expect(t, http.StatusOK)
	res = u.Do(http.MethodGet, "/navigation", nil).Expect(t, http.StatusOK)
	if res.Len("data") != 1 {
		t.Fatalf("inactive menu still visible: %s", res.Raw)
	}

	// List with filters.
	res = c.Do(http.MethodGet, "/menus?type=child&q=rekap", nil).Expect(t, http.StatusOK)
	if res.Len("data") != 1 || res.String("data.0.type") != "child" {
		t.Fatalf("filter failed: %s", res.Raw)
	}

	// Delete: parent with children is refused; child then parent works and grants go away.
	c.Do(http.MethodDelete, "/menus/"+parent, nil).ExpectError(t, http.StatusConflict, "CONFLICT")
	c.Do(http.MethodDelete, "/menus/"+child, nil).Expect(t, http.StatusNoContent)
	c.Do(http.MethodDelete, "/menus/"+parent, nil).Expect(t, http.StatusNoContent)
	c.Do(http.MethodGet, "/menus/"+parent, nil).ExpectError(t, http.StatusNotFound, "NOT_FOUND")
	if n := e.Count(`SELECT count(*) FROM role_permissions WHERE menu_id IN ($1, $2)`, uuid.MustParse(parent), uuid.MustParse(child)); n != 0 {
		t.Fatalf("grants not removed: %d", n)
	}
	// Slug can be reused after deletion.
	c.Do(http.MethodPost, "/menus", map[string]any{"name": "Laporan Bulanan", "type": "main", "group_id": manajemen, "icon": "file-text"}).Expect(t, http.StatusCreated)
}

func TestProfile(t *testing.T) {
	e := ts.NewEnv(t, nil)
	e.CreateUser("ani", "user")
	e.CreateUser("budi", "user")
	c := e.Client()
	c.MustLogin("ani")
	other := e.Client()
	other.MustLogin("ani")

	res := c.Do(http.MethodPut, "/profile", map[string]any{
		"name": "Ani Lengkap", "email": "ani.new@example.com", "phone": "081234567890",
		"birth_place": "Bandung", "birth_date": "1995-05-17", "address": "Jl. Merdeka 1",
	}).Expect(t, http.StatusOK)
	if res.String("data.birth_date") != "1995-05-17" || res.String("data.phone") != "081234567890" {
		t.Fatalf("unexpected profile: %s", res.Raw)
	}
	me := c.Do(http.MethodGet, "/auth/me", nil).Expect(t, http.StatusOK)
	if me.String("data.user.email") != "ani.new@example.com" {
		t.Fatalf("user email not synced: %s", me.Raw)
	}

	res = c.Do(http.MethodPut, "/profile", map[string]any{
		"name": "Ani", "email": "budi@example.com", "phone": "12345", "birth_date": "2999-01-01",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	if res.String("error.fields.phone") == "" || res.String("error.fields.birth_date") == "" {
		t.Fatalf("missing field errors: %s", res.Raw)
	}
	res = c.Do(http.MethodPut, "/profile", map[string]any{"name": "Ani", "email": "budi@example.com"}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	if res.String("error.fields.email") == "" {
		t.Fatalf("expected email uniqueness error: %s", res.Raw)
	}

	c.Do(http.MethodPut, "/profile/password", map[string]any{
		"current_password": "salah123", "password": "BaruLagi123", "password_confirmation": "BaruLagi123",
	}).ExpectError(t, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	c.Do(http.MethodPut, "/profile/password", map[string]any{
		"current_password": ts.Password("ani"), "password": "BaruLagi123", "password_confirmation": "BaruLagi123",
	}).Expect(t, http.StatusNoContent)
	c.Do(http.MethodGet, "/auth/me", nil).Expect(t, http.StatusOK)                                       // current session kept
	other.Do(http.MethodGet, "/auth/me", nil).ExpectError(t, http.StatusUnauthorized, "UNAUTHENTICATED") // others revoked
	e.Client().Login("ani", "BaruLagi123").Expect(t, http.StatusOK)
}
