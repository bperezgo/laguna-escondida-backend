package sync

import (
	"encoding/json"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stock flows in two directions now. Upward, the restaurant sends signed movements that the
// cloud folds into the amount it owns. Downward, the restaurant adopts the cloud's numbers
// once a day. These tests drive both halves over the real HTTP boundary, synchronously.

// SYNC-INV-23 — a sale recorded at the restaurant reduces the cloud's on-hand by exactly the
// amount sold, and lands in the cloud's ledger.
func TestSync_Stock_CloudFoldsEdgeSales(t *testing.T) {
	r := newRig(t)
	productID := r.seedBothNodesProduct("SKU-FOLD-1", "Cerveza")

	r.seedCloudStock(productID, 50)
	r.seedEdgeStock(productID, 50)

	r.edgeSell(productID, 3)
	r.edgeSell(productID, 2)
	res := r.push()

	assert.Equal(t, 2, res.PushedOps, "one op per movement, no snapshots")
	assert.Equal(t, 45, r.cloudStockAmount(productID), "50 less the 5 sold")
	assert.Equal(t, int64(2), r.cloudCount("historic_stock", "product_id = ? AND kind = ?", productID, string(dto.StockMovementKindSale)),
		"both sales recorded in the cloud ledger")
	assert.Equal(t, 45, r.cloudLedgerSum(productID), "the ledger explains the amount")
}

// SYNC-INV-22 — the bug this change exists to fix: a purchase recorded in the office used to
// be erased by the restaurant's next snapshot. Now edge traffic folds on top of it.
func TestSync_Stock_CloudPurchaseSurvivesEdgeTraffic(t *testing.T) {
	r := newRig(t)
	productID := r.seedBothNodesProduct("SKU-FOLD-2", "Aguardiente")

	r.seedCloudStock(productID, 20)
	r.seedEdgeStock(productID, 20)

	// The restaurant sells against its own (soon to be stale) copy.
	r.edgeSell(productID, 4)

	// Meanwhile the office records a delivery.
	r.cloudRecordPurchase(productID, 100)
	require.Equal(t, 120, r.cloudStockAmount(productID))

	r.push()

	assert.Equal(t, 116, r.cloudStockAmount(productID), "20 + 100 - 4: both writers counted")
	assert.Equal(t, 116, r.cloudLedgerSum(productID))
	assert.Equal(t, int64(1), r.cloudCount("historic_stock", "product_id = ? AND kind = ? AND change = ?",
		productID, string(dto.StockMovementKindPurchase), 100),
		"the purchase is still on the ledger")
}

// SYNC-INV-25 — whatever the restaurant queues for a stock change, none of it carries an
// on-hand amount the cloud could assign.
func TestSync_Stock_EdgeNeverSendsAbsolute(t *testing.T) {
	r := newRig(t)
	productID := r.seedBothNodesProduct("SKU-DELTA-1", "Gaseosa")
	r.seedEdgeStock(productID, 60)

	r.edgeSell(productID, 7)

	queued := r.edgeQueuedOps()
	require.Len(t, queued, 1, "exactly one op per movement")
	assert.Equal(t, dto.SyncEntityHistoricStock, queued[0].EntityType, "no stock snapshot op is queued")

	var payload dto.HistoricStockSyncPayload
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	assert.Equal(t, -7, payload.Change, "the op carries the change")
	assert.Equal(t, dto.StockMovementKindSale, payload.Kind)

	// The payload has no field an applier could read as on-hand: prove it by shape.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(queued[0].Payload, &raw))
	assert.NotContains(t, raw, "amount", "no absolute on the wire")
	assert.Contains(t, raw, "change")
}

// SYNC-INV-26 — selling never depends on the cloud. With it unreachable the sales complete,
// the movements queue, and once it returns each one folds exactly once.
func TestSync_Stock_OfflineThenDrains(t *testing.T) {
	r := newRig(t)
	productID := r.seedBothNodesProduct("SKU-OFFLINE-1", "Ron")

	r.seedCloudStock(productID, 40)
	r.seedEdgeStock(productID, 40)

	r.takeCloudOffline() // the link drops mid-service

	r.edgeSell(productID, 6)
	r.edgeSell(productID, 4)

	assert.Equal(t, 30, r.edgeStockAmount(productID), "the restaurant keeps selling on its own numbers")
	assert.Equal(t, int64(2), r.edgeCount("sync_outbox", "synced_at IS NULL"), "both movements are queued")

	_, err := r.edgePush.PushPending(r.ctx)
	require.Error(t, err, "a push to an unreachable cloud fails")
	assert.Equal(t, int64(2), r.edgeCount("sync_outbox", "synced_at IS NULL"), "nothing is lost on a failed push")

	r.bringCloudOnline()

	r.push()
	r.push() // a second drain must not re-apply anything

	assert.Equal(t, 30, r.cloudStockAmount(productID), "40 less the 10 sold, applied once")
	assert.Equal(t, int64(0), r.edgeCount("sync_outbox", "synced_at IS NULL"), "the queue drained")
	assert.Equal(t, 30, r.cloudLedgerSum(productID))
}

// SYNC-INV-27 — the day opens with the cloud's numbers, including what the office recorded
// while the restaurant was closed.
func TestSync_Stock_DailyRefreshAdoptsCloudNumbers(t *testing.T) {
	r := newRig(t)
	productID := r.seedBothNodesProduct("SKU-REFRESH-1", "Vino")

	r.seedCloudStock(productID, 10)
	r.seedEdgeStock(productID, 10)

	// Overnight in the office: a delivery and a shelf count.
	r.cloudRecordPurchase(productID, 90)
	r.cloudCountStock(productID, 85)
	require.Equal(t, 85, r.cloudStockAmount(productID))

	res := r.refreshEdgeStock()

	assert.Equal(t, 1, res.Stock, "one stock row refreshed")
	assert.Equal(t, 85, r.edgeStockAmount(productID), "the restaurant opens on the cloud's number")
	assert.Equal(t, r.cloudStockAmount(productID), r.edgeStockAmount(productID))

	// A second refresh with nothing new changes nothing.
	assert.Equal(t, 0, r.refreshEdgeStock().Stock)
	assert.Equal(t, 85, r.edgeStockAmount(productID))
}

// SYNC-INV-24 — after a mixed day, every touched product's amount equals the sum of its
// movements. This is the invariant the reconciliation job checks in production.
func TestSync_Stock_LedgerSumsToAmount(t *testing.T) {
	r := newRig(t)
	beer := r.seedBothNodesProduct("SKU-MIX-1", "Cerveza")
	rum := r.seedBothNodesProduct("SKU-MIX-2", "Ron")

	r.seedCloudStock(beer, 100)
	r.seedCloudStock(rum, 50)
	r.seedEdgeStock(beer, 100)
	r.seedEdgeStock(rum, 50)

	// The restaurant sells.
	r.edgeSell(beer, 12)
	r.edgeSell(rum, 3)
	r.edgeSell(beer, 8)
	r.push()

	// The office records a delivery and corrects a count.
	r.cloudRecordPurchase(beer, 60)
	r.cloudCountStock(rum, 44)

	// More sales arrive after the count and must still take effect on top of it.
	r.edgeSell(rum, 2)
	r.push()

	for _, productID := range []string{beer, rum} {
		assert.Equal(t, r.cloudLedgerSum(productID), r.cloudStockAmount(productID),
			"amount == SUM(change) for %s", productID)
	}
	assert.Equal(t, 140, r.cloudStockAmount(beer), "100 - 20 + 60")
	assert.Equal(t, 42, r.cloudStockAmount(rum), "50 - 3 counted to 44, then - 2")
}
