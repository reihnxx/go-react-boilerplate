-- +goose Up
CREATE TABLE audit_logs (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id        uuid,
    impersonator_user_id uuid,
    action               text        NOT NULL,
    entity_type          text        NOT NULL,
    entity_id            text,
    metadata             jsonb       NOT NULL DEFAULT '{}',
    ip                   inet,
    user_agent           text,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_user_id);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at);

-- +goose Down
DROP TABLE audit_logs;
