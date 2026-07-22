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

// BillSyncApplier applies a replicated bill op (restaurant → cloud) to local state. A create
// upserts the bill_owner, the bill header, and its line items. An update touches only the
// invoice-result columns (cufe/tascode/document_url) — bills are append-only, so line items
// never change after the bill is finalized. A delete soft-deletes (not expected for bills,
// handled defensively). It joins the apply transaction via GetTxOrDB.
type BillSyncApplier struct {
	db *gorm.DB
}

func NewBillSyncApplier(db *gorm.DB) ports.SyncApplier {
	return &BillSyncApplier{db: db}
}

func (a *BillSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	switch op.Operation {
	case dto.SyncOperationDelete:
		return a.applyDelete(ctx, op)
	case dto.SyncOperationUpdate:
		return a.applyUpdate(ctx, op)
	default:
		return a.applyCreate(ctx, op)
	}
}

func (a *BillSyncApplier) applyCreate(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var payload dto.BillSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal bill payload: %w", err)
	}

	var billOwnerID *string
	if payload.Customer != nil {
		if err := a.upsertBillOwner(db, payload.Customer); err != nil {
			return err
		}
		billOwnerID = &payload.Customer.DocumentNumber
	}

	header := &billModel{
		ID:             payload.ID,
		BillOwnerID:    billOwnerID,
		TotalAmount:    payload.TotalAmount,
		DiscountAmount: payload.DiscountAmount,
		PayAmount:      payload.PayAmount,
		PaymentMethod:  payload.PaymentMethod,
		VAT:            payload.VAT,
		ICO:            payload.ICO,
		Tip:            payload.Tip,
		DocumentURL:    payload.DocumentURL,
		CUFE:           payload.CUFE,
		Tascode:        payload.Tascode,
		CreatedAt:      payload.CreatedAt,
		UpdatedAt:      payload.UpdatedAt,
	}
	// Upsert so a replayed create is idempotent; clear deleted_at to resurrect if needed.
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"bill_owner_id", "total_amount", "discount_amount", "pay_amount", "payment_method",
			"vat", "ico", "tip", "document_url", "cufe", "tascode", "updated_at", "deleted_at",
		}),
	}).Create(header).Error; err != nil {
		return fmt.Errorf("upsert bill: %w", err)
	}

	return a.reconcileProducts(db, &payload)
}

// applyUpdate writes only the invoice-result columns produced when the invoice is submitted.
// A missing bill (update arriving before its create) is a no-op the create will reconcile.
func (a *BillSyncApplier) applyUpdate(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var payload dto.BillSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal bill update payload: %w", err)
	}

	updates := map[string]any{"updated_at": time.Now()}
	if payload.CUFE != nil {
		updates["cufe"] = *payload.CUFE
	}
	if payload.Tascode != nil {
		updates["tascode"] = *payload.Tascode
	}
	if payload.DocumentURL != nil {
		updates["document_url"] = *payload.DocumentURL
	}

	if err := db.Model(&billModel{}).
		Where("id = ?", payload.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update bill invoice result: %w", err)
	}
	return nil
}

func (a *BillSyncApplier) applyDelete(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var tombstone dto.SyncTombstone
	if err := json.Unmarshal(op.Payload, &tombstone); err != nil {
		return fmt.Errorf("unmarshal bill tombstone: %w", err)
	}
	id := tombstone.ID
	if id == "" {
		id = op.EntityID
	}

	now := time.Now()
	if err := db.Model(&billModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("soft-delete bill: %w", err)
	}
	return nil
}

func (a *BillSyncApplier) upsertBillOwner(db *gorm.DB, customer *dto.Customer) error {
	identificationType := string(customer.DocumentType)
	now := time.Now()
	owner := &billOwnerModel{
		ID:                 customer.DocumentNumber,
		Email:              customer.Email,
		Name:               customer.Name,
		IdentificationType: &identificationType,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"email":               owner.Email,
			"name":                owner.Name,
			"identification_type": owner.IdentificationType,
			"updated_at":          now,
		}),
	}).Create(owner).Error; err != nil {
		return fmt.Errorf("upsert bill_owner: %w", err)
	}
	return nil
}

// reconcileProducts replaces the bill's line items with the snapshot. A finalized bill is
// append-only (its items never change), and the snapshot carries no per-line id to upsert on,
// so a wholesale replace is the simplest way to keep a (possibly replayed) create idempotent.
func (a *BillSyncApplier) reconcileProducts(db *gorm.DB, payload *dto.BillSyncPayload) error {
	if err := db.Unscoped().
		Where("bill_id = ?", payload.ID).
		Delete(&billProductModel{}).Error; err != nil {
		return fmt.Errorf("clear bill products: %w", err)
	}

	for _, item := range payload.Products {
		product := &billProductModel{
			BillID:    payload.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			CreatedAt: payload.CreatedAt,
			UpdatedAt: payload.UpdatedAt,
		}
		if err := db.Create(product).Error; err != nil {
			return fmt.Errorf("insert bill product: %w", err)
		}
	}
	return nil
}
