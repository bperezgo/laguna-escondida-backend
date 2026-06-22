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

// OpenBillSyncApplier applies a replicated open_bill op to local state: create/update
// upsert the order header and reconcile its line items, while a delete tombstone
// soft-deletes the order and its products. It joins the ambient apply transaction via
// GetTxOrDB, so the whole apply commits or rolls back together with the inbox row the
// sync service records for the op.
type OpenBillSyncApplier struct {
	db *gorm.DB
}

func NewOpenBillSyncApplier(db *gorm.DB) ports.SyncApplier {
	return &OpenBillSyncApplier{db: db}
}

func (a *OpenBillSyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	if op.Operation == dto.SyncOperationDelete {
		return a.applyDelete(ctx, op)
	}
	return a.applyUpsert(ctx, op)
}

func (a *OpenBillSyncApplier) applyDelete(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var tombstone dto.SyncTombstone
	if err := json.Unmarshal(op.Payload, &tombstone); err != nil {
		return fmt.Errorf("unmarshal open_bill tombstone: %w", err)
	}
	id := tombstone.ID
	if id == "" {
		id = op.EntityID
	}

	now := time.Now()
	if err := db.Model(&openBillModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("soft-delete open_bill: %w", err)
	}
	if err := db.Model(&openBillProductModel{}).
		Where("open_bill_id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("soft-delete open_bill products: %w", err)
	}
	return nil
}

func (a *OpenBillSyncApplier) applyUpsert(ctx context.Context, op *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, a.db)

	var payload dto.OpenBillSyncPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal open_bill payload: %w", err)
	}

	header := &openBillModel{
		ID:                 payload.ID,
		TemporalIdentifier: payload.TemporalIdentifier,
		TotalAmount:        payload.TotalAmount,
		Status:             string(payload.Status),
		CreatedBy:          payload.CreatedByID,
		Descriptor:         payload.Descriptor,
		CreatedAt:          payload.CreatedAt,
		UpdatedAt:          payload.UpdatedAt,
	}
	// On conflict, refresh the mutable header columns and clear deleted_at so a
	// re-created order is resurrected. created_by/created_at are immutable.
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"temporal_identifier", "total_amount", "status", "descriptor", "updated_at", "deleted_at",
		}),
	}).Create(header).Error; err != nil {
		return fmt.Errorf("upsert open_bill: %w", err)
	}

	return a.reconcileProducts(db, &payload)
}

// reconcileProducts upserts every line item in the snapshot and soft-deletes any
// existing product no longer present, so the local order matches the peer's snapshot.
// status/area/priority are not in the snapshot, so they are left untouched on update
// and fall back to DB defaults on insert (product-status sub-ops sync separately).
func (a *OpenBillSyncApplier) reconcileProducts(db *gorm.DB, payload *dto.OpenBillSyncPayload) error {
	keepIDs := make([]string, 0, len(payload.Products))
	for _, item := range payload.Products {
		product := &openBillProductModel{
			ID:         item.OpenBillProductID,
			OpenBillID: payload.ID,
			ProductID:  item.ProductID,
			Quantity:   item.Quantity,
			Notes:      item.Notes,
			CreatedAt:  payload.CreatedAt,
			UpdatedAt:  payload.UpdatedAt,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"open_bill_id", "product_id", "quantity", "notes", "updated_at", "deleted_at",
			}),
		}).Create(product).Error; err != nil {
			return fmt.Errorf("upsert open_bill product: %w", err)
		}
		keepIDs = append(keepIDs, item.OpenBillProductID)
	}

	now := time.Now()
	prune := db.Model(&openBillProductModel{}).
		Where("open_bill_id = ? AND deleted_at IS NULL", payload.ID)
	if len(keepIDs) > 0 {
		prune = prune.Where("id NOT IN ?", keepIDs)
	}
	if err := prune.Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("prune open_bill products: %w", err)
	}
	return nil
}
