package menu

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

type Group struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
}

type Menu struct {
	ID         uuid.UUID  `json:"id"`
	Type       string     `json:"type"` // "main" or "child"
	GroupID    *uuid.UUID `json:"group_id"`
	GroupName  *string    `json:"group_name"`
	ParentID   *uuid.UUID `json:"parent_id"`
	ParentName *string    `json:"parent_name"`
	Name       string     `json:"name"`
	Slug       string     `json:"slug"`
	Path       *string    `json:"path"`
	Icon       *string    `json:"icon"`
	SortOrder  int        `json:"sort_order"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Filter struct {
	GroupID  *uuid.UUID
	ParentID *uuid.UUID
	Type     string
	IsActive *bool
}

var listSort = httpx.SortSpec{
	Allowed: map[string]string{
		"name": "m.name", "slug": "m.slug", "sort_order": "m.sort_order", "created_at": "m.created_at",
		// Sidebar order: group, parent position, parents before children, own position.
		"position": "(coalesce(g.sort_order, pg.sort_order), coalesce(p.sort_order, m.sort_order), coalesce(p.name, m.name), m.parent_id IS NOT NULL, m.sort_order)",
	},
	Default: "position",
}

type Store struct{ db database.DBTX }

func NewStore(db database.DBTX) *Store { return &Store{db: db} }

func (s *Store) WithTx(tx pgx.Tx) *Store { return &Store{db: tx} }

const menuSelect = `
	SELECT m.id, m.group_id, coalesce(g.name, pg.name), m.parent_id, p.name, m.name, m.slug, m.path, m.icon,
	       m.sort_order, m.is_active, m.created_at, m.updated_at`

const menuFrom = `
	FROM menus m
	LEFT JOIN menu_groups g ON g.id = m.group_id
	LEFT JOIN menus p ON p.id = m.parent_id
	LEFT JOIN menu_groups pg ON pg.id = p.group_id`

func scanMenu(row pgx.Row, extra ...any) (Menu, error) {
	var m Menu
	dest := append([]any{&m.ID, &m.GroupID, &m.GroupName, &m.ParentID, &m.ParentName, &m.Name, &m.Slug,
		&m.Path, &m.Icon, &m.SortOrder, &m.IsActive, &m.CreatedAt, &m.UpdatedAt}, extra...)
	err := row.Scan(dest...)
	m.Type = "main"
	if m.ParentID != nil {
		m.Type = "child"
	}
	return m, err
}

func (s *Store) List(ctx context.Context, q httpx.PageQuery, f Filter) ([]Menu, int64, error) {
	where := []string{"m.deleted_at IS NULL"}
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	if q.Search != "" {
		p := arg(httpx.LikePattern(q.Search))
		where = append(where, fmt.Sprintf("(m.name ILIKE %[1]s OR m.slug ILIKE %[1]s OR m.path ILIKE %[1]s)", p))
	}
	if f.GroupID != nil {
		where = append(where, "coalesce(m.group_id, p.group_id) = "+arg(*f.GroupID))
	}
	if f.ParentID != nil {
		where = append(where, "m.parent_id = "+arg(*f.ParentID))
	}
	switch f.Type {
	case "main":
		where = append(where, "m.parent_id IS NULL")
	case "child":
		where = append(where, "m.parent_id IS NOT NULL")
	}
	if f.IsActive != nil {
		where = append(where, "m.is_active = "+arg(*f.IsActive))
	}
	sql := menuSelect + `, count(*) OVER ()` + menuFrom + ` WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY ` + q.OrderBy() + `, m.id LIMIT ` + arg(q.PerPage) + ` OFFSET ` + arg(q.Offset())
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var total int64
	out := []Menu{}
	for rows.Next() {
		m, err := scanMenu(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Menu, error) {
	return scanMenu(s.db.QueryRow(ctx, menuSelect+menuFrom+` WHERE m.id = $1 AND m.deleted_at IS NULL`, id))
}

// All returns every non-deleted menu ordered for tree building.
func (s *Store) All(ctx context.Context) ([]Menu, error) {
	rows, err := s.db.Query(ctx, menuSelect+menuFrom+` WHERE m.deleted_at IS NULL ORDER BY m.sort_order, m.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Menu{}
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Groups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, sort_order FROM menu_groups ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Group, error) {
		var g Group
		err := row.Scan(&g.ID, &g.Name, &g.SortOrder)
		return g, err
	})
}

func (s *Store) GroupExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM menu_groups WHERE id = $1)`, id).Scan(&ok)
	return ok, err
}

func (s *Store) CountChildren(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM menus WHERE parent_id = $1 AND deleted_at IS NULL`, id).Scan(&n)
	return n, err
}

// NextSortOrder returns max(sort_order)+10 among the siblings.
func (s *Store) NextSortOrder(ctx context.Context, groupID, parentID *uuid.UUID) (int, error) {
	var n int
	var err error
	if parentID != nil {
		err = s.db.QueryRow(ctx, `SELECT coalesce(max(sort_order), 0) + 10 FROM menus WHERE parent_id = $1 AND deleted_at IS NULL`, *parentID).Scan(&n)
	} else {
		err = s.db.QueryRow(ctx, `SELECT coalesce(max(sort_order), 0) + 10 FROM menus WHERE parent_id IS NULL AND group_id = $1 AND deleted_at IS NULL`, groupID).Scan(&n)
	}
	return n, err
}

type writeFields struct {
	GroupID   *uuid.UUID
	ParentID  *uuid.UUID
	Name      string
	Path      *string
	Icon      *string
	SortOrder int
	IsActive  bool
}

func (s *Store) Insert(ctx context.Context, id uuid.UUID, slug string, f writeFields) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO menus (id, group_id, parent_id, name, slug, path, icon, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, f.GroupID, f.ParentID, f.Name, slug, f.Path, f.Icon, f.SortOrder, f.IsActive)
	if name, ok := database.UniqueViolation(err); ok && name == "menus_slug_key" {
		return httpx.FieldError("slug", "Slug sudah digunakan.")
	}
	return err
}

func (s *Store) Update(ctx context.Context, id uuid.UUID, f writeFields) error {
	_, err := s.db.Exec(ctx, `
		UPDATE menus SET group_id = $2, parent_id = $3, name = $4, path = $5, icon = $6, sort_order = $7, is_active = $8
		WHERE id = $1 AND deleted_at IS NULL`,
		id, f.GroupID, f.ParentID, f.Name, f.Path, f.Icon, f.SortOrder, f.IsActive)
	return err
}

// SoftDelete marks the menu deleted and removes all permissions on it.
func (s *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM role_permissions WHERE menu_id = $1`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE menus SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}
