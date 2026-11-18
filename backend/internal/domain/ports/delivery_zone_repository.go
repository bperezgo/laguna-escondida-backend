package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type DeliveryZoneRepository interface {
	Create(ctx context.Context, zone *dto.DeliveryZone) error
	Update(ctx context.Context, zone *dto.DeliveryZone) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*dto.DeliveryZone, error)
	FindAll(ctx context.Context) ([]*dto.DeliveryZone, error)
	// ListActive returns active, non-deleted zones, oldest first.
	ListActive(ctx context.Context) ([]*dto.DeliveryZone, error)
}

// DeliveryZoneFinder is the read-only slice OrderService needs to place a delivery.
type DeliveryZoneFinder interface {
	ListActive(ctx context.Context) ([]*dto.DeliveryZone, error)
}
