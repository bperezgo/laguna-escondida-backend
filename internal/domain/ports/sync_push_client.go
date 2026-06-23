package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncPushClient sends a batch of this node's outbox ops to a peer's sync endpoint
// and returns what the peer acknowledged. The edge implementation POSTs to the
// cloud's /api/sync/push; the domain depends only on this contract.
type SyncPushClient interface {
	Push(ctx context.Context, req *dto.SyncPushRequest) (*dto.SyncPushResponse, error)
}
