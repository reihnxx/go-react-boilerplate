package role

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

// MenuSlug is the permission key for role management.
const MenuSlug = "roles"

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router, g *rbac.Guard) {
	r.With(g.Require(MenuSlug, rbac.View)).Get("/actions", httpx.Handle(h.actions))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/roles", httpx.Handle(h.list))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/roles/{id}", httpx.Handle(h.get))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/roles/{id}/permissions", httpx.Handle(h.matrix))
	r.With(g.Require(MenuSlug, rbac.Create)).Post("/roles", httpx.Handle(h.create))
	r.With(g.Require(MenuSlug, rbac.Update)).Patch("/roles/{id}", httpx.Handle(h.update))
	r.With(g.Require(MenuSlug, rbac.Update)).Put("/roles/{id}/permissions", httpx.Handle(h.setPermissions))
	r.With(g.Require(MenuSlug, rbac.Delete)).Delete("/roles/{id}", httpx.Handle(h.delete))
}

func (h *Handler) actions(w http.ResponseWriter, r *http.Request) error {
	a, err := h.svc.Actions(r.Context())
	if err != nil {
		return err
	}
	httpx.OK(w, a)
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	q, err := httpx.ParsePageQuery(r, listSort)
	if err != nil {
		return err
	}
	active, err := httpx.OptionalBool(r, "is_active")
	if err != nil {
		return err
	}
	roles, total, err := h.svc.List(r.Context(), q, active)
	if err != nil {
		return err
	}
	httpx.Paginated(w, roles, httpx.NewPageMeta(q, total))
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	role, err := h.svc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.OK(w, role)
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	role, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.Created(w, role)
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
	role, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	httpx.OK(w, role)
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

func (h *Handler) matrix(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	m, err := h.svc.Matrix(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.OK(w, m)
	return nil
}

func (h *Handler) setPermissions(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Permissions []GrantInput `json:"permissions"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	m, err := h.svc.SetPermissions(r.Context(), rbac.FromContext(r.Context()), id, in.Permissions)
	if err != nil {
		return err
	}
	httpx.OK(w, m)
	return nil
}
