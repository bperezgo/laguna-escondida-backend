package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// OrganizationIntegrationRepository stores the context organization's integrations, one per provider.
type OrganizationIntegrationRepository interface {
	// Upsert creates the provider's integration or replaces its secrets, returning it as stored.
	Upsert(ctx context.Context, integration *dto.OrganizationIntegrationRecord) (*dto.OrganizationIntegrationRecord, error)
	// Get reports found=false when the organization has no integration for the provider.
	Get(ctx context.Context, provider dto.IntegrationProvider) (integration *dto.OrganizationIntegrationRecord, found bool, err error)
	// List is ordered by provider.
	List(ctx context.Context) ([]*dto.OrganizationIntegrationRecord, error)
	// Delete reports deleted=false when there was none.
	Delete(ctx context.Context, provider dto.IntegrationProvider) (deleted bool, err error)
}
