package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncStockPullClient fetches the cloud's stock rows changed since a cursor. It is a
// channel of its own rather than part of SyncPullClient because stock refreshes daily
// while reference data refreshes every minute (design D4).
type SyncStockPullClient interface {
	PullStock(ctx context.Context, since time.Time) (*dto.SyncStockPullResponse, error)
}
