// Package audit records security-relevant and data-changing events.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/identity"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
)

type Entry struct {
	Action     string // e.g. "user.create", "auth.login"
	EntityType string // e.g. "user", "role", "menu", "session"
	EntityID   string
	Metadata   map[string]any
	// ActorID overrides the principal from the context (used during login,
	// before a principal exists).
	ActorID *uuid.UUID
}

// Record inserts an audit row using db, so callers can include it in the
// same transaction as the change being audited.
func Record(ctx context.Context, db database.DBTX, e Entry) error {
	var actor, impersonator *uuid.UUID
	if p := identity.PrincipalFrom(ctx); p != nil {
		id := p.UserID
		actor = &id
		impersonator = p.ImpersonatorID
	}
	if e.ActorID != nil {
		actor = e.ActorID
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	var entityID *string
	if e.EntityID != "" {
		entityID = &e.EntityID
	}
	client := identity.ClientFrom(ctx)

	_, err = db.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, impersonator_user_id, action, entity_type, entity_id,
		                        metadata, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, NULLIF($7::text, '')::inet, NULLIF($8::text, ''))`,
		actor, impersonator, e.Action, e.EntityType, entityID, string(meta), client.IP, client.UserAgent)
	return err
}
