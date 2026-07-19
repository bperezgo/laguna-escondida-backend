package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// HistoricStockSyncApplier appends a replicated stock movement (edge → cloud) to the cloud's
// historic_stock ledger. The ledger is append-only, so there is only an insert path; a replayed
// op is a no-op via ON CONFLICT (op_id) DO NOTHING (the inbox already dedups by op_id, this is
// the safety net). It joins the apply transaction via GetTxOrDB.
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
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "op_id"}},
		DoNothing: true,
	}).Create(&historicStockModel{
		OpID:          &opID,
		ProductID:     payload.ProductID,
		UnitOfMeasure: payload.UnitOfMeasure,
		Change:        payload.Change,
		CreatedAt:     payload.CreatedAt,
	}).Error; err != nil {
		return fmt.Errorf("insert historic_stock: %w", err)
	}
	return nil
}
