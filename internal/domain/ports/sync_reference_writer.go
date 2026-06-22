package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncReferenceWriter is the edge side of pull: it upserts the reference rows received
// from the cloud (insert-or-update by id, applying deleted_at so soft-deletes land too).
// Each method is a no-op for an empty slice.
type SyncReferenceWriter interface {
	UpsertProducts(ctx context.Context, products []dto.ProductSyncPayload) error
	UpsertUsers(ctx context.Context, users []dto.UserSyncPayload) error
	UpsertSuppliers(ctx context.Context, suppliers []dto.SupplierSyncPayload) error
}
