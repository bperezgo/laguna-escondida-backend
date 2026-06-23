package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// EdgeStatusService computes the data-backed half of the edge node's status: how many of
// this node's local changes are still queued for the cloud (the unsynced outbox) and how
// stale the oldest of them is. Connectivity (online / last successful sync) is a runtime
// concern tracked in the platform layer, not here — this service only reads the outbox,
// so it stays a pure domain use case.
type EdgeStatusService struct {
	outboxRepo   ports.SyncOutboxRepository
	syncIdentity dto.SyncIdentity
}

func NewEdgeStatusService(outboxRepo ports.SyncOutboxRepository, syncIdentity dto.SyncIdentity) *EdgeStatusService {
	return &EdgeStatusService{
		outboxRepo:   outboxRepo,
		syncIdentity: syncIdentity,
	}
}

// GetSyncHealth reports this node's pending (unsynced) outbox count and the sync lag in
// seconds — now minus the oldest unsynced row's created_at — which is 0 when nothing is
// pending. The lag answers "how far behind is the cloud from this node's local truth".
func (s *EdgeStatusService) GetSyncHealth(ctx context.Context) (*dto.EdgeSyncHealth, error) {
	stats, err := s.outboxRepo.PendingStats(ctx, s.syncIdentity.NodeID)
	if err != nil {
		return nil, fmt.Errorf("read outbox pending stats: %w", err)
	}

	lagSeconds := 0
	if stats.OldestPendingAt != nil {
		if lag := time.Since(*stats.OldestPendingAt); lag > 0 {
			lagSeconds = int(lag.Seconds())
		}
	}

	return &dto.EdgeSyncHealth{
		PendingOps:     stats.PendingCount,
		SyncLagSeconds: lagSeconds,
	}, nil
}
