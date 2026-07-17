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

// PendingInvoiceSyncApplier creates a pending_invoice row on the cloud when the edge
// syncs a bill payment. The row starts with consecutive = NULL and request_payload = NULL;
// the cloud cron fills those in right before the first submission attempt, so all
// consecutive numbers come from the cloud's centralized invoice_sequences counter.
type PendingInvoiceSyncApplier struct {
	db *gorm.DB
}

func NewPendingInvoiceSyncApplier(db *gorm.DB) ports.SyncApplier {
	return &PendingInvoiceSyncApplier{db: db}
}

func (a *PendingInvoiceSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	var p dto.PendingInvoiceSyncPayload
	if err := json.Unmarshal(op.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal pending invoice sync payload: %w", err)
	}

	db := postgres.GetTxOrDB(ctx, a.db)
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true, // idempotent on replay
	}).Create(&pendingInvoiceModel{
		ID:          p.ID,
		BillID:      p.BillID,
		PaymentCode: string(p.PaymentCode),
		Status:      string(dto.PendingInvoiceStatusPending),
		// Consecutive and RequestPayload remain NULL — assigned by cloud cron at submission time.
	}).Error
}
