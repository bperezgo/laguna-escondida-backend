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
	"gorm.io/gorm/clause"
)

// StockSyncApplier applies a replicated stock op (edge → cloud) to the cloud's read-only
// on-hand mirror. The edge is the single writer, so create and update are both a plain
// upsert of the current amount keyed by the (product_id, version) composite PK — the last
// snapshot per product wins, so no summation is needed. A delete soft-deletes by product_id.
// It joins the apply transaction via GetTxOrDB.
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
	return a.applyUpsert(ctx, op)
}

func (a *StockSyncApplier) applyUpsert(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var payload dto.StockSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal stock payload: %w", err)
	}

	model := &stockModel{
		ProductID:     payload.ProductID,
		Version:       payload.Version,
		Amount:        payload.Amount,
		UnitOfMeasure: payload.UnitOfMeasure,
		CreatedAt:     payload.CreatedAt,
		UpdatedAt:     payload.UpdatedAt,
		DeletedAt:     payload.DeletedAt,
	}
	// Upsert on the (product_id, version) composite PK so a replayed op is idempotent;
	// deleted_at is written from the snapshot so a resurrected row clears its tombstone.
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "product_id"}, {Name: "version"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"amount", "unit_of_measure", "updated_at", "deleted_at",
		}),
	}).Create(model).Error; err != nil {
		return fmt.Errorf("upsert stock: %w", err)
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
