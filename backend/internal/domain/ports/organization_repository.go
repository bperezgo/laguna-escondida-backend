package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type OrganizationRepository interface {
	// Create returns error.ErrOrganizationSlugTaken when another organization has the slug.
	Create(ctx context.Context, org *dto.Organization) error
	// Get returns error.ErrOrganizationNotFound when no organization has the id.
	Get(ctx context.Context, id string) (*dto.Organization, error)
	// GetBySlug returns error.ErrOrganizationNotFound when no organization has the slug.
	GetBySlug(ctx context.Context, slug string) (*dto.Organization, error)
	// List returns every organization ordered by name.
	List(ctx context.Context) ([]*dto.Organization, error)
	// Update writes only the non-nil fields; an empty optional field is stored as NULL.
	// Returns error.ErrOrganizationNotFound when no organization has the id.
	Update(ctx context.Context, id string, req *dto.UpdateOrganizationRequest) (*dto.Organization, error)
}
