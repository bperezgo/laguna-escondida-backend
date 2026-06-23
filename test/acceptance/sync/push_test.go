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
