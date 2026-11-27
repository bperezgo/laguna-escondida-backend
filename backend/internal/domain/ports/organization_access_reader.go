package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// OrganizationAccessReader runs without an organization in ctx: it is how a request learns whether its organization may proceed.
type OrganizationAccessReader interface {
	GetAccess(ctx context.Context, orgID string) (*dto.OrganizationAccess, error)
}

// OrganizationAccessInvalidator makes this replica forget what it knows of an organization's
// access, so a payment or a cancellation applies at once.
type OrganizationAccessInvalidator interface {
	InvalidateAccess(ctx context.Context, orgID string)
}
