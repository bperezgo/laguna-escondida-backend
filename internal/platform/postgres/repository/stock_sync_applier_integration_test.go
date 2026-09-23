package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/stockmovement"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// White-box integration tests for the two stock appliers on the cloud: the historic_stock
// applier that folds a replicated movement into on-hand, and the stock applier that must
// never assign an amount a peer reported. Gated behind RUN_INTEGRATION_TESTS like the rest
// of the repository integration suite.

func historicStockOp(t *testing.T, productID string, change int, kind dto.StockMovementKind) *dto.SyncOutboxEntry {
	t.Helper()
	opID := uuid.Must(uuid.NewV7()).String()
	payload, err := json.Marshal(dto.HistoricStockSyncPayload{
		OpID:          opID,
		ProductID:     productID,
		UnitOfMeasure: "unit",
		Change:        change,
		Kind:          kind,
		CreatedAt:     time.Now(),
	})
	require.NoError(t, err)
	return &dto.SyncOutboxEntry{
		OpID:         opID,
		OriginNodeID: uuid.NewString(),
		EntityType:   dto.SyncEntityHistoricStock,
		EntityID:     opID,
		Operation:    dto.SyncOperationCreate,
		Payload:      payload,
	}
}

func stockSnapshotOp(t *testing.T, productID string, version, amount int) *dto.SyncOutboxEntry {
	t.Helper()
	now := time.Now()
	payload, err := json.Marshal(dto.StockSyncPayload{
		ProductID:     productID,
		Version:       version,
		Amount:        amount,
		UnitOfMeasure: "unit",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	require.NoError(t, err)
	return &dto.SyncOutboxEntry{
		OpID:         uuid.Must(uuid.NewV7()).String(),
		OriginNodeID: uuid.NewString(),
		EntityType:   dto.SyncEntityStock,
		EntityID:     productID,
		Operation:    dto.SyncOperationUpdate,
		Payload:      payload,
	}
}

// seedStock gives the product a live on-hand row at amount, cleaned up with the test.
func seedStock(t *testing.T, db *Database, productID string, version, amount int) {
	t.Helper()
	now := time.Now()
	require.NoError(t, db.DB.Create(&stockModel{
		ProductID:     productID,
		Version:       version,
		Amount:        amount,
		UnitOfMeasure: "unit",
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM stock WHERE product_id = ?", productID) })
}

func cleanupStockAndLedger(t *testing.T, db *Database, productID string) {
	t.Helper()
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM historic_stock WHERE product_id = ?", productID)
		db.DB.Exec("DELETE FROM stock WHERE product_id = ?", productID)
	})
}

func currentAmount(t *testing.T, db *Database, productID string) int {
	t.Helper()
	var amount int
	require.NoError(t, db.DB.Raw(
		"SELECT amount FROM stock WHERE product_id = ? AND deleted_at IS NULL", productID,
	).Scan(&amount).Error)
	return amount
}

func ledgerCount(t *testing.T, db *Database, productID string) int {
	t.Helper()
	var count int
	require.NoError(t, db.DB.Raw(
		"SELECT count(*) FROM historic_stock WHERE product_id = ?", productID,
	).Scan(&count).Error)
	return count
}

func TestHistoricStockSyncApplier_Apply_FoldsIntoOnHand_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewHistoricStockSyncApplier(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 30)

	require.NoError(t, applier.Apply(ctx, historicStockOp(t, productID, -3, dto.StockMovementKindSale)))

	assert.Equal(t, 27, currentAmount(t, db, productID))
	assert.Equal(t, 1, ledgerCount(t, db, productID))
}

func TestHistoricStockSyncApplier_Apply_CreatesRowWhenNoneExists_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewHistoricStockSyncApplier(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)

	require.NoError(t, applier.Apply(ctx, historicStockOp(t, productID, 12, dto.StockMovementKindPurchase)))

	assert.Equal(t, 12, currentAmount(t, db, productID))

	var version int
	require.NoError(t, db.DB.Raw("SELECT version FROM stock WHERE product_id = ?", productID).Scan(&version).Error)
	assert.Equal(t, 1, version, "the new row adopts the product's version")
}

func TestHistoricStockSyncApplier_Apply_AllowsNegativeResult_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewHistoricStockSyncApplier(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 2)

	require.NoError(t, applier.Apply(ctx, historicStockOp(t, productID, -5, dto.StockMovementKindSale)))

	assert.Equal(t, -3, currentAmount(t, db, productID), "on-hand is an accounting result; a negative is a real signal")
}

// The same op replayed through ApplyPush must move the amount once. The inbox early-return in
// SyncService is the guarantee; this pins it end to end.
func TestHistoricStockSyncApplier_ApplyPush_ReplayIsExactlyOnce_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 50)

	op := historicStockOp(t, productID, -4, dto.StockMovementKindSale)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_inbox WHERE op_id = ?", op.OpID) })

	syncService := service.NewSyncService(
		postgres.NewUnitOfWork(db.DB),
		NewSyncInboxRepository(db.DB),
		map[dto.SyncEntityType]ports.SyncApplier{
			dto.SyncEntityHistoricStock: NewHistoricStockSyncApplier(db.DB),
		},
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	for range 3 {
		_, err := syncService.ApplyPush(ctx, &dto.SyncPushRequest{Ops: []dto.SyncOutboxEntry{*op}})
		require.NoError(t, err)
	}

	assert.Equal(t, 46, currentAmount(t, db, productID))
	assert.Equal(t, 1, ledgerCount(t, db, productID))
}

// A conflicting ledger insert (same op_id straight into the applier, bypassing the inbox)
// must not fold a second time.
func TestHistoricStockSyncApplier_Apply_ConflictingInsertDoesNotDoubleApply_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewHistoricStockSyncApplier(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 40)

	op := historicStockOp(t, productID, -7, dto.StockMovementKindSale)
	require.NoError(t, applier.Apply(ctx, op))
	require.NoError(t, applier.Apply(ctx, op))

	assert.Equal(t, 33, currentAmount(t, db, productID))
	assert.Equal(t, 1, ledgerCount(t, db, productID))
}

func TestHistoricStockSyncApplier_Apply_OrderIndependent_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewHistoricStockSyncApplier(db.DB)
	ctx := context.Background()

	changes := []int{-3, 12, -5, 20, -1}

	forward := seedProduct(t, db)
	cleanupStockAndLedger(t, db, forward)
	seedStock(t, db, forward, 1, 100)
	for _, change := range changes {
		require.NoError(t, applier.Apply(ctx, historicStockOp(t, forward, change, dto.StockMovementKindSale)))
	}

	reverse := seedProduct(t, db)
	cleanupStockAndLedger(t, db, reverse)
	seedStock(t, db, reverse, 1, 100)
	for i := len(changes) - 1; i >= 0; i-- {
		require.NoError(t, applier.Apply(ctx, historicStockOp(t, reverse, changes[i], dto.StockMovementKindSale)))
	}

	assert.Equal(t, currentAmount(t, db, forward), currentAmount(t, db, reverse))
	assert.Equal(t, 123, currentAmount(t, db, forward))
}

func TestStockSyncApplier_Apply_SnapshotDoesNotAssignAmount_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewStockSyncApplier(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 42)

	require.NoError(t, applier.Apply(ctx, stockSnapshotOp(t, productID, 1, 999)))

	assert.Equal(t, 42, currentAmount(t, db, productID), "the cloud never adopts a peer's on-hand")
}

// A snapshot produced before a cloud-authored purchase must not roll the purchase back.
func TestStockSyncApplier_Apply_SnapshotCannotEraseCloudIncrease_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 10)

	// The edge's view, captured before the office recorded anything.
	staleSnapshot := stockSnapshotOp(t, productID, 1, 10)

	require.NoError(t, NewHistoricStockSyncApplier(db.DB).
		Apply(ctx, historicStockOp(t, productID, 25, dto.StockMovementKindPurchase)))
	require.Equal(t, 35, currentAmount(t, db, productID))

	require.NoError(t, NewStockSyncApplier(db.DB).Apply(ctx, staleSnapshot))

	assert.Equal(t, 35, currentAmount(t, db, productID), "the purchase survives a stale peer snapshot")
}

func TestStockSyncApplier_Apply_DeleteStillTombstones_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	applier := NewStockSyncApplier(db.DB)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 5)

	payload, err := json.Marshal(dto.SyncTombstone{ID: productID})
	require.NoError(t, err)
	require.NoError(t, applier.Apply(ctx, &dto.SyncOutboxEntry{
		OpID:         uuid.Must(uuid.NewV7()).String(),
		OriginNodeID: uuid.NewString(),
		EntityType:   dto.SyncEntityStock,
		EntityID:     productID,
		Operation:    dto.SyncOperationDelete,
		Payload:      payload,
	}))

	var live int64
	require.NoError(t, db.DB.Raw(
		"SELECT count(*) FROM stock WHERE product_id = ? AND deleted_at IS NULL", productID,
	).Scan(&live).Error)
	assert.Equal(t, int64(0), live)
}

// A batch count is a delta against the cloud's own number, so a sale replicated afterwards
// lands on top of it instead of being erased. Exercised through the real StockService and the
// real fold, because that interaction is the whole point of recording a count as a change.
func TestStockService_CountThenReplicatedSale_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	ctx := context.Background()

	productID := seedProduct(t, db)
	cleanupStockAndLedger(t, db, productID)
	seedStock(t, db, productID, 1, 30)

	stockService := service.NewStockService(
		NewStockRepository(db.DB),
		NewProductRepository(db.DB),
		postgres.NewUnitOfWork(db.DB),
		// The cloud authors stock; it queues no op, because nothing consumes one.
		stockmovement.NewNoopEmitter(),
	)

	require.NoError(t, stockService.BulkStockCreationOrUpdating(ctx, &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{{ProductID: productID, Amount: 20}},
	}))
	require.Equal(t, 20, currentAmount(t, db, productID))

	require.NoError(t, NewHistoricStockSyncApplier(db.DB).
		Apply(ctx, historicStockOp(t, productID, -1, dto.StockMovementKindSale)))

	assert.Equal(t, 19, currentAmount(t, db, productID))
}

// ledgerRows returns the product's movements oldest-first, so a test can read what an
// authoring path actually recorded.
func ledgerRows(t *testing.T, db *Database, productID string) []dto.HistoricStock {
	t.Helper()
	var rows []dto.HistoricStock
	require.NoError(t, db.DB.Raw(
		"SELECT product_id, change, kind FROM historic_stock WHERE product_id = ? ORDER BY id", productID,
	).Scan(&rows).Error)
	return rows
}

// Every cloud authoring path records a signed change with the kind that explains it — never an
// absolute. This is what keeps amount == SUM(change) true and what makes the ledger readable.
func TestStockService_AuthoringPathsRecordKindedDeltas_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	ctx := context.Background()

	newStockService := func() *service.StockService {
		return service.NewStockService(
			NewStockRepository(db.DB),
			NewProductRepository(db.DB),
			postgres.NewUnitOfWork(db.DB),
			stockmovement.NewNoopEmitter(),
		)
	}

	t.Run("create stock", func(t *testing.T) {
		productID := seedProduct(t, db)
		cleanupStockAndLedger(t, db, productID)

		_, err := newStockService().CreateStock(ctx, &dto.CreateStockRequest{ProductID: productID, Amount: 40})
		require.NoError(t, err)

		rows := ledgerRows(t, db, productID)
		require.Len(t, rows, 1)
		assert.Equal(t, dto.StockMovementKindAdjustment, rows[0].Kind)
		assert.Equal(t, 40, rows[0].Change)
		assert.Equal(t, 40, currentAmount(t, db, productID))
	})

	t.Run("adjust amount", func(t *testing.T) {
		productID := seedProduct(t, db)
		cleanupStockAndLedger(t, db, productID)
		seedStock(t, db, productID, 1, 40)

		require.NoError(t, newStockService().AddOrDecreaseStock(ctx,
			&dto.AddOrDecreaseStockRequest{ProductID: productID, Change: -15}))

		rows := ledgerRows(t, db, productID)
		require.Len(t, rows, 1)
		assert.Equal(t, dto.StockMovementKindAdjustment, rows[0].Kind)
		assert.Equal(t, -15, rows[0].Change, "the ledger carries the change, not the resulting 25")
		assert.Equal(t, 25, currentAmount(t, db, productID))
	})

	t.Run("batch count", func(t *testing.T) {
		productID := seedProduct(t, db)
		cleanupStockAndLedger(t, db, productID)
		seedStock(t, db, productID, 1, 30)

		require.NoError(t, newStockService().BulkStockCreationOrUpdating(ctx,
			&dto.BulkStockCreationOrUpdatingRequest{Items: []dto.BulkStockItem{{ProductID: productID, Amount: 20}}}))

		rows := ledgerRows(t, db, productID)
		require.Len(t, rows, 1)
		assert.Equal(t, dto.StockMovementKindCount, rows[0].Kind)
		assert.Equal(t, -10, rows[0].Change, "a count of 20 against 30 is recorded as -10, never as 20")
		assert.Equal(t, 20, currentAmount(t, db, productID))
	})

	t.Run("delete writes no movement", func(t *testing.T) {
		productID := seedProduct(t, db)
		cleanupStockAndLedger(t, db, productID)
		seedStock(t, db, productID, 1, 30)

		require.NoError(t, newStockService().DeleteStock(ctx, productID))

		assert.Empty(t, ledgerRows(t, db, productID), "a delete tombstones the row rather than moving it")
	})
}

// newOpeningBalanceService builds the cutover command against the real database, with the
// cloud's no-op emitter — a cutover writes movements, it does not replicate them.
func newOpeningBalanceService(db *Database) *service.StockReconciliationService {
	return service.NewStockReconciliationService(
		NewStockReconciliationRepository(db.DB),
		NewStockRepository(db.DB),
		postgres.NewUnitOfWork(db.DB),
		stockmovement.NewNoopEmitter(),
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)
}

func TestStockReconciliationService_WriteOpeningBalances_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	ctx := context.Background()
	svc := newOpeningBalanceService(db)

	// Adopted from the edge: an amount with no history behind it, the cutover case.
	adopted := seedProduct(t, db)
	cleanupStockAndLedger(t, db, adopted)
	seedStock(t, db, adopted, 1, 64)

	// Already explained: its amount came from a movement the cloud folded.
	explained := seedProduct(t, db)
	cleanupStockAndLedger(t, db, explained)
	require.NoError(t, NewHistoricStockSyncApplier(db.DB).
		Apply(ctx, historicStockOp(t, explained, 15, dto.StockMovementKindSale)))

	// A product with no movements and nothing on hand: nothing to reconcile.
	empty := seedProduct(t, db)
	cleanupStockAndLedger(t, db, empty)
	seedStock(t, db, empty, 1, 0)

	t.Run("first run closes the gap", func(t *testing.T) {
		report, err := svc.WriteOpeningBalances(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, report.CheckedProducts, 3)
		assert.Equal(t, 1, report.WrittenBalances, "only the adopted product needed one")

		rows := ledgerRows(t, db, adopted)
		require.Len(t, rows, 1)
		assert.Equal(t, dto.StockMovementKindOpeningBalance, rows[0].Kind)
		assert.Equal(t, 64, rows[0].Change)

		assert.Len(t, ledgerRows(t, db, empty), 0, "a product with nothing on hand gets no movement")
		assert.Len(t, ledgerRows(t, db, explained), 1, "an already-explained amount is left alone")
	})

	t.Run("the invariant now holds", func(t *testing.T) {
		report, err := svc.Reconcile(ctx)
		require.NoError(t, err)
		for _, divergence := range report.Diverged {
			assert.NotContains(t, []string{adopted, explained, empty}, divergence.ProductID)
		}
	})

	t.Run("a second run writes nothing", func(t *testing.T) {
		report, err := svc.WriteOpeningBalances(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, report.WrittenBalances)
		assert.Len(t, ledgerRows(t, db, adopted), 1, "no duplicate opening balance")
	})
}
