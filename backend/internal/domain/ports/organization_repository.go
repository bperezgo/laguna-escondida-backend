package ports

import (
	"context"
	"time"

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
	// ListByStatus returns the organizations with that status, ordered by name.
	ListByStatus(ctx context.Context, status string) ([]*dto.Organization, error)
	// Update writes only the non-nil fields; an empty optional field is stored as NULL.
	// Returns error.ErrOrganizationNotFound when no organization has the id.
	Update(ctx context.Context, id string, req *dto.UpdateOrganizationRequest) (*dto.Organization, error)
	// StartTrial moves a pending organization to trial ending at trialEndsAt, only if it is still pending,
	// so two concurrent calls can't both start it. Returns error.ErrOrganizationNotPending otherwise.
	StartTrial(ctx context.Context, id string, trialEndsAt time.Time) (*dto.Organization, error)
}
