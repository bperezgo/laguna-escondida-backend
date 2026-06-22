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

// PurchaseEntrySyncApplier applies a replicated purchase_entry op as an upsert of the
// entry header and its items. Purchase entries are create-only in this system (the
// table has no soft-delete column), so a delete op is unexpected and rejected. It joins
// the ambient apply transaction via GetTxOrDB.
type PurchaseEntrySyncApplier struct {
	db *gorm.DB
}

func NewPurchaseEntrySyncApplier(db *gorm.DB) ports.SyncApplier {
	return &PurchaseEntrySyncApplier{db: db}
}

func (a *PurchaseEntrySyncApplier) Apply(ctx context.Context, op *dto.SyncOutboxEntry) error {
	if op.Operation == dto.SyncOperationDelete {
		return fmt.Errorf("purchase_entry does not support delete sync (op %s)", op.OpID)
	}

	db := postgres.GetTxOrDB(ctx, a.db)

	var payload dto.PurchaseEntry
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal purchase_entry payload: %w", err)
	}

	header := &purchaseEntryModel{
		ID:               payload.ID,
		SupplierID:       payload.SupplierID,
		TotalAmount:      payload.TotalAmount,
		InvoiceReference: payload.InvoiceReference,
		EntryDate:        payload.EntryDate,
		Notes:            payload.Notes,
		CreatedAt:        payload.CreatedAt,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"supplier_id", "total_amount", "invoice_reference", "entry_date", "notes",
		}),
	}).Create(header).Error; err != nil {
		return fmt.Errorf("upsert purchase_entry: %w", err)
	}

	for _, item := range payload.Items {
		itemModel := &purchaseEntryItemModel{
			ID:              item.ID,
			PurchaseEntryID: payload.ID,
			ProductID:       item.ProductID,
			Quantity:        item.Quantity,
			UnitCost:        item.UnitCost,
			TotalCost:       item.TotalCost,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"purchase_entry_id", "product_id", "quantity", "unit_cost", "total_cost",
			}),
		}).Create(itemModel).Error; err != nil {
			return fmt.Errorf("upsert purchase_entry item: %w", err)
		}
	}

	return nil
}
