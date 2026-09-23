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

// HistoricStockSyncApplier applies a replicated stock movement (edge → cloud): it appends the
// movement to the cloud's append-only historic_stock ledger and folds it into the product's
// on-hand amount. The fold — not a peer's snapshot — is what writes stock.amount on the cloud,
// so on-hand stays the sum of its movements and two movements commute.
//
// Exactly-once comes from the ambient transaction: SyncService marks the op in sync_inbox and
// returns early on a replay, so Apply runs at most once per op_id, atomically with the row that
// proves it ran. The ON CONFLICT (op_id) DO NOTHING below is the safety net for anything that
// reaches the applier outside that path — and because the fold is gated on the insert actually
// happening, a conflicting insert cannot move the amount a second time.
type HistoricStockSyncApplier struct {
	db *gorm.DB
}

func NewHistoricStockSyncApplier(db *gorm.DB) ports.SyncApplier {
	return &HistoricStockSyncApplier{db: db}
}

func (a *HistoricStockSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	var payload dto.HistoricStockSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal historic_stock payload: %w", err)
	}

	opID := payload.OpID
	db := postgres.GetTxOrDB(ctx, a.db)
	inserted := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "op_id"}},
		DoNothing: true,
	}).Create(&historicStockModel{
		OpID:          &opID,
		ProductID:     payload.ProductID,
		UnitOfMeasure: payload.UnitOfMeasure,
		Change:        payload.Change,
		Kind:          string(dto.NormalizeStockMovementKind(payload.Kind)),
		CreatedAt:     payload.CreatedAt,
	})
	if inserted.Error != nil {
		return fmt.Errorf("insert historic_stock: %w", inserted.Error)
	}
	if inserted.RowsAffected == 0 {
		return nil // already in the ledger: its effect is already in the amount
	}

	return a.fold(db, &payload)
}

// fold adds the movement's signed change to the product's live on-hand. A negative result is
// kept: on-hand is an accounting figure here, and a negative one is a real signal.
func (a *HistoricStockSyncApplier) fold(db *gorm.DB, payload *dto.HistoricStockSyncPayload) error {
	now := time.Now()

	folded := db.Model(&stockModel{}).
		Where("product_id = ? AND deleted_at IS NULL", payload.ProductID).
		UpdateColumns(map[string]any{
			"amount":     gorm.Expr("amount + ?", payload.Change),
			"updated_at": now,
		})
	if folded.Error != nil {
		return fmt.Errorf("fold historic_stock into stock: %w", folded.Error)
	}
	if folded.RowsAffected > 0 {
		return nil
	}

	// No live row: this movement is the whole history the cloud holds for the product, so the
	// row starts at its change. version comes from the product because stock's composite PK
	// carries it, even though every read keys on product_id alone. A tombstoned row at that
	// same key takes the delta rather than being resurrected — reviving a deleted stock row
	// is an authoring decision, not something a replicated sale should make.
	if err := db.Exec(`
		INSERT INTO stock (product_id, version, amount, unit_of_measure, created_at, updated_at)
		SELECT p.id, p.version, ?, ?, ?, ?
		FROM products p
		WHERE p.id = ?
		ON CONFLICT (product_id, version) DO UPDATE
		SET amount = stock.amount + EXCLUDED.amount, updated_at = EXCLUDED.updated_at
	`, payload.Change, payload.UnitOfMeasure, now, now, payload.ProductID).Error; err != nil {
		return fmt.Errorf("create stock from historic_stock: %w", err)
	}
	return nil
}
