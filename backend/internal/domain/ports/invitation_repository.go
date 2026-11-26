package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

type InvitationRepository interface {
	// Create returns error.ErrInvitationPending when the organization already has a pending invitation for the email.
	Create(ctx context.Context, inv *dto.Invitation, tokenHash string) error
	// ListPending returns the context organization's invitations that are neither accepted nor revoked.
	ListPending(ctx context.Context) ([]*dto.Invitation, error)
	// FindPending returns error.ErrInvitationNotFound for an unknown, accepted or revoked invitation, or one of another organization.
	FindPending(ctx context.Context, id string) (*dto.Invitation, error)
	ReplaceToken(ctx context.Context, id, tokenHash string, expiresAt time.Time) error
	Revoke(ctx context.Context, id string, at time.Time) error
	// FindByTokenHash runs without an organization in ctx: the token is what names it. ForUpdate locks the row in the caller's unit of work.
	FindByTokenHash(ctx context.Context, tokenHash string, forUpdate bool) (*dto.Invitation, error)
	MarkAccepted(ctx context.Context, id string, at time.Time) error
}
