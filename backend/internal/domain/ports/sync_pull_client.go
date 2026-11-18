package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncPullClient fetches reference changes from a peer's pull endpoint. The edge
// implementation issues GET /api/sync/pull?since=<cursor>; the domain depends only on
// this contract.
type SyncPullClient interface {
	Pull(ctx context.Context, since time.Time) (*dto.SyncPullResponse, error)
}
