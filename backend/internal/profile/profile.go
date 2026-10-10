// Package profile lets the signed-in user view and edit their own account and
// biodata and change their own password. It never accepts a user ID from the
// request; the account is always taken from the session.
package profile

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/audit"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
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

type Profile struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Phone      *string   `json:"phone"`
	BirthPlace *string   `json:"birth_place"`
	BirthDate  *string   `json:"birth_date"` // YYYY-MM-DD
	Address    *string   `json:"address"`
}

type UpdateInput struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	BirthPlace string `json:"birth_place"`
	BirthDate  string `json:"birth_date"`
	Address    string `json:"address"`
}

var phoneRe = regexp.MustCompile(`^(\+62|62|08)[0-9]{8,13}$`)

type Service struct {
	pool     *pgxpool.Pool
	sessions SessionRevoker
}

func NewService(pool *pgxpool.Pool, sessions SessionRevoker) *Service {
	return &Service{pool: pool, sessions: sessions}
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (Profile, error) {
	var p Profile
	var birth *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.username, u.email, pr.phone, pr.birth_place, pr.birth_date, pr.address
		FROM users u LEFT JOIN user_profiles pr ON pr.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).Scan(
		&p.ID, &p.Name, &p.Username, &p.Email, &p.Phone, &p.BirthPlace, &birth, &p.Address)
	if database.IsNoRows(err) {
		return p, httpx.NotFound("")
	}
	if birth != nil {
		d := birth.Format(time.DateOnly)
		p.BirthDate = &d
	}
	return p, err
}

func optional(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

func (s *Service) Update(ctx context.Context, userID uuid.UUID, in UpdateInput) (Profile, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	phone := strings.TrimSpace(in.Phone)
	errs := validate.New()
	errs.Required("name", name, "Nama lengkap")
	errs.MaxLength("name", name, 255, "Nama lengkap")
	errs.Required("email", email, "Email")
	errs.Email("email", email)
	if phone != "" {
		errs.Match("phone", phone, phoneRe, "Format No HP tidak valid (contoh: 08123456789).")
	}
	errs.MaxLength("birth_place", in.BirthPlace, 100, "Tempat lahir")
	errs.MaxLength("address", in.Address, 500, "Alamat")
	var birth *time.Time
	if bd := strings.TrimSpace(in.BirthDate); bd != "" {
		t, err := time.Parse(time.DateOnly, bd)
		switch {
		case err != nil:
			errs.Add("birth_date", "Tanggal lahir tidak valid (format YYYY-MM-DD).")
		case t.After(time.Now()):
			errs.Add("birth_date", "Tanggal lahir tidak boleh di masa depan.")
		default:
			birth = &t
		}
	}
	if err := errs.Err(); err != nil {
		return Profile{}, err
	}

	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET name = $2, email = $3 WHERE id = $1 AND deleted_at IS NULL`, userID, name, email)
		if c, ok := database.UniqueViolation(err); ok && c == "users_email_key" {
			return httpx.FieldError("email", "Email sudah digunakan.")
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO user_profiles (user_id, phone, birth_place, birth_date, address)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id) DO UPDATE SET phone = EXCLUDED.phone, birth_place = EXCLUDED.birth_place,
			    birth_date = EXCLUDED.birth_date, address = EXCLUDED.address`,
			userID, optional(phone), optional(in.BirthPlace), birth, optional(in.Address))
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "profile.update", EntityType: "user", EntityID: userID.String()})
	})
	if err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID)
}

// ChangePassword verifies the current password, stores the new one and
// signs out every other session of the user.
func (s *Service) ChangePassword(ctx context.Context, p *identity.Principal, current, plain, confirmation string) error {
	errs := validate.New()
	errs.Required("current_password", current, "Kata sandi lama")
	if msg := password.Validate(plain); msg != "" {
		errs.Add("password", msg)
	}
	errs.Check(plain == confirmation, "password_confirmation", "Konfirmasi kata sandi tidak cocok.")
	if err := errs.Err(); err != nil {
		return err
	}
	var stored string
	if err := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, p.UserID).Scan(&stored); err != nil {
		return err
	}
	ok, _, err := password.Verify(current, stored)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.FieldError("current_password", "Kata sandi lama salah.")
	}
	hash, err := password.Hash(plain)
	if err != nil {
		return err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, p.UserID, hash); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "profile.password", EntityType: "user", EntityID: p.UserID.String()})
	})
	if err != nil {
		return err
	}
	return s.sessions.RevokeUserSessions(ctx, p.UserID, p.SessionHash)
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router, g *rbac.Guard) {
	r.Group(func(r chi.Router) {
		r.Use(g.RequireActiveRole())
		r.Get("/profile", httpx.Handle(h.get))
		r.Put("/profile", httpx.Handle(h.update))
		r.Put("/profile/password", httpx.Handle(h.changePassword))
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	p, err := h.svc.Get(r.Context(), identity.PrincipalFrom(r.Context()).UserID)
	if err != nil {
		return err
	}
	httpx.OK(w, p)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	var in UpdateInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p, err := h.svc.Update(r.Context(), identity.PrincipalFrom(r.Context()).UserID, in)
	if err != nil {
		return err
	}
	httpx.OK(w, p)
	return nil
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword      string `json:"current_password"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := h.svc.ChangePassword(r.Context(), identity.PrincipalFrom(r.Context()), in.CurrentPassword, in.Password, in.PasswordConfirmation); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
