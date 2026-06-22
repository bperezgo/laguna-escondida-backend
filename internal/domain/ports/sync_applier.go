package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncApplier applies one received op to local state (an upsert for create/update,
// a soft-delete for a tombstone). There is one applier per entity type; the sync
// service dispatches on op.EntityType. Implementations run inside the apply
// transaction and join it via the ambient context.
type SyncApplier interface {
	Apply(ctx context.Context, op *dto.SyncOutboxEntry) error
}
