package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// PlateOptionSlotRepository holds a composite plate's option slots. Reads return only live
// slots and choices, ordered by position; the sync pull reads changes through SyncReferenceReader.
type PlateOptionSlotRepository interface {
	FindByCompositeProductID(ctx context.Context, compositeProductID string) ([]dto.PlateOptionSlot, error)
	// ReplaceForCompositeProduct makes slots the plate's whole list: rows are upserted by id and
	// every live slot, choice or item left out is soft-deleted.
	ReplaceForCompositeProduct(ctx context.Context, compositeProductID string, slots []dto.PlateOptionSlot) error
}
