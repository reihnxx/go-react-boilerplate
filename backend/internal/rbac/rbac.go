// Package rbac implements the menu × action permission model evaluated
// against the session's active role.
package rbac

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
)

type Action string

const (
	View     Action = "view"
	Create   Action = "create"
	Update   Action = "update"
	Delete   Action = "delete"
	Download Action = "download"
)

// AllActions is the canonical order used in responses.
var AllActions = []Action{View, Create, Update, Delete, Download}

func ValidAction(s string) bool {
	for _, a := range AllActions {
		if string(a) == s {
			return true
		}
	}
	return false
}

// ErrNoActiveRole means the session has no active role, or the role is no
// longer assigned, active, or existing.
var ErrNoActiveRole = errors.New("rbac: no valid active role")

// Permissions is the set of grants for one role.
type Permissions struct {
	RoleID     uuid.UUID
	SuperAdmin bool
	grants     map[string]map[Action]bool
}

// NewPermissions builds a permission set; used by the store and by tests.
func NewPermissions(roleID uuid.UUID, superAdmin bool, grants map[string][]Action) *Permissions {
	p := &Permissions{RoleID: roleID, SuperAdmin: superAdmin, grants: map[string]map[Action]bool{}}
	for menu, actions := range grants {
		for _, a := range actions {
			p.add(menu, a)
		}
	}
	return p
}

func (p *Permissions) add(menu string, a Action) {
	if p.grants[menu] == nil {
		p.grants[menu] = map[Action]bool{}
	}
	p.grants[menu][a] = true
}

// Can reports whether the role may perform action on the menu identified by slug.
func (p *Permissions) Can(menu string, a Action) bool {
	if p == nil {
		return false
	}
	return p.SuperAdmin || p.grants[menu][a]
}

// Map returns grants as slug → actions in canonical order. For a super-admin
// it returns the explicit grants only; callers check SuperAdmin separately.
func (p *Permissions) Map() map[string][]Action {
	out := make(map[string][]Action, len(p.grants))
	for menu, set := range p.grants {
		actions := []Action{}
		for _, a := range AllActions {
			if set[a] {
				actions = append(actions, a)
			}
		}
		out[menu] = actions
	}
	return out
}

type Store struct{ db database.DBTX }

func NewStore(db database.DBTX) *Store { return &Store{db: db} }

// ForSession loads the permissions of roleID for userID, verifying that the
// role is still assigned to the user, active and not deleted.
func (s *Store) ForSession(ctx context.Context, userID, roleID uuid.UUID) (*Permissions, error) {
	var super bool
	err := s.db.QueryRow(ctx, `
		SELECT r.is_super_admin
		FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id AND ur.user_id = $1
		WHERE r.id = $2 AND r.is_active AND r.deleted_at IS NULL`, userID, roleID).Scan(&super)
	if database.IsNoRows(err) {
		return nil, ErrNoActiveRole
	}
	if err != nil {
		return nil, err
	}
	return s.load(ctx, roleID, super)
}

// ForRole loads the permissions of a role regardless of who holds it.
func (s *Store) ForRole(ctx context.Context, roleID uuid.UUID) (*Permissions, error) {
	var super bool
	err := s.db.QueryRow(ctx, `SELECT is_super_admin FROM roles WHERE id = $1 AND deleted_at IS NULL`, roleID).Scan(&super)
	if err != nil {
		return nil, err
	}
	return s.load(ctx, roleID, super)
}

func (s *Store) load(ctx context.Context, roleID uuid.UUID, super bool) (*Permissions, error) {
	p := NewPermissions(roleID, super, nil)
	rows, err := s.db.Query(ctx, `
		SELECT m.slug, rp.action
		FROM role_permissions rp
		JOIN menus m ON m.id = rp.menu_id
		WHERE rp.role_id = $1 AND m.is_active AND m.deleted_at IS NULL`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug, action string
		if err := rows.Scan(&slug, &action); err != nil {
			return nil, err
		}
		p.add(slug, Action(action))
	}
	return p, rows.Err()
}

type permsKey struct{}

// WithPermissions stores permissions in the context.
func WithPermissions(ctx context.Context, p *Permissions) context.Context {
	return context.WithValue(ctx, permsKey{}, p)
}

// FromContext returns the permissions loaded by a Guard middleware, or nil.
func FromContext(ctx context.Context) *Permissions {
	p, _ := ctx.Value(permsKey{}).(*Permissions)
	return p
}

// Guard provides route middleware. It is the only enforcement point for
// route-level authorization; the frontend never decides access.
type Guard struct{ store *Store }

func NewGuard(store *Store) *Guard { return &Guard{store: store} }

// resolve loads (or reuses) the active role's permissions for the request.
func (g *Guard) resolve(r *http.Request) (*http.Request, *Permissions, error) {
	if p := FromContext(r.Context()); p != nil {
		return r, p, nil
	}
	principal := identity.PrincipalFrom(r.Context())
	if principal == nil {
		return r, nil, httpx.Unauthenticated()
	}
	if principal.ActiveRoleID == nil {
		return r, nil, httpx.RoleSelectionRequired()
	}
	perms, err := g.store.ForSession(r.Context(), principal.UserID, *principal.ActiveRoleID)
	if errors.Is(err, ErrNoActiveRole) {
		return r, nil, httpx.RoleSelectionRequired()
	}
	if err != nil {
		return r, nil, err
	}
	return r.WithContext(WithPermissions(r.Context(), perms)), perms, nil
}

func (g *Guard) middleware(check func(*Permissions) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, perms, err := g.resolve(r)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			if !check(perms) {
				httpx.WriteError(w, r, httpx.Forbidden(""))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireActiveRole only requires a valid active role (e.g. profile, navigation).
func (g *Guard) RequireActiveRole() func(http.Handler) http.Handler {
	return g.middleware(func(*Permissions) bool { return true })
}

// Require requires the action on the menu identified by slug.
func (g *Guard) Require(menu string, a Action) func(http.Handler) http.Handler {
	return g.middleware(func(p *Permissions) bool { return p.Can(menu, a) })
}

// RequireSuperAdmin requires the active role to be the super-admin role.
func (g *Guard) RequireSuperAdmin() func(http.Handler) http.Handler {
	return g.middleware(func(p *Permissions) bool { return p.SuperAdmin })
}
