// Package identity carries the authenticated principal and client metadata
// through the request context. It has no dependencies on feature packages so
// that auth, rbac and the modules can all share it.
package identity

import (
	"context"

	"github.com/google/uuid"
)

// Principal is the authenticated user behind the current session.
type Principal struct {
	UserID             uuid.UUID
	Name               string
	Username           string
	Email              string
	SessionHash        []byte
	CSRFToken          string
	ActiveRoleID       *uuid.UUID
	ImpersonatorID     *uuid.UUID
	ImpersonatorRoleID *uuid.UUID
}

func (p *Principal) IsImpersonating() bool { return p != nil && p.ImpersonatorID != nil }

// Client describes the caller's network origin.
type Client struct {
	IP        string
	UserAgent string
}

type principalKey struct{}
type clientKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the principal or nil when the request is anonymous.
func PrincipalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}

func WithClient(ctx context.Context, c Client) context.Context {
	return context.WithValue(ctx, clientKey{}, c)
}

func ClientFrom(ctx context.Context) Client {
	c, _ := ctx.Value(clientKey{}).(Client)
	return c
}
