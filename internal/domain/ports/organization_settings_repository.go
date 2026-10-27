package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// OrderingMerge turns the stored ordering settings into the ones to store, or refuses with an error.
type OrderingMerge func(current dto.OrderingSettings) (dto.OrderingSettings, error)

// OrganizationSettingsRepository reads and writes the settings of the context's organization.
type OrganizationSettingsRepository interface {
	// Get returns the settings and timezone; zero settings when none are stored.
	Get(ctx context.Context) (*dto.OrganizationSettingsView, error)
	// UpdateOrdering locks the row, applies merge to its ordering and stores the result in one
	// transaction, so concurrent patches don't overwrite each other. It returns what was stored.
	UpdateOrdering(ctx context.Context, merge OrderingMerge) (*dto.OrganizationSettingsView, error)
}
