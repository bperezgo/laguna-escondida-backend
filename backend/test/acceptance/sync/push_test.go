package sync

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Push replicates edge-owned data (orders) up to the cloud via the op-log: the edge
// drains its outbox over the real HTTP boundary, the cloud applies and acks, and the
// edge advances its high-water mark. These tests seed a cloud user + product so the
// applier's foreign keys resolve, queue outbox ops on the edge, and push synchronously.

func (r *rig) seedCloudUserAndProduct(t *testing.T) (userID, productID string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	u := newUser("waiter", "$2a$10$abcdefghijklmnopqrstuvexamplehashvalue00000000000000", now)
	p := newProduct("SKU-PUSH", "Cafe", now)
	r.seedCloudUsers(u)
	r.seedCloudProducts(p)
	return u.ID, p.ID
}

// SYNC-INV-10 / SYNC-INV-11 — a pushed order lands on the cloud open_bills, the cloud
// records the op in its inbox, the edge stamps the outbox synced, and the per-peer
// high-water mark advances.
func TestSync_Push_OrderLandsOnCloudAndMarksSynced(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)

	entry, orderID := r.openBillOutboxEntry(userID, productID)
	r.appendEdgeOutbox(entry) // Append assigns entry.Seq

	res := r.push()

	assert.Equal(t, 1, res.PushedOps, "one op pushed")
	assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ?", orderID), "order landed on cloud")
	assert.Equal(t, int64(1), r.cloudCount("sync_inbox", "op_id = ?", entry.OpID), "cloud recorded inbox op (SYNC-INV-11)")
	assert.Equal(t, int64(0), r.edgeCount("sync_outbox", "synced_at IS NULL"), "edge outbox drained")
	assert.Equal(t, entry.Seq, r.edgePushedSeq(), "high-water mark advanced to the acked seq")
}

// SYNC-INV-13 — replaying the same push (the lost-ack retry case) applies once: the
// order count is unchanged and the op is deduped via sync_inbox.op_id.
func TestSync_Idempotency_DuplicatePushAppliesOnce(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)

	entry, orderID := r.openBillOutboxEntry(userID, productID)
	req := &dto.SyncPushRequest{NodeID: r.identity.NodeID, Ops: []dto.SyncOutboxEntry{*entry}}

	first := r.cloudApply(req)
	second := r.cloudApply(req) // a retried, previously-acked batch

	assert.Len(t, first.AckedOpIDs, 1, "first apply acks the op")
	assert.Len(t, second.AckedOpIDs, 1, "replay still acks (idempotent)")
	assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ?", orderID), "order applied exactly once")
	assert.Equal(t, int64(1), r.cloudCount("sync_inbox", "op_id = ?", entry.OpID), "deduped in inbox")
}

// SYNC-INV-14 — ops apply in seq order, the high-water mark equals the max acked seq,
// and a subsequent no-op push never regresses it.
func TestSync_Ordering_AppliesInSeqAndMonotonicHWM(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)

	orderIDs := make([]string, 0, 3)
	var maxSeq int64
	for range 3 {
		entry, orderID := r.openBillOutboxEntry(userID, productID)
		r.appendEdgeOutbox(entry)
		orderIDs = append(orderIDs, orderID)
		if entry.Seq > maxSeq {
			maxSeq = entry.Seq
		}
	}

	res := r.push()
	assert.Equal(t, 3, res.PushedOps, "all queued ops pushed")
	for _, id := range orderIDs {
		assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ?", id), "order %s landed", id)
	}
	assert.Equal(t, maxSeq, r.edgePushedSeq(), "HWM equals the max acked seq")
	assert.Equal(t, int64(0), r.edgeCount("sync_outbox", "synced_at IS NULL"), "outbox fully drained")

	res2 := r.push()
	require.Equal(t, 0, res2.PushedOps, "second push is a no-op")
	assert.Equal(t, maxSeq, r.edgePushedSeq(), "HWM stable, never regresses")
}

// seedCloudProduct seeds a single cloud product so the stock applier's product_id FK
// resolves, and returns its id.
func (r *rig) seedCloudProduct(t *testing.T, sku string) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	p := newProduct(sku, "Cafe", now)
	r.seedCloudProducts(p)
	return p.ID
}

// Stock replicates edge → cloud as a state snapshot: the edge is the single writer, so the
// cloud applier upserts the current on-hand amount keyed by (product_id, version). These
// tests seed a cloud product so the FK resolves, then push stock ops over the HTTP boundary.

// SYNC-INV-22 — the cloud owns on-hand, so a snapshot from a peer creates nothing. An edge
// that predates this change still emits these ops; they are acked and dropped, never applied.
func TestSync_Push_StockSnapshotIsNotApplied(t *testing.T) {
	r := newRig(t)
	productID := r.seedCloudProduct(t, "SKU-STOCK-1")

	r.appendEdgeOutbox(r.stockOutboxEntry(productID, 1, 42, dto.SyncOperationCreate))

	res := r.push()

	assert.Equal(t, 1, res.PushedOps, "one op pushed")
	assert.Equal(t, int64(0), r.cloudCount("stock", "product_id = ?", productID), "no row created from a reported amount")
	assert.Equal(t, int64(0), r.edgeCount("sync_outbox", "synced_at IS NULL"), "edge outbox still drains")
}

// SYNC-INV-22 — a snapshot never assigns the amount, however recent it is. The cloud's number
// comes from the movements it folded; adopting a peer's would erase whatever the office did.
func TestSync_Push_StockSnapshotNeverAssignsAmount(t *testing.T) {
	r := newRig(t)
	productID := r.seedCloudProduct(t, "SKU-STOCK-2")

	r.appendEdgeOutbox(r.historicStockOutboxEntry(productID, 30))
	r.appendEdgeOutbox(r.stockOutboxEntry(productID, 1, 87, dto.SyncOperationUpdate))

	res := r.push()

	assert.Equal(t, 2, res.PushedOps, "both ops pushed")
	assert.Equal(t, int64(1), r.cloudCount("stock", "product_id = ?", productID), "single row per product")
	assert.Equal(t, 30, r.cloudStockAmount(productID), "the folded movement stands; the snapshot is ignored")
}

// SYNC-INV-13 — replaying a stock snapshot is still acked and still deduped in the inbox,
// even though applying it is a no-op.
func TestSync_Push_StockSnapshotReplayIsStillAcked(t *testing.T) {
	r := newRig(t)
	productID := r.seedCloudProduct(t, "SKU-STOCK-3")

	entry := r.stockOutboxEntry(productID, 1, 55, dto.SyncOperationCreate)
	req := &dto.SyncPushRequest{NodeID: r.identity.NodeID, Ops: []dto.SyncOutboxEntry{*entry}}

	first := r.cloudApply(req)
	second := r.cloudApply(req) // a retried, previously-acked batch

	assert.Len(t, first.AckedOpIDs, 1, "first apply acks the op")
	assert.Len(t, second.AckedOpIDs, 1, "replay still acks (idempotent)")
	assert.Equal(t, int64(0), r.cloudCount("stock", "product_id = ?", productID), "still nothing assigned")
	assert.Equal(t, int64(1), r.cloudCount("sync_inbox", "op_id = ?", entry.OpID), "deduped in inbox")
}

// A stock delete tombstone still soft-deletes the row: a removal is a deliberate decision,
// not a reported amount, so it is the one snapshot-channel op the cloud still acts on. The
// row it removes is seeded by a movement, since a snapshot no longer creates one.
func TestSync_Push_StockDeleteSoftDeletesMirror(t *testing.T) {
	r := newRig(t)
	productID := r.seedCloudProduct(t, "SKU-STOCK-4")

	r.appendEdgeOutbox(r.historicStockOutboxEntry(productID, 30))
	r.appendEdgeOutbox(r.stockOutboxEntry(productID, 1, 0, dto.SyncOperationDelete))

	res := r.push()

	assert.Equal(t, 2, res.PushedOps, "both ops pushed")
	assert.Equal(t, int64(0), r.cloudCount("stock", "product_id = ? AND deleted_at IS NULL", productID), "mirror soft-deleted")
	assert.Equal(t, int64(1), r.cloudCount("stock", "product_id = ? AND deleted_at IS NOT NULL", productID), "tombstone retained")
}

// The historic_stock movement ledger replicates edge → cloud as append-only create ops: each
// delta lands as its own row (deltas, not absolutes), so the cloud accumulates the full history.

func TestSync_Push_HistoricStockLedgerLandsOnCloud(t *testing.T) {
	r := newRig(t)
	productID := r.seedCloudProduct(t, "SKU-HIST-1")

	r.appendEdgeOutbox(r.historicStockOutboxEntry(productID, -5))
	r.appendEdgeOutbox(r.historicStockOutboxEntry(productID, 12))

	res := r.push()

	assert.Equal(t, 2, res.PushedOps, "both ledger ops pushed")
	assert.Equal(t, int64(2), r.cloudCount("historic_stock", "product_id = ?", productID), "both movements landed")
	assert.Equal(t, int64(1), r.cloudCount("historic_stock", "product_id = ? AND change = ?", productID, -5), "the -5 delta")
	assert.Equal(t, int64(1), r.cloudCount("historic_stock", "product_id = ? AND change = ?", productID, 12), "the +12 delta")
	assert.Equal(t, int64(0), r.edgeCount("sync_outbox", "synced_at IS NULL"), "edge outbox drained")
}

// Replaying a ledger op (lost-ack retry) appends exactly once: deduped by op_id via sync_inbox,
// with ON CONFLICT (op_id) as the applier's safety net.
func TestSync_Push_HistoricStockReplayIsIdempotent(t *testing.T) {
	r := newRig(t)
	productID := r.seedCloudProduct(t, "SKU-HIST-2")

	entry := r.historicStockOutboxEntry(productID, 7)
	req := &dto.SyncPushRequest{NodeID: r.identity.NodeID, Ops: []dto.SyncOutboxEntry{*entry}}

	first := r.cloudApply(req)
	second := r.cloudApply(req) // a retried, previously-acked batch

	assert.Len(t, first.AckedOpIDs, 1, "first apply acks the op")
	assert.Len(t, second.AckedOpIDs, 1, "replay still acks (idempotent)")
	assert.Equal(t, int64(1), r.cloudCount("historic_stock", "op_id = ?", entry.OpID), "appended exactly once")
	assert.Equal(t, int64(1), r.cloudCount("sync_inbox", "op_id = ?", entry.OpID), "deduped in inbox")
}

// An order line's resolved side-dish selections replicate edge → cloud with the open_bill, so
// the cloud mirror reflects what the customer actually got.
func TestSync_Push_OrderLineSideDishesLandOnCloud(t *testing.T) {
	r := newRig(t)
	userID, plateID := r.seedCloudUserAndProduct(t)
	saladID := r.seedCloudProduct(t, "SKU-SD-SALAD")

	entry, orderID := r.openBillOutboxEntryWithSideDishes(userID, plateID, []dto.SideDishSelection{
		{IngredientProductID: saladID, Quantity: 3},
	})
	r.appendEdgeOutbox(entry)

	r.push()

	assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ?", orderID), "order landed on cloud")
	assert.Equal(t, int64(1),
		r.cloudCount("open_bill_product_side_dishes", "ingredient_product_id = ? AND quantity = ?", saladID, 3),
		"side-dish selection replicated to cloud")
}
