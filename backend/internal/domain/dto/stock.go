package dto

import "time"

type Stock struct {
	ProductID     string        `json:"product_id"`
	Version       int           `json:"version"`
	Amount        int           `json:"amount"`
	UnitOfMeasure UnitOfMeasure `json:"unit_of_measure"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type StockListResponse struct {
	Stocks []*Stock `json:"stocks"`
	Total  *int     `json:"total,omitempty"`
}

type CreateStockRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Amount    int    `json:"amount" validate:"required"`
}

type AddOrDecreaseStockRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Change    int    `json:"change" validate:"required"`
}

type BulkStockItem struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Amount    int    `json:"amount" validate:"required,min=0"`
}

type BulkStockCreationOrUpdatingRequest struct {
	Items []BulkStockItem `json:"items" validate:"required,dive"`
}

// StockMovementKind names the event a ledger row records. On-hand is the sum of its
// movements, so the kind is what lets the history explain a number: a sale reads
// differently from a correction or the opening balance written when the cloud adopted
// ownership. A row whose origin predates the field — or a peer that has not shipped it —
// is StockMovementKindUnknown.
type StockMovementKind string

const (
	StockMovementKindSale           StockMovementKind = "sale"
	StockMovementKindPurchase       StockMovementKind = "purchase"
	StockMovementKindAdjustment     StockMovementKind = "adjustment"
	StockMovementKindCount          StockMovementKind = "count"
	StockMovementKindOpeningBalance StockMovementKind = "opening_balance"
	StockMovementKindUnknown        StockMovementKind = "unknown"
)

// NormalizeStockMovementKind maps an empty or unrecognized kind to unknown, so a peer that
// sends no kind (or one this node does not know) never violates the column's CHECK.
func NormalizeStockMovementKind(kind StockMovementKind) StockMovementKind {
	switch kind {
	case StockMovementKindSale,
		StockMovementKindPurchase,
		StockMovementKindAdjustment,
		StockMovementKindCount,
		StockMovementKindOpeningBalance,
		StockMovementKindUnknown:
		return kind
	default:
		return StockMovementKindUnknown
	}
}

type HistoricStock struct {
	ID int `json:"id"`
	// OpID is the ledger row's cross-node identity, generated when the row is created and
	// reused as the sync op id so the movement replicates edge → cloud. Empty for legacy
	// rows written before sync was enabled.
	OpID          string            `json:"op_id,omitempty"`
	ProductID     string            `json:"product_id"`
	UnitOfMeasure UnitOfMeasure     `json:"unit_of_measure"`
	CreatedAt     time.Time         `json:"created_at"`
	Change        int               `json:"change"`
	Kind          StockMovementKind `json:"kind"`
}

// StockLedgerTotal pairs a product's stored on-hand with the sum of the movements recorded
// for it (zero when it has none). It is the raw input to reconciliation, not a report.
type StockLedgerTotal struct {
	ProductID     string
	Amount        int
	LedgerSum     int
	UnitOfMeasure UnitOfMeasure
}

// StockDivergence is one product whose on-hand disagrees with its movement history. Both
// numbers are reported, because either one can be the wrong one and only a person can say.
type StockDivergence struct {
	ProductID string `json:"product_id"`
	Amount    int    `json:"amount"`
	LedgerSum int    `json:"ledger_sum"`
	Delta     int    `json:"delta"`
}

// StockReconciliationReport is one run of the invariant check: how many products were
// compared and which ones disagreed. Report-only — nothing is corrected.
type StockReconciliationReport struct {
	CheckedProducts int               `json:"checked_products"`
	Diverged        []StockDivergence `json:"diverged"`
}

// StockSyncFreshness says how recently the cloud last folded a movement replicated from the
// restaurant. LastMovementAppliedAt is nil when the restaurant has never pushed one, which a
// counting screen must treat as "unknown", not "fresh".
type StockSyncFreshness struct {
	LastMovementAppliedAt *time.Time `json:"last_movement_applied_at"`
	StalenessSeconds      *int       `json:"staleness_seconds"`
}

// StockOpeningBalanceReport is one run of the cutover baseline: how many products were
// examined and how many needed a reconciling movement. A second run reports zero written,
// because the first run is what made the sums agree.
type StockOpeningBalanceReport struct {
	CheckedProducts int `json:"checked_products"`
	WrittenBalances int `json:"written_balances"`
}
