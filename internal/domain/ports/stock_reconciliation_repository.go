package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// StockReconciliationRepository reads the two numbers the cloud's on-hand invariant relates:
// the stored amount and the sum of the movements behind it. It only reads — correcting a
// divergence is a decision, not a job.
type StockReconciliationRepository interface {
	// FindLedgerTotals returns one row per live stock row: its on-hand amount alongside the
	// sum of that product's recorded movements (zero when it has none).
	FindLedgerTotals(ctx context.Context) ([]dto.StockLedgerTotal, error)
	// LastEdgeMovementAppliedAt returns when the cloud last applied a movement replicated
	// from a peer, or nil if it never has.
	LastEdgeMovementAppliedAt(ctx context.Context) (*time.Time, error)
}
