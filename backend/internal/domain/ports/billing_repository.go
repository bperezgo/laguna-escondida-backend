package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// BillingRepository is cloud-only and takes the organization explicitly: the operator reads any
// organization's billing.
type BillingRepository interface {
	// Get reports false when the organization has chosen nothing yet.
	Get(ctx context.Context, organizationID string) (*dto.OrganizationBilling, bool, error)
	// GetMany returns the rows that exist, keyed by organization id.
	GetMany(ctx context.Context, organizationIDs []string) (map[string]*dto.OrganizationBilling, error)
	// SetPeriod creates the row or changes only its period.
	SetPeriod(ctx context.Context, organizationID, period string) error
	// SaveCard creates the row or replaces only its payment source, card and consent.
	SaveCard(ctx context.Context, organizationID string, card dto.BillingCard) error
}
