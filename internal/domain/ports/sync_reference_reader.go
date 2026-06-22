package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncReferenceReader is the cloud side of pull: it returns reference rows whose
// updated_at or deleted_at is strictly newer than since, so the edge can replicate the
// changes. Deleted rows are included (with deleted_at set) so soft-deletes propagate.
type SyncReferenceReader interface {
	FindChangedProducts(ctx context.Context, since time.Time) ([]dto.ProductSyncPayload, error)
	FindChangedUsers(ctx context.Context, since time.Time) ([]dto.UserSyncPayload, error)
	FindChangedSuppliers(ctx context.Context, since time.Time) ([]dto.SupplierSyncPayload, error)
}
