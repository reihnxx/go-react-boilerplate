package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
)

type Store struct{ db database.DBTX }

func NewStore(db database.DBTX) *Store { return &Store{db: db} }

func (s *Store) WithTx(tx pgx.Tx) *Store { return &Store{db: tx} }

type account struct {
	ID           uuid.UUID
	Name         string
	Username     string
	Email        string
	PasswordHash string
	IsActive     bool
}

// RoleRef is a role assigned to the user.
type RoleRef struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	IsSuperAdmin bool      `json:"is_super_admin"`
}

type sessionRow struct {
	TokenHash          []byte
	UserID             uuid.UUID
	ActiveRoleID       *uuid.UUID
	ImpersonatorUserID *uuid.UUID
	ImpersonatorRoleID *uuid.UUID
	CSRFToken          string
	LastSeenAt         time.Time
	ExpiresAt          time.Time
}

const accountColumns = `id, name, username, email, password_hash, is_active`

func scanAccount(row pgx.Row) (account, error) {
	var a account
	err := row.Scan(&a.ID, &a.Name, &a.Username, &a.Email, &a.PasswordHash, &a.IsActive)
	return a, err
}

// AccountByLogin finds a non-deleted user by email (if login contains "@")
// or username, case-insensitively.
func (s *Store) AccountByLogin(ctx context.Context, login string) (account, error) {
	column := "username"
	if strings.Contains(login, "@") {
		column = "email"
	}
	return scanAccount(s.db.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM users WHERE lower(`+column+`) = lower($1) AND deleted_at IS NULL`, login))
}

func (s *Store) AccountByID(ctx context.Context, id uuid.UUID) (account, error) {
	return scanAccount(s.db.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id))
}

func (s *Store) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash)
	return err
}

func (s *Store) TouchLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
	return err
}

// UserRoles returns the active, non-deleted roles assigned to the user.
func (s *Store) UserRoles(ctx context.Context, userID uuid.UUID) ([]RoleRef, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.name, r.slug, r.is_super_admin
		FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND r.is_active AND r.deleted_at IS NULL
		ORDER BY r.is_super_admin DESC, r.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (RoleRef, error) {
		var r RoleRef
		err := row.Scan(&r.ID, &r.Name, &r.Slug, &r.IsSuperAdmin)
		return r, err
	})
}

// --- sessions ---

func (s *Store) InsertSession(ctx context.Context, row sessionRow, ip, userAgent string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, active_role_id, impersonator_user_id, impersonator_role_id,
		                      csrf_token, ip, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7::text, '')::inet, NULLIF($8::text, ''), $9)`,
		row.TokenHash, row.UserID, row.ActiveRoleID, row.ImpersonatorUserID, row.ImpersonatorRoleID,
		row.CSRFToken, ip, userAgent, row.ExpiresAt)
	return err
}

// SessionWithAccount loads an unexpired session and its (non-deleted) user.
func (s *Store) SessionWithAccount(ctx context.Context, tokenHash []byte) (sessionRow, account, error) {
	var sr sessionRow
	var a account
	err := s.db.QueryRow(ctx, `
		SELECT s.token_hash, s.user_id, s.active_role_id, s.impersonator_user_id, s.impersonator_role_id,
		       s.csrf_token, s.last_seen_at, s.expires_at,
		       u.id, u.name, u.username, u.email, u.password_hash, u.is_active
		FROM sessions s JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL
		WHERE s.token_hash = $1 AND s.expires_at > now()`, tokenHash).Scan(
		&sr.TokenHash, &sr.UserID, &sr.ActiveRoleID, &sr.ImpersonatorUserID, &sr.ImpersonatorRoleID,
		&sr.CSRFToken, &sr.LastSeenAt, &sr.ExpiresAt,
		&a.ID, &a.Name, &a.Username, &a.Email, &a.PasswordHash, &a.IsActive)
	return sr, a, err
}

// TouchSession slides the idle expiry, capped by the absolute lifetime.
func (s *Store) TouchSession(ctx context.Context, tokenHash []byte, idle, maxLife time.Duration) error {
	_, err := s.db.Exec(ctx, `
		UPDATE sessions
		SET last_seen_at = now(),
		    expires_at = LEAST(created_at + make_interval(secs => $3::double precision),
		                       now() + make_interval(secs => $2::double precision))
		WHERE token_hash = $1`, tokenHash, idle.Seconds(), maxLife.Seconds())
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteUserSessions removes all sessions of a user (including sessions where
// the user is the impersonation target), except the one with hash except.
func (s *Store) DeleteUserSessions(ctx context.Context, userID uuid.UUID, except []byte) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM sessions
		WHERE (user_id = $1 OR impersonator_user_id = $1)
		  AND ($2::bytea IS NULL OR token_hash <> $2)`, userID, except)
	return err
}

func (s *Store) SetActiveRole(ctx context.Context, tokenHash []byte, roleID *uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE sessions SET active_role_id = $2 WHERE token_hash = $1`, tokenHash, roleID)
	return err
}

// PurgeExpired removes expired sessions, stale throttle rows and old reset tokens.
func (s *Store) PurgeExpired(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		WITH a AS (DELETE FROM sessions WHERE expires_at <= now()),
		     b AS (DELETE FROM login_attempts
		           WHERE coalesce(locked_until, window_started_at) < now() - interval '1 day'),
		     c AS (DELETE FROM password_reset_tokens WHERE expires_at < now() - interval '1 day')
		SELECT 1`)
	return err
}

// --- throttling ---

// LockedFor returns how long key remains locked (0 when not locked).
func (s *Store) LockedFor(ctx context.Context, key string) (time.Duration, error) {
	var secs float64
	err := s.db.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM (locked_until - now()))
		FROM login_attempts WHERE key = $1 AND locked_until > now()`, key).Scan(&secs)
	if database.IsNoRows(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// RegisterFailure counts a failed attempt within window. When the count reaches
// max the key is locked for window and the lock duration is returned.
func (s *Store) RegisterFailure(ctx context.Context, key string, max int, window time.Duration) (time.Duration, error) {
	var attempts int
	err := s.db.QueryRow(ctx, `
		INSERT INTO login_attempts (key, attempts, window_started_at)
		VALUES ($1, 1, now())
		ON CONFLICT (key) DO UPDATE SET
		    attempts = CASE WHEN login_attempts.window_started_at < now() - make_interval(secs => $2::double precision)
		                    THEN 1 ELSE login_attempts.attempts + 1 END,
		    window_started_at = CASE WHEN login_attempts.window_started_at < now() - make_interval(secs => $2::double precision)
		                    THEN now() ELSE login_attempts.window_started_at END,
		    locked_until = NULL
		RETURNING attempts`, key, window.Seconds()).Scan(&attempts)
	if err != nil {
		return 0, err
	}
	if attempts < max {
		return 0, nil
	}
	_, err = s.db.Exec(ctx, `
		UPDATE login_attempts SET locked_until = now() + make_interval(secs => $2::double precision)
		WHERE key = $1`, key, window.Seconds())
	return window, err
}

func (s *Store) ClearAttempts(ctx context.Context, key string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM login_attempts WHERE key = $1`, key)
	return err
}

// --- password reset ---

func (s *Store) InsertResetToken(ctx context.Context, tokenHash []byte, userID uuid.UUID, expiresAt time.Time) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM password_reset_tokens WHERE user_id = $1`, userID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt)
	return err
}

// ConsumeResetToken marks a valid token as used and returns its user.
func (s *Store) ConsumeResetToken(ctx context.Context, tokenHash []byte) (uuid.UUID, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `
		UPDATE password_reset_tokens SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash).Scan(&userID)
	return userID, err
}
