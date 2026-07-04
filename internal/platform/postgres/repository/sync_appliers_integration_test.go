package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// White-box integration tests for the sync appliers. They run against the local
// Postgres the migrations were applied to and are gated behind RUN_INTEGRATION_TESTS,
// mirroring the other repository integration tests. Being in package repository lets
// them seed FK referents (user/product/supplier) via the package's own model structs,
// so they track the current schema instead of hand-rolled SQL.

func newApplierTestDB(t *testing.T) *Database {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}
	loadEnvForApplierTest()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		applierEnvOr("DB_HOST", "localhost"),
		applierEnvOr("DB_PORT", "5432"),
		applierEnvOr("DB_USER", "postgres"),
		applierEnvOr("DB_PASSWORD", "postgres"),
		applierEnvOr("DB_NAME", "laguna_escondida"),
		applierEnvOr("DB_SSLMODE", "disable"),
	)

	db, err := NewDatabase(dsn)
	require.NoError(t, err, "connect to test database")
	return db
}

func applierEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvForApplierTest() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		envPath := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(envPath); statErr == nil {
			_ = godotenv.Load(envPath)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func seedUser(t *testing.T, db *Database) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, db.DB.Create(&userModel{
		ID:       id,
		Username: "applier-user-" + id[:8],
		Name:     "Applier Tester",
		Password: "x",
	}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM users WHERE id = ?", id) })
	return id
}

func seedProduct(t *testing.T, db *Database) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, db.DB.Create(&productModel{
		ID:                  id,
		Name:                "Applier Product",
		Category:            "test",
		ProductType:         "SELLABLE",
		UnitOfMeasure:       "unit",
		Version:             1,
		UnitPrice:           decimal.NewFromInt(10),
		VAT:                 decimal.Zero,
		VATAmount:           decimal.Zero,
		ICO:                 decimal.Zero,
		ICOAmount:           decimal.Zero,
		SKU:                 "SKU-" + id[:8],
		TotalPriceWithTaxes: decimal.NewFromInt(10),
	}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM products WHERE id = ?", id) })
	return id
}

func seedSupplier(t *testing.T, db *Database) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, db.DB.Create(&supplierModel{
		ID:   id,
		Name: "Applier Supplier " + id[:8],
	}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM suppliers WHERE id = ?", id) })
	return id
}

func openBillOp(t *testing.T, operation dto.SyncOperation, payload json.RawMessage) *dto.SyncOutboxEntry {
	t.Helper()
	return &dto.SyncOutboxEntry{
		OpID:         uuid.Must(uuid.NewV7()).String(),
		OriginNodeID: uuid.NewString(),
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     uuid.NewString(),
		Operation:    operation,
		Payload:      payload,
	}
}

func marshalOpenBillPayload(t *testing.T, payload dto.OpenBillSyncPayload) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func marshalTombstone(t *testing.T, payload dto.SyncTombstone) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func marshalPurchaseEntryPayload(t *testing.T, payload dto.PurchaseEntry) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func TestOpenBillSyncApplier_Apply_Create_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewOpenBillSyncApplier(db.DB)

	userID := seedUser(t, db)
	productID := seedProduct(t, db)

	billID := uuid.NewString()
	openBillProductID := uuid.NewString()
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM open_bills_products WHERE open_bill_id = ?", billID)
		db.DB.Exec("DELETE FROM open_bills WHERE id = ?", billID)
	})

	now := time.Now()
	payload := dto.OpenBillSyncPayload{
		ID:                 billID,
		TemporalIdentifier: uuid.NewString(),
		TotalAmount:        decimal.NewFromInt(20),
		Status:             dto.CommandStatusCreated,
		CreatedByID:        userID,
		Products: []dto.OpenBillSyncProduct{
			{OpenBillProductID: openBillProductID, ProductID: productID, Quantity: 2, Status: dto.CommandStatusCancelled, Priority: 3},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, applier.Apply(context.Background(), openBillOp(t, dto.SyncOperationCreate, marshalOpenBillPayload(t, payload))))

	var headerCount int64
	db.DB.Raw("SELECT COUNT(*) FROM open_bills WHERE id = ? AND deleted_at IS NULL", billID).Scan(&headerCount)
	assert.Equal(t, int64(1), headerCount, "open_bill header upserted")

	var quantity int
	db.DB.Raw("SELECT quantity FROM open_bills_products WHERE id = ? AND deleted_at IS NULL", openBillProductID).Scan(&quantity)
	assert.Equal(t, 2, quantity, "line item upserted with snapshot quantity")

	// Status/priority ride in the snapshot, so the peer reproduces them exactly instead
	// of falling back to the DB default ('completed'). This is the core sync-bug fix.
	var status string
	db.DB.Raw("SELECT status FROM open_bills_products WHERE id = ? AND deleted_at IS NULL", openBillProductID).Scan(&status)
	assert.Equal(t, string(dto.CommandStatusCancelled), status, "line item status replicated from snapshot, not defaulted")

	var priority int
	db.DB.Raw("SELECT priority FROM open_bills_products WHERE id = ? AND deleted_at IS NULL", openBillProductID).Scan(&priority)
	assert.Equal(t, 3, priority, "line item priority replicated from snapshot")

	// Re-applying the same snapshot is an upsert: state is unchanged, still one row.
	require.NoError(t, applier.Apply(context.Background(), openBillOp(t, dto.SyncOperationCreate, marshalOpenBillPayload(t, payload))))
	var afterReapply int64
	db.DB.Raw("SELECT COUNT(*) FROM open_bills_products WHERE open_bill_id = ? AND deleted_at IS NULL", billID).Scan(&afterReapply)
	assert.Equal(t, int64(1), afterReapply, "re-apply leaves exactly one live line item")
}

func TestOpenBillSyncApplier_Apply_Update_ReconcilesProducts_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewOpenBillSyncApplier(db.DB)

	userID := seedUser(t, db)
	productA := seedProduct(t, db)
	productB := seedProduct(t, db)

	billID := uuid.NewString()
	itemA := uuid.NewString()
	itemB := uuid.NewString()
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM open_bills_products WHERE open_bill_id = ?", billID)
		db.DB.Exec("DELETE FROM open_bills WHERE id = ?", billID)
	})

	now := time.Now()
	create := dto.OpenBillSyncPayload{
		ID: billID, TemporalIdentifier: uuid.NewString(), TotalAmount: decimal.NewFromInt(10),
		Status: dto.CommandStatusCreated, CreatedByID: userID,
		Products: []dto.OpenBillSyncProduct{
			{OpenBillProductID: itemA, ProductID: productA, Quantity: 1, Status: dto.CommandStatusCreated},
			{OpenBillProductID: itemB, ProductID: productB, Quantity: 1, Status: dto.CommandStatusCreated},
		},
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, applier.Apply(context.Background(), openBillOp(t, dto.SyncOperationCreate, marshalOpenBillPayload(t, create))))

	// Update drops itemB, bumps itemA's quantity, and marks itemA completed — the exact
	// shape of a kitchen status transition replicating via an update op.
	update := create
	update.Products = []dto.OpenBillSyncProduct{{OpenBillProductID: itemA, ProductID: productA, Quantity: 5, Status: dto.CommandStatusCompleted}}
	update.TotalAmount = decimal.NewFromInt(50)
	require.NoError(t, applier.Apply(context.Background(), openBillOp(t, dto.SyncOperationUpdate, marshalOpenBillPayload(t, update))))

	var qtyA int
	db.DB.Raw("SELECT quantity FROM open_bills_products WHERE id = ? AND deleted_at IS NULL", itemA).Scan(&qtyA)
	assert.Equal(t, 5, qtyA, "kept item quantity updated from snapshot")

	var statusA string
	db.DB.Raw("SELECT status FROM open_bills_products WHERE id = ? AND deleted_at IS NULL", itemA).Scan(&statusA)
	assert.Equal(t, string(dto.CommandStatusCompleted), statusA, "status transition replicated on update")

	var liveB int64
	db.DB.Raw("SELECT COUNT(*) FROM open_bills_products WHERE id = ? AND deleted_at IS NULL", itemB).Scan(&liveB)
	assert.Equal(t, int64(0), liveB, "removed item soft-deleted")
}

func TestOpenBillSyncApplier_Apply_DeleteTombstone_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewOpenBillSyncApplier(db.DB)

	userID := seedUser(t, db)
	productID := seedProduct(t, db)

	billID := uuid.NewString()
	openBillProductID := uuid.NewString()
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM open_bills_products WHERE open_bill_id = ?", billID)
		db.DB.Exec("DELETE FROM open_bills WHERE id = ?", billID)
	})

	now := time.Now()
	create := dto.OpenBillSyncPayload{
		ID: billID, TemporalIdentifier: uuid.NewString(), TotalAmount: decimal.NewFromInt(10),
		Status: dto.CommandStatusCreated, CreatedByID: userID,
		Products:  []dto.OpenBillSyncProduct{{OpenBillProductID: openBillProductID, ProductID: productID, Quantity: 1, Status: dto.CommandStatusCreated}},
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, applier.Apply(context.Background(), openBillOp(t, dto.SyncOperationCreate, marshalOpenBillPayload(t, create))))

	require.NoError(t, applier.Apply(context.Background(), openBillOp(t, dto.SyncOperationDelete, marshalTombstone(t, dto.SyncTombstone{ID: billID}))))

	var liveHeader int64
	db.DB.Raw("SELECT COUNT(*) FROM open_bills WHERE id = ? AND deleted_at IS NULL", billID).Scan(&liveHeader)
	assert.Equal(t, int64(0), liveHeader, "tombstone soft-deletes the header")

	var liveProducts int64
	db.DB.Raw("SELECT COUNT(*) FROM open_bills_products WHERE open_bill_id = ? AND deleted_at IS NULL", billID).Scan(&liveProducts)
	assert.Equal(t, int64(0), liveProducts, "tombstone soft-deletes the line items")
}

func TestPurchaseEntrySyncApplier_Apply_Create_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewPurchaseEntrySyncApplier(db.DB)

	supplierID := seedSupplier(t, db)
	productID := seedProduct(t, db)

	entryID := uuid.NewString()
	itemID := uuid.NewString()
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM purchase_entry_items WHERE purchase_entry_id = ?", entryID)
		db.DB.Exec("DELETE FROM purchase_entries WHERE id = ?", entryID)
	})

	now := time.Now()
	payload := dto.PurchaseEntry{
		ID:          entryID,
		SupplierID:  supplierID,
		TotalAmount: decimal.NewFromInt(21),
		EntryDate:   now,
		Items: []*dto.PurchaseEntryItem{
			{ID: itemID, PurchaseEntryID: entryID, ProductID: productID, Quantity: decimal.NewFromInt(2), UnitCost: decimal.NewFromFloat(10.5), TotalCost: decimal.NewFromInt(21)},
		},
		CreatedAt: now,
	}

	op := &dto.SyncOutboxEntry{
		OpID: uuid.Must(uuid.NewV7()).String(), OriginNodeID: uuid.NewString(),
		EntityType: dto.SyncEntityPurchaseEntry, EntityID: entryID, Operation: dto.SyncOperationCreate,
		Payload: marshalPurchaseEntryPayload(t, payload),
	}
	require.NoError(t, applier.Apply(context.Background(), op))

	var entryCount int64
	db.DB.Raw("SELECT COUNT(*) FROM purchase_entries WHERE id = ?", entryID).Scan(&entryCount)
	assert.Equal(t, int64(1), entryCount, "purchase entry header upserted")

	var itemCount int64
	db.DB.Raw("SELECT COUNT(*) FROM purchase_entry_items WHERE id = ?", itemID).Scan(&itemCount)
	assert.Equal(t, int64(1), itemCount, "purchase entry item upserted")

	// Idempotent at the applier level: re-applying the snapshot keeps one row each.
	require.NoError(t, applier.Apply(context.Background(), op))
	db.DB.Raw("SELECT COUNT(*) FROM purchase_entry_items WHERE purchase_entry_id = ?", entryID).Scan(&itemCount)
	assert.Equal(t, int64(1), itemCount, "re-apply leaves exactly one item")
}
