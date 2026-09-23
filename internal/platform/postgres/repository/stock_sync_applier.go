package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

// StockSyncApplier handles a replicated stock op (edge → cloud) now that the cloud owns
// on-hand. The cloud derives amount from the movements it folds (HistoricStockSyncApplier),
// so a create/update snapshot carries an amount that is only the peer's opinion: assigning it
// would let a stale snapshot erase a purchase or a count the office recorded here. The upsert
// path is therefore gone and a snapshot is accepted and ignored.
//
// It stays registered rather than being dropped, for two reasons: an edge that has not yet
// shipped the change still emits snapshot ops, and SyncService fails the whole push when no
// applier is registered for an entity type — so unregistering it would stall that edge's
// outbox behind an op nobody can apply. The delete path still runs: a tombstone is a
// deliberate removal, not a reported amount.
type StockSyncApplier struct {
	db *gorm.DB
}

func NewStockSyncApplier(db *gorm.DB) ports.SyncApplier {
	return &StockSyncApplier{db: db}
}

func (a *StockSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	if op.Operation == dto.SyncOperationDelete {
		return a.applyDelete(ctx, op)
	}
	return a.discardSnapshot(op)
}

// discardSnapshot validates the payload and drops it. Unmarshalling is kept so a malformed
// op still surfaces as an error instead of being silently acked.
func (a *StockSyncApplier) discardSnapshot(op *dto.SyncOutboxEntry) error {
	var payload dto.StockSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal stock payload: %w", err)
	}
	return nil
}

func (a *StockSyncApplier) applyDelete(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var tombstone dto.SyncTombstone
	if err := json.Unmarshal(op.Payload, &tombstone); err != nil {
		return fmt.Errorf("unmarshal stock tombstone: %w", err)
	}
	id := tombstone.ID
	if id == "" {
		id = op.EntityID
	}

	now := time.Now()
	if err := db.Model(&stockModel{}).
		Where("product_id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("soft-delete stock: %w", err)
	}
	return nil
}
