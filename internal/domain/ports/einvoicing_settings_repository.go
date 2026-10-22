package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type EInvoicingSettingsRepository interface {
	// Get returns error.ErrEInvoicingNotConfigured when the organization has no settings.
	Get(ctx context.Context, organizationID string) (*dto.EInvoicingSettings, error)
}
