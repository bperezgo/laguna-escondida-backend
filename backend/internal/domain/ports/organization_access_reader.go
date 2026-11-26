package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// OrganizationAccessReader runs without an organization in ctx: it is how a request learns whether its organization may proceed.
type OrganizationAccessReader interface {
	GetAccess(ctx context.Context, orgID string) (*dto.OrganizationAccess, error)
}
