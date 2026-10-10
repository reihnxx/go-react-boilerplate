-- +goose Up
CREATE TABLE menu_groups (
    id         uuid PRIMARY KEY,
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    sort_order integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER menu_groups_set_updated_at BEFORE UPDATE ON menu_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE menus (
    id         uuid PRIMARY KEY,
    group_id   uuid REFERENCES menu_groups (id) ON DELETE RESTRICT,
    parent_id  uuid REFERENCES menus (id) ON DELETE RESTRICT,
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    slug       text        NOT NULL CHECK (slug ~ '^[a-z][a-z0-9_]*$'),
    path       text CHECK (path IS NULL OR path ~ '^/'),
    icon       text,
    sort_order integer     NOT NULL DEFAULT 0,
    is_active  boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT menus_group_or_parent CHECK (parent_id IS NOT NULL OR group_id IS NOT NULL),
    CONSTRAINT menus_not_own_parent CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE UNIQUE INDEX menus_slug_key ON menus (slug) WHERE deleted_at IS NULL;
CREATE INDEX menus_parent_id_idx ON menus (parent_id);
CREATE INDEX menus_group_sort_idx ON menus (group_id, sort_order);
CREATE TRIGGER menus_set_updated_at BEFORE UPDATE ON menus
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE actions (
    code       text PRIMARY KEY,
    label      text    NOT NULL,
    sort_order integer NOT NULL
);

CREATE TABLE role_permissions (
    role_id    uuid        NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    menu_id    uuid        NOT NULL REFERENCES menus (id) ON DELETE CASCADE,
    action     text        NOT NULL REFERENCES actions (code),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, menu_id, action)
);
CREATE INDEX role_permissions_menu_id_idx ON role_permissions (menu_id);

-- +goose Down
DROP TABLE role_permissions;
DROP TABLE actions;
DROP TABLE menus;
DROP TABLE menu_groups;
