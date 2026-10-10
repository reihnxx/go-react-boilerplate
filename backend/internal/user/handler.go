package user

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

// MenuSlug is the permission key for this module.
const MenuSlug = "users"

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router, g *rbac.Guard) {
	r.With(g.Require(MenuSlug, rbac.View)).Get("/users", httpx.Handle(h.list))
	r.With(g.Require(MenuSlug, rbac.View)).Get("/users/{id}", httpx.Handle(h.get))
	r.With(g.Require(MenuSlug, rbac.Create)).Post("/users", httpx.Handle(h.create))
	r.With(g.Require(MenuSlug, rbac.Update)).Patch("/users/{id}", httpx.Handle(h.update))
	r.With(g.Require(MenuSlug, rbac.Update)).Patch("/users/{id}/status", httpx.Handle(h.setStatus))
	r.With(g.Require(MenuSlug, rbac.Update)).Put("/users/{id}/roles", httpx.Handle(h.setRoles))
	r.With(g.Require(MenuSlug, rbac.Update)).Put("/users/{id}/password", httpx.Handle(h.setPassword))
	r.With(g.Require(MenuSlug, rbac.Delete)).Delete("/users/{id}", httpx.Handle(h.delete))
}

func actorFrom(r *http.Request) Actor {
	return Actor{ID: identity.PrincipalFrom(r.Context()).UserID, Perms: rbac.FromContext(r.Context())}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	q, err := httpx.ParsePageQuery(r, listSort)
	if err != nil {
		return err
	}
	var f Filter
	if f.IsActive, err = httpx.OptionalBool(r, "is_active"); err != nil {
		return err
	}
	if s := r.URL.Query().Get("role_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return httpx.BadRequest("Parameter role_id tidak valid.")
		}
		f.RoleID = &id
	}
	users, total, err := h.svc.List(r.Context(), q, f)
	if err != nil {
		return err
	}
	httpx.Paginated(w, users, httpx.NewPageMeta(q, total))
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.OK(w, u)
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var in CreateInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	u, err := h.svc.Create(r.Context(), actorFrom(r), in)
	if err != nil {
		return err
	}
	httpx.Created(w, u)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in UpdateInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	u, err := h.svc.Update(r.Context(), actorFrom(r), id, in)
	if err != nil {
		return err
	}
	httpx.OK(w, u)
	return nil
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		IsActive *bool `json:"is_active"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.IsActive == nil {
		return httpx.FieldError("is_active", "Status wajib diisi.")
	}
	u, err := h.svc.SetStatus(r.Context(), actorFrom(r), id, *in.IsActive)
	if err != nil {
		return err
	}
	httpx.OK(w, u)
	return nil
}

func (h *Handler) setRoles(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		RoleIDs []uuid.UUID `json:"role_ids"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	u, err := h.svc.SetRoles(r.Context(), actorFrom(r), id, in.RoleIDs)
	if err != nil {
		return err
	}
	httpx.OK(w, u)
	return nil
}

func (h *Handler) setPassword(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := h.svc.SetPassword(r.Context(), actorFrom(r), id, in.Password, in.PasswordConfirmation); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), actorFrom(r), id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
