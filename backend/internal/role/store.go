package role

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

type Role struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	IsSuperAdmin bool      `json:"is_super_admin"`
	IsSystem     bool      `json:"is_system"`
	IsActive     bool      `json:"is_active"`
	UserCount    int       `json:"user_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ActionInfo struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type grant struct {
	MenuID uuid.UUID
	Action string
}

var listSort = httpx.SortSpec{
	Allowed: map[string]string{"name": "r.name", "created_at": "r.created_at", "user_count": "user_count"},
	Default: "name",
}

type Store struct{ db database.DBTX }

func NewStore(db database.DBTX) *Store { return &Store{db: db} }

func (s *Store) WithTx(tx pgx.Tx) *Store { return &Store{db: tx} }

const roleSelect = `
	SELECT r.id, r.name, r.slug, r.is_super_admin, r.is_system, r.is_active, r.created_at, r.updated_at,
	       (SELECT count(*) FROM user_roles ur JOIN users u ON u.id = ur.user_id
	        WHERE ur.role_id = r.id AND u.deleted_at IS NULL) AS user_count
	FROM roles r`

func scanRole(row pgx.Row, extra ...any) (Role, error) {
	var r Role
	dest := append([]any{&r.ID, &r.Name, &r.Slug, &r.IsSuperAdmin, &r.IsSystem, &r.IsActive, &r.CreatedAt, &r.UpdatedAt, &r.UserCount}, extra...)
	err := row.Scan(dest...)
	return r, err
}

func (s *Store) List(ctx context.Context, q httpx.PageQuery, active *bool) ([]Role, int64, error) {
	where := []string{"r.deleted_at IS NULL"}
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if q.Search != "" {
		where = append(where, "r.name ILIKE "+arg(httpx.LikePattern(q.Search)))
	}
	if active != nil {
		where = append(where, "r.is_active = "+arg(*active))
	}
	sql := roleSelect + ` WHERE ` + strings.Join(where, " AND ")
	sql = `SELECT *, count(*) OVER () FROM (` + sql + `) r ORDER BY ` + q.OrderBy() + `, r.id LIMIT ` +
		arg(q.PerPage) + ` OFFSET ` + arg(q.Offset())
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var total int64
	out := []Role{}
	for rows.Next() {
		r, err := scanRole(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Role, error) {
	return scanRole(s.db.QueryRow(ctx, roleSelect+` WHERE r.id = $1 AND r.deleted_at IS NULL`, id))
}

func (s *Store) SlugExists(ctx context.Context, slug string) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM roles WHERE slug = $1 AND deleted_at IS NULL)`, slug).Scan(&ok)
	return ok, err
}

func (s *Store) Insert(ctx context.Context, id uuid.UUID, name, slug string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO roles (id, name, slug) VALUES ($1, $2, $3)`, id, name, slug)
	return mapUnique(err)
}

func (s *Store) Update(ctx context.Context, id uuid.UUID, name string, active bool) error {
	_, err := s.db.Exec(ctx, `UPDATE roles SET name = $2, is_active = $3 WHERE id = $1 AND deleted_at IS NULL`, id, name, active)
	return mapUnique(err)
}

// SoftDelete marks the role deleted and removes its assignments and grants.
func (s *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM user_roles WHERE role_id = $1`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE roles SET deleted_at = now(), is_active = false WHERE id = $1`, id)
	return err
}

func (s *Store) Actions(ctx context.Context) ([]ActionInfo, error) {
	rows, err := s.db.Query(ctx, `SELECT code, label FROM actions ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ActionInfo, error) {
		var a ActionInfo
		err := row.Scan(&a.Code, &a.Label)
		return a, err
	})
}

func (s *Store) Grants(ctx context.Context, roleID uuid.UUID) ([]grant, error) {
	rows, err := s.db.Query(ctx, `SELECT menu_id, action FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (grant, error) {
		var g grant
		err := row.Scan(&g.MenuID, &g.Action)
		return g, err
	})
}

// ReplaceGrants replaces all grants of the role. Run inside a transaction.
func (s *Store) ReplaceGrants(ctx context.Context, roleID uuid.UUID, grants []grant) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	if len(grants) == 0 {
		return nil
	}
	menuIDs := make([]string, len(grants))
	actions := make([]string, len(grants))
	for i, g := range grants {
		menuIDs[i] = g.MenuID.String()
		actions[i] = g.Action
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, menu_id, action)
		SELECT $1, m, a FROM unnest($2::uuid[], $3::text[]) AS t(m, a)`, roleID, menuIDs, actions)
	return err
}

func mapUnique(err error) error {
	if name, ok := database.UniqueViolation(err); ok && name == "roles_name_key" {
		return httpx.FieldError("name", "Nama peran sudah digunakan.")
	}
	return err
}
