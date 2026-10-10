package menu

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

// MenuSlug is the permission key for menu management.
const MenuSlug = "menus"

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router, g *rbac.Guard) {
	r.With(g.RequireActiveRole()).Get("/navigation", httpx.Handle(h.navigation))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/menu-groups", httpx.Handle(h.groups))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/menus", httpx.Handle(h.list))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/menus/{id}", httpx.Handle(h.get))
	r.With(g.Require(MenuSlug, rbac.Create)).Post("/menus", httpx.Handle(h.create))
	r.With(g.Require(MenuSlug, rbac.Update)).Patch("/menus/{id}", httpx.Handle(h.update))
	r.With(g.Require(MenuSlug, rbac.Delete)).Delete("/menus/{id}", httpx.Handle(h.delete))
}

func (h *Handler) navigation(w http.ResponseWriter, r *http.Request) error {
	nav, err := h.svc.Navigation(r.Context(), rbac.FromContext(r.Context()))
	if err != nil {
		return err
	}
	httpx.OK(w, nav)
	return nil
}

func (h *Handler) groups(w http.ResponseWriter, r *http.Request) error {
	groups, err := h.svc.Groups(r.Context())
	if err != nil {
		return err
	}
	httpx.OK(w, groups)
	return nil
}

func optionalUUID(r *http.Request, name string) (*uuid.UUID, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, httpx.BadRequest("Parameter " + name + " tidak valid.")
	}
	return &id, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	q, err := httpx.ParsePageQuery(r, listSort)
	if err != nil {
		return err
	}
	f := Filter{Type: r.URL.Query().Get("type")}
	if f.Type != "" && f.Type != "main" && f.Type != "child" {
		return httpx.BadRequest("Parameter type harus main atau child.")
	}
	if f.GroupID, err = optionalUUID(r, "group_id"); err != nil {
		return err
	}
	if f.ParentID, err = optionalUUID(r, "parent_id"); err != nil {
		return err
	}
	if f.IsActive, err = httpx.OptionalBool(r, "is_active"); err != nil {
		return err
	}
	menus, total, err := h.svc.List(r.Context(), q, f)
	if err != nil {
		return err
	}
	httpx.Paginated(w, menus, httpx.NewPageMeta(q, total))
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	m, err := h.svc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.OK(w, m)
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	m, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.Created(w, m)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	m, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	httpx.OK(w, m)
	return nil
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
