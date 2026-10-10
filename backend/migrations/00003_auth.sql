-- +goose Up
CREATE TABLE sessions (
    token_hash           bytea PRIMARY KEY,
    user_id              uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    active_role_id       uuid REFERENCES roles (id) ON DELETE SET NULL,
    impersonator_user_id uuid REFERENCES users (id) ON DELETE CASCADE,
    impersonator_role_id uuid REFERENCES roles (id) ON DELETE SET NULL,
    csrf_token           text        NOT NULL,
    ip                   inet,
    user_agent           text,
    created_at           timestamptz NOT NULL DEFAULT now(),
    last_seen_at         timestamptz NOT NULL DEFAULT now(),
    expires_at           timestamptz NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Brute-force throttle (login, forgot password). Key = "<scope>|<identifier>|<ip>".
CREATE TABLE login_attempts (
    key               text PRIMARY KEY,
    attempts          integer     NOT NULL,
    window_started_at timestamptz NOT NULL,
    locked_until      timestamptz
);

CREATE TABLE password_reset_tokens (
    token_hash bytea PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens (user_id);

-- +goose Down
DROP TABLE password_reset_tokens;
DROP TABLE login_attempts;
DROP TABLE sessions;
