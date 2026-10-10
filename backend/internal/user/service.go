// Package user implements user administration: CRUD, activation, role
// assignment and admin password changes, including the guards that prevent
// privilege escalation and lock-out of the last super-admin.
package user

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/audit"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/password"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/validate"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

// SessionRevoker is implemented by auth.Service.
type SessionRevoker interface {
	RevokeUserSessions(ctx context.Context, userID uuid.UUID, except []byte) error
}

// Actor is the user performing an administrative action.
type Actor struct {
	ID    uuid.UUID
	Perms *rbac.Permissions
}

func (a Actor) isSuperAdmin() bool { return a.Perms != nil && a.Perms.SuperAdmin }

type Service struct {
	pool     *pgxpool.Pool
	store    *Store
	perms    *rbac.Store
	sessions SessionRevoker
}

func NewService(pool *pgxpool.Pool, perms *rbac.Store) *Service {
	return &Service{pool: pool, store: NewStore(pool), perms: perms}
}

// SetSessionRevoker wires the auth service after construction (auth depends
// on this service for registration, so it is built afterwards).
func (s *Service) SetSessionRevoker(r SessionRevoker) { s.sessions = r }

type CreateInput struct {
	Name                 string      `json:"name"`
	Username             string      `json:"username"`
	Email                string      `json:"email"`
	Password             string      `json:"password"`
	PasswordConfirmation string      `json:"password_confirmation"`
	IsActive             *bool       `json:"is_active"`
	RoleIDs              []uuid.UUID `json:"role_ids"`
}

type UpdateInput struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func normalize(name, username, email string) (string, string, string) {
	return strings.TrimSpace(name), strings.TrimSpace(username), strings.ToLower(strings.TrimSpace(email))
}

func validateProfile(errs validate.Errors, name, username, email string) {
	errs.Required("name", name, "Nama")
	errs.MaxLength("name", name, 255, "Nama")
	errs.Required("username", username, "Username")
	errs.MinLength("username", username, 3, "Username")
	errs.MaxLength("username", username, 50, "Username")
	if username != "" {
		errs.Match("username", username, usernameRe, "Username hanya boleh berisi huruf, angka, titik, garis bawah, atau tanda hubung.")
	}
	errs.Required("email", email, "Email")
	errs.Email("email", email)
}

func validatePassword(errs validate.Errors, plain, confirmation string) {
	if plain == "" {
		errs.Add("password", "Kata sandi wajib diisi.")
	} else if msg := password.Validate(plain); msg != "" {
		errs.Add("password", msg)
	}
	errs.Check(plain == confirmation, "password_confirmation", "Konfirmasi kata sandi tidak cocok.")
}

func (s *Service) List(ctx context.Context, q httpx.PageQuery, f Filter) ([]User, int64, error) {
	return s.store.List(ctx, q, f)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := s.store.Get(ctx, id)
	if database.IsNoRows(err) {
		return u, httpx.NotFound("Pengguna tidak ditemukan.")
	}
	return u, err
}

func (s *Service) Create(ctx context.Context, actor Actor, in CreateInput) (User, error) {
	name, username, email := normalize(in.Name, in.Username, in.Email)
	errs := validate.New()
	validateProfile(errs, name, username, email)
	validatePassword(errs, in.Password, in.PasswordConfirmation)
	if err := errs.Err(); err != nil {
		return User{}, err
	}
	roles, err := s.resolveRoles(ctx, in.RoleIDs)
	if err != nil {
		return User{}, err
	}
	if err := s.ensureCanGrant(ctx, actor, roles); err != nil {
		return User{}, err
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return User{}, err
	}
	active := in.IsActive == nil || *in.IsActive
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}

	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if err := st.Insert(ctx, id, name, username, email, hash, active); err != nil {
			return err
		}
		if err := st.ReplaceRoles(ctx, id, roleIDs(roles)); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "user.create", EntityType: "user", EntityID: id.String(),
			Metadata: map[string]any{"username": username, "role_ids": roleIDs(roles)}})
	})
	if err != nil {
		return User{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, actor Actor, id uuid.UUID, in UpdateInput) (User, error) {
	target, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}
	if err := ensureCanManage(actor, target); err != nil {
		return User{}, err
	}
	name, username, email := normalize(in.Name, in.Username, in.Email)
	errs := validate.New()
	validateProfile(errs, name, username, email)
	if err := errs.Err(); err != nil {
		return User{}, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).Update(ctx, id, name, username, email); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "user.update", EntityType: "user", EntityID: id.String(),
			Metadata: map[string]any{"before": map[string]string{"name": target.Name, "username": target.Username, "email": target.Email}}})
	})
	if err != nil {
		return User{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) SetStatus(ctx context.Context, actor Actor, id uuid.UUID, active bool) (User, error) {
	target, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}
	if err := ensureCanManage(actor, target); err != nil {
		return User{}, err
	}
	if !active && id == actor.ID {
		return User{}, httpx.Conflict("Anda tidak dapat menonaktifkan akun sendiri.")
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if !active {
			if err := s.ensureNotLastSuperAdmin(ctx, st, target, "menonaktifkan"); err != nil {
				return err
			}
		}
		if err := st.SetActive(ctx, id, active); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "user.status", EntityType: "user", EntityID: id.String(),
			Metadata: map[string]any{"is_active": active}})
	})
	if err != nil {
		return User{}, err
	}
	if !active {
		if err := s.sessions.RevokeUserSessions(ctx, id, nil); err != nil {
			return User{}, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Service) SetRoles(ctx context.Context, actor Actor, id uuid.UUID, ids []uuid.UUID) (User, error) {
	target, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}
	if err := ensureCanManage(actor, target); err != nil {
		return User{}, err
	}
	if id == actor.ID && !actor.isSuperAdmin() {
		return User{}, httpx.Forbidden("Anda tidak dapat mengubah peran akun sendiri.")
	}
	roles, err := s.resolveRoles(ctx, ids)
	if err != nil {
		return User{}, err
	}
	current := map[uuid.UUID]bool{}
	for _, r := range target.Roles {
		current[r.ID] = true
	}
	var added []RoleRef
	keepsSuper := false
	for _, r := range roles {
		if !current[r.ID] {
			added = append(added, r)
		}
		keepsSuper = keepsSuper || r.IsSuperAdmin
	}
	if err := s.ensureCanGrant(ctx, actor, added); err != nil {
		return User{}, err
	}

	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if target.IsSuperAdmin() && !keepsSuper {
			if err := s.ensureNotLastSuperAdmin(ctx, st, target, "mencabut peran Super Admin dari"); err != nil {
				return err
			}
		}
		if err := st.ReplaceRoles(ctx, id, roleIDs(roles)); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "user.roles", EntityType: "user", EntityID: id.String(),
			Metadata: map[string]any{"role_ids": roleIDs(roles)}})
	})
	if err != nil {
		return User{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) SetPassword(ctx context.Context, actor Actor, id uuid.UUID, plain, confirmation string) error {
	target, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureCanManage(actor, target); err != nil {
		return err
	}
	errs := validate.New()
	validatePassword(errs, plain, confirmation)
	if err := errs.Err(); err != nil {
		return err
	}
	hash, err := password.Hash(plain)
	if err != nil {
		return err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).SetPasswordHash(ctx, id, hash); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "user.password", EntityType: "user", EntityID: id.String()})
	})
	if err != nil {
		return err
	}
	return s.sessions.RevokeUserSessions(ctx, id, nil)
}

func (s *Service) Delete(ctx context.Context, actor Actor, id uuid.UUID) error {
	target, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureCanManage(actor, target); err != nil {
		return err
	}
	if id == actor.ID {
		return httpx.Conflict("Anda tidak dapat menghapus akun sendiri.")
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if err := s.ensureNotLastSuperAdmin(ctx, st, target, "menghapus"); err != nil {
			return err
		}
		if err := st.SoftDelete(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "user.delete", EntityType: "user", EntityID: id.String(),
			Metadata: map[string]any{"username": target.Username}})
	})
	if err != nil {
		return err
	}
	return s.sessions.RevokeUserSessions(ctx, id, nil)
}

// Register creates a self-registered user with the given role (auth.Registrar).
func (s *Service) Register(ctx context.Context, name, username, email, plain, roleSlug string) (uuid.UUID, error) {
	name, username, email = normalize(name, username, email)
	errs := validate.New()
	validateProfile(errs, name, username, email)
	validatePassword(errs, plain, plain)
	if err := errs.Err(); err != nil {
		return uuid.Nil, err
	}
	role, err := s.store.RoleBySlug(ctx, roleSlug)
	if err != nil {
		return uuid.Nil, err
	}
	if role.IsSuperAdmin {
		return uuid.Nil, httpx.Forbidden("Peran default pendaftaran tidak valid.")
	}
	return s.insertWithRoles(ctx, name, username, email, plain, []uuid.UUID{role.ID})
}

// CreateSuperAdmin is used by the create-admin CLI. It bypasses actor checks.
func (s *Service) CreateSuperAdmin(ctx context.Context, name, username, email, plain string) (uuid.UUID, error) {
	name, username, email = normalize(name, username, email)
	errs := validate.New()
	validateProfile(errs, name, username, email)
	validatePassword(errs, plain, plain)
	if err := errs.Err(); err != nil {
		return uuid.Nil, err
	}
	var superID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT id FROM roles WHERE is_super_admin AND deleted_at IS NULL`).Scan(&superID); err != nil {
		return uuid.Nil, err
	}
	return s.insertWithRoles(ctx, name, username, email, plain, []uuid.UUID{superID})
}

// CreateWithRoleSlugs is used by the dev seeder. It bypasses actor checks.
func (s *Service) CreateWithRoleSlugs(ctx context.Context, name, username, email, plain string, active bool, slugs ...string) (uuid.UUID, error) {
	name, username, email = normalize(name, username, email)
	errs := validate.New()
	validateProfile(errs, name, username, email)
	validatePassword(errs, plain, plain)
	if err := errs.Err(); err != nil {
		return uuid.Nil, err
	}
	var ids []uuid.UUID
	for _, slug := range slugs {
		r, err := s.store.RoleBySlug(ctx, slug)
		if err != nil {
			return uuid.Nil, err
		}
		ids = append(ids, r.ID)
	}
	id, err := s.insertWithRoles(ctx, name, username, email, plain, ids)
	if err != nil || active {
		return id, err
	}
	return id, s.store.SetActive(ctx, id, false)
}

func (s *Service) insertWithRoles(ctx context.Context, name, username, email, plain string, roles []uuid.UUID) (uuid.UUID, error) {
	hash, err := password.Hash(plain)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if err := st.Insert(ctx, id, name, username, email, hash, true); err != nil {
			return err
		}
		return st.ReplaceRoles(ctx, id, roles)
	})
	return id, err
}

// --- guards ---

// resolveRoles de-duplicates ids and checks that each role exists and is active.
func (s *Service) resolveRoles(ctx context.Context, ids []uuid.UUID) ([]RoleRef, error) {
	seen := map[uuid.UUID]bool{}
	unique := []uuid.UUID{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil, nil
	}
	roles, err := s.store.RolesByIDs(ctx, unique)
	if err != nil {
		return nil, err
	}
	if len(roles) != len(unique) {
		return nil, httpx.FieldError("role_ids", "Satu atau lebih peran tidak valid.")
	}
	return roles, nil
}

// ensureCanManage: only a super-admin may modify an account that holds the
// super-admin role.
func ensureCanManage(actor Actor, target User) error {
	if target.IsSuperAdmin() && !actor.isSuperAdmin() {
		return httpx.Forbidden("Hanya Super Admin yang dapat mengubah akun Super Admin.")
	}
	return nil
}

// ensureCanGrant prevents privilege escalation: a non-super-admin may only
// grant roles whose permissions are a subset of their own.
func (s *Service) ensureCanGrant(ctx context.Context, actor Actor, roles []RoleRef) error {
	if actor.isSuperAdmin() {
		return nil
	}
	for _, r := range roles {
		if r.IsSuperAdmin {
			return httpx.Forbidden("Hanya Super Admin yang dapat memberikan peran Super Admin.")
		}
		rp, err := s.perms.ForRole(ctx, r.ID)
		if err != nil {
			return err
		}
		for menu, actions := range rp.Map() {
			for _, a := range actions {
				if !actor.Perms.Can(menu, a) {
					return httpx.Forbidden("Anda tidak dapat memberikan peran \"" + r.Name + "\" karena hak aksesnya melebihi hak akses Anda.")
				}
			}
		}
	}
	return nil
}

// ensureNotLastSuperAdmin blocks an operation that would leave no active
// super-admin. It locks the super-admin role row; call it inside the tx.
func (s *Service) ensureNotLastSuperAdmin(ctx context.Context, st *Store, target User, verb string) error {
	if !target.IsSuperAdmin() || !target.IsActive {
		return nil
	}
	roleID, err := st.LockSuperAdminRole(ctx)
	if err != nil {
		return err
	}
	others, err := st.CountActiveSuperAdminsExcept(ctx, roleID, target.ID)
	if err != nil {
		return err
	}
	if others == 0 {
		return httpx.Conflict("Tidak dapat " + verb + " Super Admin aktif terakhir.")
	}
	return nil
}

func roleIDs(roles []RoleRef) []uuid.UUID {
	out := make([]uuid.UUID, len(roles))
	for i, r := range roles {
		out[i] = r.ID
	}
	return out
}
