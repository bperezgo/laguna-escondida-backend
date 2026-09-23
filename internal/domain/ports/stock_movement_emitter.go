package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// StockMovementEmitter queues a stock movement for replication to the node that owns on-hand.
// It exists so the services that write stock don't have to know which node they run on: the
// restaurant emits (its movements are what the cloud folds), the cloud does not (nothing
// consumes a cloud-origin stock op, so emitting would just accumulate undelivered rows).
//
// Emit is called inside the same transaction as the ledger write, so the movement and its
// replication commit or roll back together.
type StockMovementEmitter interface {
	Emit(ctx context.Context, movement *dto.HistoricStock) error
}
