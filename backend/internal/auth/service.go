// Package auth implements login/logout, server-side sessions, active-role
// selection, password reset, self-registration and impersonation.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/audit"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/mailer"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/password"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/validate"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

const (
	msgInvalidCredentials = "Username/email atau kata sandi salah."
	resetTokenTTL         = 60 * time.Minute
	touchInterval         = time.Minute
	mailTimeout           = 30 * time.Second
	// Inputs longer than this are rejected without hashing, so a 1 MiB
	// "password" cannot be used to burn CPU in argon2.
	maxLoginInputBytes = 1024
)

// Registrar creates self-registered users. Implemented by user.Service and
// injected to avoid an import cycle.
type Registrar interface {
	Register(ctx context.Context, name, username, email, plainPassword, roleSlug string) (uuid.UUID, error)
}

type Config struct {
	AppName             string
	BaseURL             string
	SessionIdle         time.Duration
	SessionMaxLife      time.Duration
	MaxAttempts         int
	IPMaxAttempts       int
	Lockout             time.Duration
	RegistrationEnabled bool
	DefaultRoleSlug     string
}

type Service struct {
	pool      *pgxpool.Pool
	store     *Store
	perms     *rbac.Store
	registrar Registrar
	mail      mailer.Mailer
	cfg       Config
	logger    *slog.Logger
}

func NewService(pool *pgxpool.Pool, perms *rbac.Store, registrar Registrar, mail mailer.Mailer, cfg Config, logger *slog.Logger) *Service {
	return &Service{pool: pool, store: NewStore(pool), perms: perms, registrar: registrar, mail: mail, cfg: cfg, logger: logger}
}

// Store exposes the session store (used by the janitor).
func (s *Service) Store() *Store { return s.store }

// IssuedSession is what the handler needs to set the cookie.
type IssuedSession struct {
	Token     string
	ExpiresAt time.Time
	Principal *identity.Principal
}

type MeUser struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
}

// Me is the authenticated-user payload used by the SPA.
type Me struct {
	User                  MeUser                   `json:"user"`
	Roles                 []RoleRef                `json:"roles"`
	ActiveRole            *RoleRef                 `json:"active_role"`
	RequiresRoleSelection bool                     `json:"requires_role_selection"`
	IsSuperAdmin          bool                     `json:"is_super_admin"`
	Permissions           map[string][]rbac.Action `json:"permissions"`
	CSRFToken             string                   `json:"csrf_token"`
	Impersonator          *MeUser                  `json:"impersonator"`
}

func newToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func throttleKey(scope, identifier, ip string) string {
	return scope + "|" + strings.ToLower(strings.TrimSpace(identifier)) + "|" + ip
}

func lockedError(d time.Duration) error {
	secs := int(d.Seconds() + 0.999)
	return httpx.TooManyRequests(fmt.Sprintf("Terlalu banyak percobaan. Coba lagi dalam %d detik.", secs), d)
}

// Login verifies credentials and creates a new session. prevHash is the
// caller's existing session (if any), which is revoked to prevent fixation.
func (s *Service) Login(ctx context.Context, login, plain string, prevHash []byte) (*IssuedSession, error) {
	login = strings.TrimSpace(login)
	errs := validate.New()
	errs.Required("login", login, "Username atau email")
	errs.Required("password", plain, "Kata sandi")
	if err := errs.Err(); err != nil {
		return nil, err
	}

	client := identity.ClientFrom(ctx)
	key := throttleKey("login", login, client.IP)
	ipKey := throttleKey("login-ip", "", client.IP)
	for _, k := range []string{ipKey, key} {
		if d, err := s.store.LockedFor(ctx, k); err != nil {
			return nil, err
		} else if d > 0 {
			return nil, lockedError(d)
		}
	}

	var acc account
	err := pgx.ErrNoRows
	oversized := len(plain) > maxLoginInputBytes || len(login) > maxLoginInputBytes
	if !oversized {
		acc, err = s.store.AccountByLogin(ctx, login)
		if err != nil && !database.IsNoRows(err) {
			return nil, err
		}
	}
	ok := false
	if oversized {
		// fall through to the failure path without hashing
	} else if err == nil {
		match, rehash, verr := password.Verify(plain, acc.PasswordHash)
		if verr != nil {
			s.logger.ErrorContext(ctx, "password verify failed", "user_id", acc.ID, "err", verr)
		}
		ok = match && acc.IsActive
		if match && rehash {
			if h, herr := password.Hash(plain); herr == nil {
				if uerr := s.store.UpdatePasswordHash(ctx, acc.ID, h); uerr != nil {
					s.logger.ErrorContext(ctx, "rehash password", "err", uerr)
				}
			}
		}
	} else {
		password.VerifyDummy(plain)
	}

	if !ok {
		// The per-IP counter is never cleared on success, otherwise one valid
		// account would let an attacker reset it while spraying others.
		ipLock, err := s.store.RegisterFailure(ctx, ipKey, s.cfg.IPMaxAttempts, s.cfg.Lockout)
		if err != nil {
			return nil, err
		}
		userLock, err := s.store.RegisterFailure(ctx, key, s.cfg.MaxAttempts, s.cfg.Lockout)
		if err != nil {
			return nil, err
		}
		if d := max(ipLock, userLock); d > 0 {
			return nil, lockedError(d)
		}
		return nil, httpx.InvalidCredentials(msgInvalidCredentials)
	}
	if err := s.store.ClearAttempts(ctx, key); err != nil {
		return nil, err
	}

	roles, err := s.store.UserRoles(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	var active *uuid.UUID
	if len(roles) == 1 {
		active = &roles[0].ID
	}

	var issued *IssuedSession
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if prevHash != nil {
			if err := st.DeleteSession(ctx, prevHash); err != nil {
				return err
			}
		}
		var err error
		issued, err = s.createSession(ctx, st, acc, active, nil, nil)
		if err != nil {
			return err
		}
		if err := st.TouchLastLogin(ctx, acc.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "auth.login", EntityType: "user", EntityID: acc.ID.String(), ActorID: &acc.ID})
	})
	if err != nil {
		return nil, err
	}
	return issued, nil
}

func (s *Service) createSession(ctx context.Context, st *Store, acc account, activeRole, impersonator, impersonatorRole *uuid.UUID) (*IssuedSession, error) {
	token, hash, err := newToken()
	if err != nil {
		return nil, err
	}
	csrf, _, err := newToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.cfg.SessionIdle)
	client := identity.ClientFrom(ctx)
	row := sessionRow{
		TokenHash: hash, UserID: acc.ID, ActiveRoleID: activeRole,
		ImpersonatorUserID: impersonator, ImpersonatorRoleID: impersonatorRole,
		CSRFToken: csrf, ExpiresAt: expires,
	}
	if err := st.InsertSession(ctx, row, client.IP, client.UserAgent); err != nil {
		return nil, err
	}
	return &IssuedSession{
		Token:     token,
		ExpiresAt: time.Now().Add(s.cfg.SessionMaxLife),
		Principal: principalFrom(row, acc),
	}, nil
}

func principalFrom(row sessionRow, acc account) *identity.Principal {
	return &identity.Principal{
		UserID: acc.ID, Name: acc.Name, Username: acc.Username, Email: acc.Email,
		SessionHash: row.TokenHash, CSRFToken: row.CSRFToken,
		ActiveRoleID: row.ActiveRoleID, ImpersonatorID: row.ImpersonatorUserID, ImpersonatorRoleID: row.ImpersonatorRoleID,
	}
}

// Authenticate resolves a raw session token. It returns (nil, nil) when the
// token is unknown, expired, or belongs to an inactive/deleted user.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (*identity.Principal, error) {
	hash := hashToken(rawToken)
	row, acc, err := s.store.SessionWithAccount(ctx, hash)
	if database.IsNoRows(err) {
		// Unknown, expired or orphaned: make sure nothing is left behind.
		return nil, s.store.DeleteSession(ctx, hash)
	}
	if err != nil {
		return nil, err
	}
	if !acc.IsActive {
		return nil, s.store.DeleteSession(ctx, hash)
	}
	if time.Since(row.LastSeenAt) >= touchInterval {
		if err := s.store.TouchSession(ctx, hash, s.cfg.SessionIdle, s.cfg.SessionMaxLife); err != nil {
			return nil, err
		}
	}
	return principalFrom(row, acc), nil
}

func (s *Service) Logout(ctx context.Context, p *identity.Principal) error {
	if err := s.store.DeleteSession(ctx, p.SessionHash); err != nil {
		return err
	}
	return audit.Record(ctx, s.pool, audit.Entry{Action: "auth.logout", EntityType: "user", EntityID: p.UserID.String()})
}

// Me builds the current-user payload.
func (s *Service) Me(ctx context.Context, p *identity.Principal) (*Me, error) {
	roles, err := s.store.UserRoles(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	me := &Me{
		User:        MeUser{ID: p.UserID, Name: p.Name, Username: p.Username, Email: p.Email},
		Roles:       roles,
		Permissions: map[string][]rbac.Action{},
		CSRFToken:   p.CSRFToken,
	}
	if p.ActiveRoleID != nil {
		for i := range roles {
			if roles[i].ID == *p.ActiveRoleID {
				me.ActiveRole = &roles[i]
			}
		}
	}
	if me.ActiveRole != nil {
		perms, err := s.perms.ForSession(ctx, p.UserID, me.ActiveRole.ID)
		if err != nil && !errors.Is(err, rbac.ErrNoActiveRole) {
			return nil, err
		}
		if perms != nil {
			me.IsSuperAdmin = perms.SuperAdmin
			me.Permissions = perms.Map()
		}
	}
	me.RequiresRoleSelection = me.ActiveRole == nil
	if p.ImpersonatorID != nil {
		imp, err := s.store.AccountByID(ctx, *p.ImpersonatorID)
		if err != nil && !database.IsNoRows(err) {
			return nil, err
		}
		if err == nil {
			me.Impersonator = &MeUser{ID: imp.ID, Name: imp.Name, Username: imp.Username, Email: imp.Email}
		}
	}
	return me, nil
}

// SetActiveRole switches the session's role after verifying the user holds it.
func (s *Service) SetActiveRole(ctx context.Context, p *identity.Principal, roleID uuid.UUID) error {
	roles, err := s.store.UserRoles(ctx, p.UserID)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if r.ID == roleID {
			if err := s.store.SetActiveRole(ctx, p.SessionHash, &roleID); err != nil {
				return err
			}
			p.ActiveRoleID = &roleID
			return audit.Record(ctx, s.pool, audit.Entry{Action: "auth.role_selected", EntityType: "role", EntityID: roleID.String()})
		}
	}
	return httpx.FieldError("role_id", "Otoritas tidak tersedia untuk akun Anda.")
}

// RevokeUserSessions deletes a user's sessions except the one with hash
// except. Used by the user and profile modules.
func (s *Service) RevokeUserSessions(ctx context.Context, userID uuid.UUID, except []byte) error {
	return s.store.DeleteUserSessions(ctx, userID, except)
}

// --- impersonation ---

// StartImpersonation replaces the admin's session with a session for target.
// Route middleware already ensures the actor's active role is super-admin.
func (s *Service) StartImpersonation(ctx context.Context, p *identity.Principal, targetID uuid.UUID) (*IssuedSession, error) {
	if p.IsImpersonating() {
		return nil, httpx.Conflict("Hentikan impersonasi yang sedang berjalan terlebih dahulu.")
	}
	if targetID == p.UserID {
		return nil, httpx.Conflict("Tidak dapat melakukan impersonasi ke akun sendiri.")
	}
	target, err := s.store.AccountByID(ctx, targetID)
	if database.IsNoRows(err) {
		return nil, httpx.NotFound("Pengguna tidak ditemukan.")
	}
	if err != nil {
		return nil, err
	}
	if !target.IsActive {
		return nil, httpx.Conflict("Pengguna tidak aktif.")
	}
	roles, err := s.store.UserRoles(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.IsSuperAdmin {
			return nil, httpx.Forbidden("Tidak dapat melakukan impersonasi ke akun Super Admin.")
		}
	}
	var active *uuid.UUID
	if len(roles) == 1 {
		active = &roles[0].ID
	}

	var issued *IssuedSession
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if err := st.DeleteSession(ctx, p.SessionHash); err != nil {
			return err
		}
		adminID := p.UserID
		var err error
		issued, err = s.createSession(ctx, st, target, active, &adminID, p.ActiveRoleID)
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "auth.impersonation_start", EntityType: "user", EntityID: target.ID.String()})
	})
	return issued, err
}

// StopImpersonation ends the impersonated session and issues a fresh session
// for the original admin with the role they had before.
func (s *Service) StopImpersonation(ctx context.Context, p *identity.Principal) (*IssuedSession, error) {
	if !p.IsImpersonating() {
		return nil, httpx.Conflict("Anda tidak sedang melakukan impersonasi.")
	}
	admin, err := s.store.AccountByID(ctx, *p.ImpersonatorID)
	if err != nil && !database.IsNoRows(err) {
		return nil, err
	}
	adminValid := err == nil && admin.IsActive

	var issued *IssuedSession
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		if err := st.DeleteSession(ctx, p.SessionHash); err != nil {
			return err
		}
		if !adminValid {
			return nil
		}
		var role *uuid.UUID
		if p.ImpersonatorRoleID != nil {
			roles, err := st.UserRoles(ctx, admin.ID)
			if err != nil {
				return err
			}
			for _, r := range roles {
				if r.ID == *p.ImpersonatorRoleID {
					role = p.ImpersonatorRoleID
				}
			}
		}
		var err error
		issued, err = s.createSession(ctx, st, admin, role, nil, nil)
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "auth.impersonation_stop", EntityType: "user",
			EntityID: p.UserID.String(), ActorID: &admin.ID})
	})
	if err != nil {
		return nil, err
	}
	if !adminValid {
		return nil, httpx.Unauthenticated()
	}
	return issued, nil
}

// --- password reset ---

// ForgotPassword emails a reset link if the address belongs to an active
// user. The response is identical either way.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	errs := validate.New()
	errs.Required("email", email, "Email")
	errs.Email("email", email)
	if err := errs.Err(); err != nil {
		return err
	}

	client := identity.ClientFrom(ctx)
	key := throttleKey("forgot", "", client.IP)
	if d, err := s.store.LockedFor(ctx, key); err != nil {
		return err
	} else if d > 0 {
		return lockedError(d)
	}
	// Every request counts, successful or not, to limit email abuse.
	if _, err := s.store.RegisterFailure(ctx, key, s.cfg.MaxAttempts, s.cfg.Lockout); err != nil {
		return err
	}

	acc, err := s.store.AccountByLogin(ctx, email)
	if database.IsNoRows(err) || (err == nil && !acc.IsActive) {
		return nil
	}
	if err != nil {
		return err
	}

	token, hash, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.InsertResetToken(ctx, hash, acc.ID, time.Now().Add(resetTokenTTL)); err != nil {
		return err
	}
	link := s.cfg.BaseURL + "/reset-password?token=" + token
	msg := mailer.Message{
		To:      acc.Email,
		Subject: "Reset Password Akun Anda",
		Text: "Halo " + acc.Name + ",\n\n" +
			"Kami menerima permintaan untuk mereset password akun Anda.\n" +
			"Buka tautan berikut untuk membuat password baru (berlaku 60 menit):\n\n" + link + "\n\n" +
			"Jika Anda tidak merasa meminta reset password, abaikan email ini.\n\n" + s.cfg.AppName,
	}
	// Send in the background so the response time does not reveal whether
	// the address exists (SMTP is slow). WithoutCancel keeps request values
	// such as the request ID but survives the end of the request.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mailTimeout)
	go func() {
		defer cancel()
		if err := s.mail.Send(sendCtx, msg); err != nil {
			s.logger.ErrorContext(sendCtx, "send reset email", "err", err, "user_id", acc.ID)
		}
	}()
	return audit.Record(ctx, s.pool, audit.Entry{Action: "auth.password_reset_requested", EntityType: "user",
		EntityID: acc.ID.String(), ActorID: &acc.ID})
}

// ResetPassword sets a new password using a reset token and revokes every
// session of the user.
func (s *Service) ResetPassword(ctx context.Context, token, plain, confirmation string) error {
	errs := validate.New()
	errs.Required("token", token, "Token")
	if msg := password.Validate(plain); msg != "" {
		errs.Add("password", msg)
	}
	errs.Check(plain == confirmation, "password_confirmation", "Konfirmasi kata sandi tidak cocok.")
	if err := errs.Err(); err != nil {
		return err
	}
	hash, err := password.Hash(plain)
	if err != nil {
		return err
	}
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.WithTx(tx)
		userID, err := st.ConsumeResetToken(ctx, hashToken(token))
		if database.IsNoRows(err) {
			return httpx.FieldError("token", "Tautan reset tidak valid atau sudah kedaluwarsa.")
		}
		if err != nil {
			return err
		}
		if err := st.UpdatePasswordHash(ctx, userID, hash); err != nil {
			return err
		}
		if err := st.DeleteUserSessions(ctx, userID, nil); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{Action: "auth.password_reset", EntityType: "user",
			EntityID: userID.String(), ActorID: &userID})
	})
}

// --- registration ---

type RegisterInput struct {
	Name                 string `json:"name"`
	Username             string `json:"username"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

func (s *Service) RegistrationEnabled() bool { return s.cfg.RegistrationEnabled }

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	if in.Password != in.PasswordConfirmation {
		return httpx.FieldError("password_confirmation", "Konfirmasi kata sandi tidak cocok.")
	}
	id, err := s.registrar.Register(ctx, in.Name, in.Username, in.Email, in.Password, s.cfg.DefaultRoleSlug)
	if err != nil {
		return err
	}
	return audit.Record(ctx, s.pool, audit.Entry{Action: "auth.register", EntityType: "user", EntityID: id.String(), ActorID: &id})
}
