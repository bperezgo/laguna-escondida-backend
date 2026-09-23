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
	FindChangedProductResponsibilities(ctx context.Context, since time.Time) ([]dto.ProductResponsibilitySyncPayload, error)
	FindChangedProductIngredients(ctx context.Context, since time.Time) ([]dto.ProductIngredientSyncPayload, error)
	// FindChangedStock is read by the stock pull, not the reference pull. stock.updated_at
	// moves on every sale, so it travels on its own daily channel rather than adding a
	// steady stream of rows to the every-minute reference pull (design D4).
	FindChangedStock(ctx context.Context, since time.Time) ([]dto.StockSyncPayload, error)
}
