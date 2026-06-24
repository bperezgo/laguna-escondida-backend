package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func billOp(t *testing.T, operation dto.SyncOperation, billID string, payload json.RawMessage) *dto.SyncOutboxEntry {
	t.Helper()
	return &dto.SyncOutboxEntry{
		OpID:         uuid.Must(uuid.NewV7()).String(),
		OriginNodeID: uuid.NewString(),
		EntityType:   dto.SyncEntityBill,
		EntityID:     billID,
		Operation:    operation,
		Payload:      payload,
	}
}

func marshalBillPayload(t *testing.T, payload dto.BillSyncPayload) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func TestBillSyncApplier_Apply_Create_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewBillSyncApplier(db.DB)

	productID := seedProduct(t, db)
	billID := uuid.NewString()
	customerID := "CC-" + uuid.NewString()[:8]
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM bill_products WHERE bill_id = ?", billID)
		db.DB.Exec("DELETE FROM bills WHERE id = ?", billID)
		db.DB.Exec("DELETE FROM bill_owners WHERE id = ?", customerID)
	})

	now := time.Now()
	payload := dto.BillSyncPayload{
		ID: billID,
		Customer: &dto.Customer{
			DocumentNumber: customerID,
			DocumentType:   dto.DocumentType("CC"),
			Name:           "Bill Sync Customer",
			Email:          "sync@example.com",
		},
		TotalAmount: decimal.NewFromInt(20),
		VAT:         decimal.Zero,
		ICO:         decimal.Zero,
		Tip:         decimal.Zero,
		Products:    []dto.BillSyncProduct{{ProductID: productID, Quantity: 2}},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	require.NoError(t, applier.Apply(context.Background(), billOp(t, dto.SyncOperationCreate, billID, marshalBillPayload(t, payload))))

	var headerCount, ownerCount, productCount int64
	db.DB.Raw("SELECT COUNT(*) FROM bills WHERE id = ? AND deleted_at IS NULL", billID).Scan(&headerCount)
	db.DB.Raw("SELECT COUNT(*) FROM bill_owners WHERE id = ?", customerID).Scan(&ownerCount)
	db.DB.Raw("SELECT COUNT(*) FROM bill_products WHERE bill_id = ?", billID).Scan(&productCount)
	assert.Equal(t, int64(1), headerCount, "bill header upserted")
	assert.Equal(t, int64(1), ownerCount, "bill_owner upserted")
	assert.Equal(t, int64(1), productCount, "bill line item inserted")

	// Re-apply is idempotent: products are replaced wholesale, still exactly one row.
	require.NoError(t, applier.Apply(context.Background(), billOp(t, dto.SyncOperationCreate, billID, marshalBillPayload(t, payload))))
	db.DB.Raw("SELECT COUNT(*) FROM bill_products WHERE bill_id = ?", billID).Scan(&productCount)
	assert.Equal(t, int64(1), productCount, "re-apply leaves exactly one line item")
}

func TestBillSyncApplier_Apply_Update_SetsInvoiceResult_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewBillSyncApplier(db.DB)

	productID := seedProduct(t, db)
	billID := uuid.NewString()
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM bill_products WHERE bill_id = ?", billID)
		db.DB.Exec("DELETE FROM bills WHERE id = ?", billID)
	})

	now := time.Now()
	createPayload := dto.BillSyncPayload{
		ID:          billID,
		TotalAmount: decimal.NewFromInt(20),
		VAT:         decimal.Zero,
		ICO:         decimal.Zero,
		Tip:         decimal.Zero,
		Products:    []dto.BillSyncProduct{{ProductID: productID, Quantity: 1}},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, applier.Apply(context.Background(), billOp(t, dto.SyncOperationCreate, billID, marshalBillPayload(t, createPayload))))

	cufe := "cufe-abc"
	tascode := "tas-789"
	updatePayload := dto.BillSyncPayload{ID: billID, CUFE: &cufe, Tascode: &tascode}
	require.NoError(t, applier.Apply(context.Background(), billOp(t, dto.SyncOperationUpdate, billID, marshalBillPayload(t, updatePayload))))

	var gotCUFE, gotTascode string
	db.DB.Raw("SELECT cufe FROM bills WHERE id = ?", billID).Scan(&gotCUFE)
	db.DB.Raw("SELECT tascode FROM bills WHERE id = ?", billID).Scan(&gotTascode)
	assert.Equal(t, cufe, gotCUFE, "update writes cufe")
	assert.Equal(t, tascode, gotTascode, "update writes tascode")
}
