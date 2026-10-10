package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
)

type RoleRef struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	IsSuperAdmin bool      `json:"is_super_admin"`
}

type User struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Roles       []RoleRef  `json:"roles"`
}

// IsSuperAdmin reports whether the user holds the super-admin role.
func (u *User) IsSuperAdmin() bool {
	for _, r := range u.Roles {
		if r.IsSuperAdmin {
			return true
		}
	}
	return false
}

type Filter struct {
	IsActive *bool
	RoleID   *uuid.UUID
}

var listSort = httpx.SortSpec{
	Allowed: map[string]string{
		"name": "u.name", "username": "u.username", "email": "u.email",
		"is_active": "u.is_active", "created_at": "u.created_at", "last_login_at": "u.last_login_at",
	},
	Default: "created_at", DefaultDesc: true,
}

type Store struct{ db database.DBTX }

func NewStore(db database.DBTX) *Store { return &Store{db: db} }

func (s *Store) WithTx(tx pgx.Tx) *Store { return &Store{db: tx} }

const userColumns = `u.id, u.name, u.username, u.email, u.is_active, u.last_login_at, u.created_at, u.updated_at`

func scanUser(row pgx.Row, extra ...any) (User, error) {
	var u User
	dest := append([]any{&u.ID, &u.Name, &u.Username, &u.Email, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt}, extra...)
	err := row.Scan(dest...)
	u.Roles = []RoleRef{}
	return u, err
}

func (s *Store) List(ctx context.Context, q httpx.PageQuery, f Filter) ([]User, int64, error) {
	where := []string{"u.deleted_at IS NULL"}
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	if q.Search != "" {
		p := arg(httpx.LikePattern(q.Search))
		where = append(where, fmt.Sprintf("(u.name ILIKE %[1]s OR u.username ILIKE %[1]s OR u.email ILIKE %[1]s)", p))
	}
	if f.IsActive != nil {
		where = append(where, "u.is_active = "+arg(*f.IsActive))
	}
	if f.RoleID != nil {
		where = append(where, "EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id AND ur.role_id = "+arg(*f.RoleID)+")")
	}
	sql := `SELECT ` + userColumns + `, count(*) OVER () FROM users u WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY ` + q.OrderBy() + `, u.id LIMIT ` + arg(q.PerPage) + ` OFFSET ` + arg(q.Offset())

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var total int64
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := s.attachRoles(ctx, users); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := scanUser(s.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users u WHERE u.id = $1 AND u.deleted_at IS NULL`, id))
	if err != nil {
		return u, err
	}
	users := []User{u}
	if err := s.attachRoles(ctx, users); err != nil {
		return u, err
	}
	return users[0], nil
}

func (s *Store) attachRoles(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, len(users))
	index := make(map[uuid.UUID]int, len(users))
	for i, u := range users {
		ids[i] = u.ID.String()
		index[u.ID] = i
	}
	rows, err := s.db.Query(ctx, `
		SELECT ur.user_id, r.id, r.name, r.slug, r.is_super_admin
		FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = ANY($1::uuid[]) AND r.deleted_at IS NULL
		ORDER BY r.is_super_admin DESC, r.name`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID uuid.UUID
		var r RoleRef
		if err := rows.Scan(&userID, &r.ID, &r.Name, &r.Slug, &r.IsSuperAdmin); err != nil {
			return err
		}
		i := index[userID]
		users[i].Roles = append(users[i].Roles, r)
	}
	return rows.Err()
}

func (s *Store) Insert(ctx context.Context, id uuid.UUID, name, username, email, hash string, active bool) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO users (id, name, username, email, password_hash, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`, id, name, username, email, hash, active)
	return mapUnique(err)
}

func (s *Store) Update(ctx context.Context, id uuid.UUID, name, username, email string) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET name = $2, username = $3, email = $4 WHERE id = $1 AND deleted_at IS NULL`,
		id, name, username, email)
	return mapUnique(err)
}

func (s *Store) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET is_active = $2 WHERE id = $1 AND deleted_at IS NULL`, id, active)
	return err
}

func (s *Store) SetPasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1 AND deleted_at IS NULL`, id, hash)
	return err
}

func (s *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET deleted_at = now(), is_active = false WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

func (s *Store) ReplaceRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, unnest($2::uuid[])`, userID, uuidStrings(roleIDs))
	return err
}

// RolesByIDs returns the active, non-deleted roles among ids.
func (s *Store) RolesByIDs(ctx context.Context, ids []uuid.UUID) ([]RoleRef, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, slug, is_super_admin FROM roles
		WHERE id = ANY($1::uuid[]) AND is_active AND deleted_at IS NULL`, uuidStrings(ids))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRole)
}

func (s *Store) RoleBySlug(ctx context.Context, slug string) (RoleRef, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, slug, is_super_admin FROM roles
		WHERE slug = $1 AND is_active AND deleted_at IS NULL`, slug)
	if err != nil {
		return RoleRef{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanRole)
}

func scanRole(row pgx.CollectableRow) (RoleRef, error) {
	var r RoleRef
	err := row.Scan(&r.ID, &r.Name, &r.Slug, &r.IsSuperAdmin)
	return r, err
}

// LockSuperAdminRole locks the super-admin role row so concurrent changes to
// super-admin membership are serialized. Must run inside a transaction.
func (s *Store) LockSuperAdminRole(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM roles WHERE is_super_admin AND deleted_at IS NULL FOR UPDATE`).Scan(&id)
	return id, err
}

// CountActiveSuperAdminsExcept counts active, non-deleted users holding
// roleID other than userID.
func (s *Store) CountActiveSuperAdminsExcept(ctx context.Context, roleID, userID uuid.UUID) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM user_roles ur JOIN users u ON u.id = ur.user_id
		WHERE ur.role_id = $1 AND u.id <> $2 AND u.is_active AND u.deleted_at IS NULL`, roleID, userID).Scan(&n)
	return n, err
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// mapUnique converts unique violations on username/email into field errors.
func mapUnique(err error) error {
	if name, ok := database.UniqueViolation(err); ok {
		switch name {
		case "users_username_key":
			return httpx.FieldError("username", "Username sudah digunakan.")
		case "users_email_key":
			return httpx.FieldError("email", "Email sudah digunakan.")
		}
	}
	return err
}
