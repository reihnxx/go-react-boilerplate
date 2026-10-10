// Package role manages roles and their menu × action permission matrix.
package role

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/audit"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/menu"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/validate"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a role name into a slug ("Kepala Sekolah" → "kepala-sekolah").
func Slugify(name string) string {
	s := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "-"), "-")
	s = strings.TrimLeft(s, "0123456789-")
	if s == "" {
		s = "role"
	}
	if len(s) > 60 {
		s = strings.TrimRight(s[:60], "-")
	}
	return s
}

type Input struct {
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active"`
}

type MatrixMenu struct {
	ID       uuid.UUID    `json:"id"`
	Name     string       `json:"name"`
	Slug     string       `json:"slug"`
	IsActive bool         `json:"is_active"`
	Granted  []string     `json:"granted"`
	Children []MatrixMenu `json:"children"`
}

type Matrix struct {
	Role     Role         `json:"role"`
	ReadOnly bool         `json:"read_only"`
	Actions  []ActionInfo `json:"actions"`
	Menus    []MatrixMenu `json:"menus"`
}

type GrantInput struct {
	MenuID  uuid.UUID `json:"menu_id"`
	Actions []string  `json:"actions"`
}

type Service struct {
	pool  *pgxpool.Pool
	store *Store
	menus *menu.Service
}

func NewService(pool *pgxpool.Pool, menus *menu.Service) *Service {
	return &Service{pool: pool, store: NewStore(pool), menus: menus}
}

func (s *Service) List(ctx context.Context, q httpx.PageQuery, active *bool) ([]Role, int64, error) {
	return s.store.List(ctx, q, active)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Role, error) {
	r, err := s.store.Get(ctx, id)
	if database.IsNoRows(err) {
		return r, httpx.NotFound("Peran tidak ditemukan.")
	}
	return r, err
}

func (s *Service) Actions(ctx context.Context) ([]ActionInfo, error) { return s.store.Actions(ctx) }

func validateName(name string) error {
	errs := validate.New()
	errs.Required("name", name, "Nama peran")
	errs.MaxLength("name", name, 100, "Nama peran")
	return errs.Err()
}

func (s *Service) Create(ctx context.Context, in Input) (Role, error) {
	name := strings.TrimSpace(in.Name)
	if err := validateName(name); err != nil {
		return Role{}, err
	}
	base := Slugify(name)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.store.SlugExists(ctx, slug)
		if err != nil {
			return Role{}, err
		}
		if !exists {
			break
		}
		slug = base + "-" + strconv.Itoa(i)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Role{}, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).Insert(ctx, id, name, slug); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "role.create", EntityType: "role", EntityID: id.String(),
			Metadata: map[string]any{"name": name, "slug": slug}})
	})
	if err != nil {
		return Role{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (Role, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Role{}, err
	}
	name := strings.TrimSpace(in.Name)
	if err := validateName(name); err != nil {
		return Role{}, err
	}
	active := current.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	if !active && current.IsSystem {
		return Role{}, httpx.Conflict("Peran sistem tidak dapat dinonaktifkan.")
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).Update(ctx, id, name, active); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "role.update", EntityType: "role", EntityID: id.String(),
			Metadata: map[string]any{"name": name, "is_active": active}})
	})
	if err != nil {
		return Role{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.IsSystem {
		return httpx.Conflict("Peran sistem tidak dapat dihapus.")
	}
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).SoftDelete(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "role.delete", EntityType: "role", EntityID: id.String(),
			Metadata: map[string]any{"name": current.Name}})
	})
}

// Matrix returns the permission matrix for a role.
func (s *Service) Matrix(ctx context.Context, id uuid.UUID) (Matrix, error) {
	r, err := s.Get(ctx, id)
	if err != nil {
		return Matrix{}, err
	}
	actions, err := s.store.Actions(ctx)
	if err != nil {
		return Matrix{}, err
	}
	tree, err := s.menus.Tree(ctx)
	if err != nil {
		return Matrix{}, err
	}
	granted := map[uuid.UUID][]string{}
	if r.IsSuperAdmin {
		for _, n := range flatten(tree) {
			for _, a := range actions {
				granted[n.ID] = append(granted[n.ID], a.Code)
			}
		}
	} else {
		grants, err := s.store.Grants(ctx, id)
		if err != nil {
			return Matrix{}, err
		}
		for _, g := range grants {
			granted[g.MenuID] = append(granted[g.MenuID], g.Action)
		}
	}
	order := map[string]int{}
	for i, a := range actions {
		order[a.Code] = i
	}
	var build func([]menu.Node) []MatrixMenu
	build = func(nodes []menu.Node) []MatrixMenu {
		out := []MatrixMenu{}
		for _, n := range nodes {
			g := sortActions(granted[n.ID], order)
			out = append(out, MatrixMenu{ID: n.ID, Name: n.Name, Slug: n.Slug, IsActive: n.IsActive, Granted: g, Children: build(n.Children)})
		}
		return out
	}
	return Matrix{Role: r, ReadOnly: r.IsSuperAdmin, Actions: actions, Menus: build(tree)}, nil
}

// SetPermissions replaces the role's grants. A non-super-admin actor may only
// add grants they hold themselves, preventing privilege escalation.
func (s *Service) SetPermissions(ctx context.Context, actor *rbac.Permissions, id uuid.UUID, in []GrantInput) (Matrix, error) {
	r, err := s.Get(ctx, id)
	if err != nil {
		return Matrix{}, err
	}
	if r.IsSuperAdmin {
		return Matrix{}, httpx.Conflict("Hak akses Super Admin tidak dapat diubah karena selalu memiliki semua akses.")
	}
	tree, err := s.menus.Tree(ctx)
	if err != nil {
		return Matrix{}, err
	}
	slugs := map[uuid.UUID]string{}
	for _, n := range flatten(tree) {
		slugs[n.ID] = n.Slug
	}
	current, err := s.store.Grants(ctx, id)
	if err != nil {
		return Matrix{}, err
	}
	had := map[grant]bool{}
	for _, g := range current {
		had[g] = true
	}

	seen := map[grant]bool{}
	grants := []grant{}
	for _, item := range in {
		slug, ok := slugs[item.MenuID]
		if !ok {
			return Matrix{}, httpx.FieldError("permissions", "Menu tidak ditemukan: "+item.MenuID.String())
		}
		for _, a := range item.Actions {
			if !rbac.ValidAction(a) {
				return Matrix{}, httpx.FieldError("permissions", "Aksi tidak valid: "+a)
			}
			g := grant{MenuID: item.MenuID, Action: a}
			if seen[g] {
				continue
			}
			seen[g] = true
			if !had[g] && !actor.Can(slug, rbac.Action(a)) {
				return Matrix{}, httpx.Forbidden("Anda tidak dapat memberikan hak akses yang tidak Anda miliki (" + slug + ": " + a + ").")
			}
			grants = append(grants, g)
		}
	}

	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).ReplaceGrants(ctx, id, grants); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "role.permissions", EntityType: "role", EntityID: id.String(),
			Metadata: map[string]any{"grant_count": len(grants)}})
	})
	if err != nil {
		return Matrix{}, err
	}
	return s.Matrix(ctx, id)
}

func flatten(nodes []menu.Node) []menu.Node {
	var out []menu.Node
	for _, n := range nodes {
		out = append(out, n)
		out = append(out, flatten(n.Children)...)
	}
	return out
}

func sortActions(actions []string, order map[string]int) []string {
	out := append([]string{}, actions...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && order[out[j]] < order[out[j-1]]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
