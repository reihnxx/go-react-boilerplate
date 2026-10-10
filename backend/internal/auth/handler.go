package auth

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

const (
	CookieName = "sid"
	CSRFHeader = "X-CSRF-Token"
)

type Handler struct {
	svc          *Service
	cookieSecure bool
}

func NewHandler(svc *Service, cookieSecure bool) *Handler {
	return &Handler{svc: svc, cookieSecure: cookieSecure}
}

// Routes registers the auth endpoints. guard protects impersonation start.
func (h *Handler) Routes(r chi.Router, guard *rbac.Guard) {
	r.Get("/auth/options", httpx.Handle(h.options))
	r.Post("/auth/login", httpx.Handle(h.login))
	r.Post("/auth/forgot-password", httpx.Handle(h.forgotPassword))
	r.Post("/auth/reset-password", httpx.Handle(h.resetPassword))
	r.Post("/auth/register", httpx.Handle(h.register))

	r.Group(func(r chi.Router) {
		r.Use(RequireAuth)
		r.Post("/auth/logout", httpx.Handle(h.logout))
		r.Get("/auth/me", httpx.Handle(h.me))
		r.Put("/auth/active-role", httpx.Handle(h.setActiveRole))
		r.Post("/auth/impersonation/stop", httpx.Handle(h.stopImpersonation))
		r.With(guard.RequireSuperAdmin()).Post("/users/{id}/impersonate", httpx.Handle(h.startImpersonation))
	})
}

func (h *Handler) setCookie(w http.ResponseWriter, s *IssuedSession) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: s.Token, Path: "/", Expires: s.ExpiresAt,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) respondMe(w http.ResponseWriter, r *http.Request, status int, p *identity.Principal) error {
	me, err := h.svc.Me(r.Context(), p)
	if err != nil {
		return err
	}
	if status == http.StatusCreated {
		httpx.Created(w, me)
	} else {
		httpx.OK(w, me)
	}
	return nil
}

// options exposes public, non-sensitive settings the login screen needs.
func (h *Handler) options(w http.ResponseWriter, r *http.Request) error {
	httpx.OK(w, map[string]any{
		"app_name":             h.svc.cfg.AppName,
		"registration_enabled": h.svc.RegistrationEnabled(),
	})
	return nil
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	var prev []byte
	if p := identity.PrincipalFrom(r.Context()); p != nil {
		prev = p.SessionHash
	}
	s, err := h.svc.Login(r.Context(), in.Login, in.Password, prev)
	if err != nil {
		return err
	}
	h.setCookie(w, s)
	return h.respondMe(w, r, http.StatusOK, s.Principal)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.Logout(r.Context(), identity.PrincipalFrom(r.Context())); err != nil {
		return err
	}
	h.clearCookie(w)
	httpx.NoContent(w)
	return nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	return h.respondMe(w, r, http.StatusOK, identity.PrincipalFrom(r.Context()))
}

func (h *Handler) setActiveRole(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		RoleID string `json:"role_id"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	id, err := uuid.Parse(in.RoleID)
	if err != nil {
		return httpx.FieldError("role_id", "Otoritas wajib dipilih.")
	}
	p := identity.PrincipalFrom(r.Context())
	if err := h.svc.SetActiveRole(r.Context(), p, id); err != nil {
		return err
	}
	return h.respondMe(w, r, http.StatusOK, p)
}

func (h *Handler) startImpersonation(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	s, err := h.svc.StartImpersonation(r.Context(), identity.PrincipalFrom(r.Context()), id)
	if err != nil {
		return err
	}
	h.setCookie(w, s)
	return h.respondMe(w, r, http.StatusOK, s.Principal)
}

func (h *Handler) stopImpersonation(w http.ResponseWriter, r *http.Request) error {
	s, err := h.svc.StopImpersonation(r.Context(), identity.PrincipalFrom(r.Context()))
	if err != nil {
		h.clearCookie(w)
		return err
	}
	h.setCookie(w, s)
	return h.respondMe(w, r, http.StatusOK, s.Principal)
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email string `json:"email"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := h.svc.ForgotPassword(r.Context(), in.Email); err != nil {
		return err
	}
	httpx.OK(w, map[string]string{
		"message": "Jika email terdaftar, tautan reset password telah dikirim.",
	})
	return nil
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token                string `json:"token"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := h.svc.ResetPassword(r.Context(), in.Token, in.Password, in.PasswordConfirmation); err != nil {
		return err
	}
	httpx.OK(w, map[string]string{"message": "Password berhasil direset. Silakan login."})
	return nil
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) error {
	if !h.svc.RegistrationEnabled() {
		return httpx.NotFound("Pendaftaran tidak tersedia.")
	}
	var in RegisterInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := h.svc.Register(r.Context(), in); err != nil {
		return err
	}
	httpx.Created(w, map[string]string{"message": "Berhasil mendaftar! Silakan login untuk masuk."})
	return nil
}

// --- middleware ---

// LoadSession resolves the session cookie into a principal (if valid).
func (h *Handler) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := h.svc.Authenticate(r.Context(), c.Value)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if p == nil {
			h.clearCookie(w)
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.WithPrincipal(r.Context(), p)))
	})
}

// RequireAuth rejects anonymous requests with 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if identity.PrincipalFrom(r.Context()) == nil {
			httpx.WriteError(w, r, httpx.Unauthenticated())
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRF protects state-changing requests: the Origin (or Referer) must be an
// allowed origin when present, and authenticated requests must echo the
// session's CSRF token in the X-CSRF-Token header.
func CSRF(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.ToLower(o)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if origin := requestOrigin(r); origin != "" && !allowed[origin] {
				httpx.WriteError(w, r, httpx.CSRFFailed())
				return
			}
			if p := identity.PrincipalFrom(r.Context()); p != nil {
				token := r.Header.Get(CSRFHeader)
				if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(p.CSRFToken)) != 1 {
					httpx.WriteError(w, r, httpx.CSRFFailed())
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestOrigin(r *http.Request) string {
	// "null" (sandboxed/opaque origins) is returned as-is and therefore rejected.
	if o := r.Header.Get("Origin"); o != "" {
		return strings.ToLower(strings.TrimRight(o, "/"))
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Scheme != "" && u.Host != "" {
			return strings.ToLower(u.Scheme + "://" + u.Host)
		}
	}
	return ""
}
