package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PendingInvoiceSyncApplier creates a pending_invoice row on the cloud when the edge
// syncs a bill payment. The row starts with consecutive = NULL and request_payload = NULL;
// the cloud cron fills those in right before the first submission attempt, so all
// consecutive numbers come from the cloud's centralized invoice_sequences counter.
type PendingInvoiceSyncApplier struct {
	db     *gorm.DB
	access ports.OrganizationAccessReader
}

func NewPendingInvoiceSyncApplier(db *gorm.DB) ports.SyncApplier {
	return &PendingInvoiceSyncApplier{db: db}
}

// NewPlanAwarePendingInvoiceSyncApplier marks a bill skipped when its organization's plan excludes
// e-invoicing on receipt. Deciding now, not at submission, keeps an upgrade from submitting old bills.
func NewPlanAwarePendingInvoiceSyncApplier(db *gorm.DB, access ports.OrganizationAccessReader) ports.SyncApplier {
	return &PendingInvoiceSyncApplier{db: db, access: access}
}

func (a *PendingInvoiceSyncApplier) initialStatus(ctx context.Context) (dto.PendingInvoiceStatus, error) {
	if a.access == nil {
		return dto.PendingInvoiceStatusPending, nil
	}
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return "", err
	}
	access, err := a.access.GetAccess(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("read organization plan: %w", err)
	}
	if !plans.Includes(plans.Plan(access.Plan), plans.FeatureEInvoicing) {
		return dto.PendingInvoiceStatusSkipped, nil
	}
	return dto.PendingInvoiceStatusPending, nil
}

func (a *PendingInvoiceSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	var p dto.PendingInvoiceSyncPayload
	if err := json.Unmarshal(op.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal pending invoice sync payload: %w", err)
	}

	status, err := a.initialStatus(ctx)
	if err != nil {
		return err
	}
	db, err := postgres.TenantDB(ctx, a.db)
	if err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true, // idempotent on replay
	}).Create(&pendingInvoiceModel{
		ID:          p.ID,
		BillID:      p.BillID,
		PaymentCode: string(p.PaymentCode),
		Status:      string(status),
		// Consecutive and RequestPayload remain NULL — assigned by cloud cron at submission time.
	}).Error
}
