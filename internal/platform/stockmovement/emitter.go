// Package stockmovement holds the two ways a node can replicate a stock movement: the
// restaurant queues it for the cloud, the cloud keeps it to itself.
package stockmovement

import (
	"context"
	"encoding/json"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// outboxEmitter is the restaurant's emitter: it appends the movement as an append-only
// historic_stock create op, reusing the ledger row's op_id as the sync op id (1:1) so the
// cloud dedupes on it. Only the signed change travels — never an on-hand amount.
type outboxEmitter struct {
	outboxRepo ports.SyncOutboxRepository
	nodeID     string
}

func NewOutboxEmitter(outboxRepo ports.SyncOutboxRepository, nodeID string) ports.StockMovementEmitter {
	return &outboxEmitter{outboxRepo: outboxRepo, nodeID: nodeID}
}

func (e *outboxEmitter) Emit(ctx context.Context, movement *dto.HistoricStock) error {
	payload := dto.HistoricStockSyncPayload{
		OpID:          movement.OpID,
		ProductID:     movement.ProductID,
		UnitOfMeasure: string(movement.UnitOfMeasure),
		Change:        movement.Change,
		Kind:          dto.NormalizeStockMovementKind(movement.Kind),
		CreatedAt:     movement.CreatedAt,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal historic_stock sync payload: %w", err)
	}

	return e.outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         movement.OpID,
		OriginNodeID: e.nodeID,
		EntityType:   dto.SyncEntityHistoricStock,
		EntityID:     movement.OpID,
		Operation:    dto.SyncOperationCreate,
		Payload:      payloadBytes,
	})
}

// noopEmitter is the cloud's emitter. The cloud already holds the movement it just wrote and
// the restaurant learns on-hand through the daily pull, so a queued op would never be
// delivered — it would accumulate exactly like the stranded cloud-origin bill ops do today.
type noopEmitter struct{}

func NewNoopEmitter() ports.StockMovementEmitter { return &noopEmitter{} }

func (e *noopEmitter) Emit(context.Context, *dto.HistoricStock) error { return nil }
