// Package menu manages menus and menu groups and builds the authorized
// navigation tree for the sidebar.
package menu

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
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/validate"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

var (
	slugRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)
	pathRe   = regexp.MustCompile(`^/[A-Za-z0-9/_:.-]*$`)
	iconRe   = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
)

// SlugFromName derives a permission slug ("Data Siswa" → "data_siswa").
func SlugFromName(name string) string {
	s := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "_"), "_")
	s = strings.TrimLeft(s, "0123456789_")
	if len(s) > 63 {
		s = strings.TrimRight(s[:63], "_")
	}
	return s
}

type Input struct {
	Name      string     `json:"name"`
	Slug      string     `json:"slug"` // create only; ignored on update
	Type      string     `json:"type"` // "main" or "child"
	GroupID   *uuid.UUID `json:"group_id"`
	ParentID  *uuid.UUID `json:"parent_id"`
	Path      string     `json:"path"`
	Icon      string     `json:"icon"`
	SortOrder *int       `json:"sort_order"`
	IsActive  *bool      `json:"is_active"`
}

type Service struct {
	pool  *pgxpool.Pool
	store *Store
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, store: NewStore(pool)}
}

func (s *Service) List(ctx context.Context, q httpx.PageQuery, f Filter) ([]Menu, int64, error) {
	return s.store.List(ctx, q, f)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Menu, error) {
	m, err := s.store.Get(ctx, id)
	if database.IsNoRows(err) {
		return m, httpx.NotFound("Menu tidak ditemukan.")
	}
	return m, err
}

func (s *Service) Groups(ctx context.Context) ([]Group, error) { return s.store.Groups(ctx) }

// validate checks input and returns the fields to persist. self is the menu
// being updated (nil on create).
func (s *Service) validate(ctx context.Context, in Input, self *Menu) (writeFields, error) {
	name := strings.TrimSpace(in.Name)
	path := strings.TrimSpace(in.Path)
	icon := strings.TrimSpace(in.Icon)
	errs := validate.New()
	errs.Required("name", name, "Nama menu")
	errs.MaxLength("name", name, 100, "Nama menu")
	if path != "" {
		errs.MaxLength("path", path, 200, "Path")
		errs.Match("path", path, pathRe, "Path harus diawali \"/\" dan hanya berisi huruf, angka, / _ - : .")
	}
	if icon != "" {
		errs.Match("icon", icon, iconRe, "Ikon tidak valid.")
	}
	if in.SortOrder != nil && *in.SortOrder < 0 {
		errs.Add("sort_order", "Urutan tidak boleh negatif.")
	}

	f := writeFields{Name: name, IsActive: in.IsActive == nil || *in.IsActive}
	if path != "" {
		f.Path = &path
	}
	if icon != "" {
		f.Icon = &icon
	}

	switch in.Type {
	case "main":
		if in.GroupID == nil {
			errs.Add("group_id", "Grup menu wajib diisi untuk menu utama.")
		} else if ok, err := s.store.GroupExists(ctx, *in.GroupID); err != nil {
			return f, err
		} else if !ok {
			errs.Add("group_id", "Grup menu tidak ditemukan.")
		}
		errs.Check(icon != "", "icon", "Ikon wajib diisi untuk menu utama.")
		f.GroupID = in.GroupID
	case "child":
		errs.Check(path != "", "path", "Path wajib diisi untuk submenu.")
		if in.ParentID == nil {
			errs.Add("parent_id", "Menu induk wajib diisi untuk submenu.")
			break
		}
		if self != nil && *in.ParentID == self.ID {
			errs.Add("parent_id", "Menu tidak dapat menjadi induk dirinya sendiri.")
			break
		}
		parent, err := s.store.Get(ctx, *in.ParentID)
		if database.IsNoRows(err) {
			errs.Add("parent_id", "Menu induk tidak ditemukan.")
			break
		}
		if err != nil {
			return f, err
		}
		if parent.ParentID != nil {
			errs.Add("parent_id", "Menu induk harus berupa menu utama (maksimal 2 tingkat).")
		}
		if self != nil {
			n, err := s.store.CountChildren(ctx, self.ID)
			if err != nil {
				return f, err
			}
			if n > 0 {
				errs.Add("type", "Menu yang memiliki submenu tidak dapat dijadikan submenu.")
			}
		}
		f.ParentID = in.ParentID
	default:
		errs.Add("type", "Jenis menu harus main atau child.")
	}
	if err := errs.Err(); err != nil {
		return f, err
	}

	if in.SortOrder != nil {
		f.SortOrder = *in.SortOrder
	} else if self != nil {
		f.SortOrder = self.SortOrder
	} else {
		n, err := s.store.NextSortOrder(ctx, f.GroupID, f.ParentID)
		if err != nil {
			return f, err
		}
		f.SortOrder = n
	}
	return f, nil
}

func (s *Service) Create(ctx context.Context, in Input) (Menu, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = SlugFromName(in.Name)
	}
	f, err := s.validate(ctx, in, nil)
	if err != nil {
		return Menu{}, err
	}
	if !slugRe.MatchString(slug) {
		return Menu{}, httpx.FieldError("slug", "Slug hanya boleh huruf kecil, angka dan garis bawah, diawali huruf (2–63 karakter).")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Menu{}, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).Insert(ctx, id, slug, f); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "menu.create", EntityType: "menu", EntityID: id.String(),
			Metadata: map[string]any{"slug": slug, "name": f.Name}})
	})
	if err != nil {
		return Menu{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (Menu, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Menu{}, err
	}
	f, err := s.validate(ctx, in, &current)
	if err != nil {
		return Menu{}, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).Update(ctx, id, f); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "menu.update", EntityType: "menu", EntityID: id.String(),
			Metadata: map[string]any{"slug": current.Slug}})
	})
	if err != nil {
		return Menu{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	n, err := s.store.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return httpx.Conflict("Hapus submenu terlebih dahulu sebelum menghapus menu ini.")
	}
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.WithTx(tx).SoftDelete(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "menu.delete", EntityType: "menu", EntityID: id.String(),
			Metadata: map[string]any{"slug": current.Slug}})
	})
}

// --- trees ---

// Node is a menu in the full tree (used by the permission matrix).
type Node struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	Slug     string     `json:"slug"`
	Path     *string    `json:"path"`
	IsActive bool       `json:"is_active"`
	GroupID  *uuid.UUID `json:"group_id"`
	Children []Node     `json:"children"`
}

// Tree returns all non-deleted menus as main menus with their children,
// ordered by group and position.
func (s *Service) Tree(ctx context.Context) ([]Node, error) {
	groups, err := s.store.Groups(ctx)
	if err != nil {
		return nil, err
	}
	menus, err := s.store.All(ctx)
	if err != nil {
		return nil, err
	}
	children := map[uuid.UUID][]Node{}
	for _, m := range menus {
		if m.ParentID != nil {
			children[*m.ParentID] = append(children[*m.ParentID], Node{ID: m.ID, Name: m.Name, Slug: m.Slug, Path: m.Path, IsActive: m.IsActive, Children: []Node{}})
		}
	}
	out := []Node{}
	for _, g := range groups {
		for _, m := range menus {
			if m.ParentID == nil && m.GroupID != nil && *m.GroupID == g.ID {
				kids := children[m.ID]
				if kids == nil {
					kids = []Node{}
				}
				out = append(out, Node{ID: m.ID, Name: m.Name, Slug: m.Slug, Path: m.Path, IsActive: m.IsActive, GroupID: m.GroupID, Children: kids})
			}
		}
	}
	return out, nil
}

type NavItem struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Slug     string    `json:"slug"`
	Path     *string   `json:"path"`
	Icon     *string   `json:"icon"`
	Children []NavItem `json:"children"`
}

type NavGroup struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Items []NavItem `json:"items"`
}

// Navigation returns the sidebar for the given permissions.
func (s *Service) Navigation(ctx context.Context, perms *rbac.Permissions) ([]NavGroup, error) {
	groups, err := s.store.Groups(ctx)
	if err != nil {
		return nil, err
	}
	menus, err := s.store.All(ctx)
	if err != nil {
		return nil, err
	}
	return BuildNavigation(groups, menus, perms), nil
}

// BuildNavigation keeps only active menus the role can view. A child is shown
// only if its parent is shown; a parent without a path and without visible
// children is dropped; groups without items are dropped. Input menus must be
// ordered by sort_order.
func BuildNavigation(groups []Group, menus []Menu, perms *rbac.Permissions) []NavGroup {
	visible := func(m Menu) bool { return m.IsActive && perms.Can(m.Slug, rbac.View) }

	children := map[uuid.UUID][]NavItem{}
	for _, m := range menus {
		if m.ParentID != nil && visible(m) {
			children[*m.ParentID] = append(children[*m.ParentID], NavItem{ID: m.ID, Name: m.Name, Slug: m.Slug, Path: m.Path, Icon: m.Icon, Children: []NavItem{}})
		}
	}

	out := []NavGroup{}
	for _, g := range groups {
		items := []NavItem{}
		for _, m := range menus {
			if m.ParentID != nil || m.GroupID == nil || *m.GroupID != g.ID || !visible(m) {
				continue
			}
			kids := children[m.ID]
			if kids == nil {
				kids = []NavItem{}
			}
			if m.Path == nil && len(kids) == 0 {
				continue
			}
			items = append(items, NavItem{ID: m.ID, Name: m.Name, Slug: m.Slug, Path: m.Path, Icon: m.Icon, Children: kids})
		}
		if len(items) > 0 {
			out = append(out, NavGroup{ID: g.ID, Name: g.Name, Items: items})
		}
	}
	return out
}
