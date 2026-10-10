-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    name          text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    username      text        NOT NULL CHECK (char_length(username) BETWEEN 3 AND 50),
    email         text        NOT NULL CHECK (char_length(email) BETWEEN 3 AND 255),
    password_hash text        NOT NULL,
    is_active     boolean     NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE UNIQUE INDEX users_username_key ON users (lower(username)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX users_created_at_idx ON users (created_at);
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE roles (
    id             uuid PRIMARY KEY,
    name           text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    slug           text        NOT NULL CHECK (slug ~ '^[a-z][a-z0-9-]*$'),
    is_super_admin boolean     NOT NULL DEFAULT false,
    is_system      boolean     NOT NULL DEFAULT false,
    is_active      boolean     NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE UNIQUE INDEX roles_name_key ON roles (lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX roles_slug_key ON roles (slug) WHERE deleted_at IS NULL;
-- Exactly one super-admin role may exist.
CREATE UNIQUE INDEX roles_single_super_admin ON roles (is_super_admin)
    WHERE is_super_admin AND deleted_at IS NULL;
CREATE TRIGGER roles_set_updated_at BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_roles (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    uuid        NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);
CREATE INDEX user_roles_role_id_idx ON user_roles (role_id);

CREATE TABLE user_profiles (
    user_id     uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    phone       text,
    birth_place text,
    birth_date  date,
    address     text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER user_profiles_set_updated_at BEFORE UPDATE ON user_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE user_profiles;
DROP TABLE user_roles;
DROP TABLE roles;
DROP TABLE users;
DROP FUNCTION set_updated_at();
