package service

import (
	"context"
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const logKeyNodeID = "node_id"

const defaultPushBatchSize = 100

// SyncPushService is the edge side of replication. It drains this node's unsynced
// outbox rows to the cloud in seq order, stamping rows synced and advancing the
// per-peer high-water mark only for ops the cloud actually acked. It is safe to run
// on a timer: unacked rows are retried next tick, and the cloud dedups by op_id.
type SyncPushService struct {
	unitOfWork   ports.UnitOfWork
	outboxRepo   ports.SyncOutboxRepository
	stateRepo    ports.SyncStateRepository
	pushClient   ports.SyncPushClient
	syncIdentity dto.SyncIdentity
	batchSize    int
	logger       *slog.Logger
}

func NewSyncPushService(
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	stateRepo ports.SyncStateRepository,
	pushClient ports.SyncPushClient,
	syncIdentity dto.SyncIdentity,
	batchSize int,
	logger *slog.Logger,
) *SyncPushService {
	if batchSize <= 0 {
		batchSize = defaultPushBatchSize
	}
	return &SyncPushService{
		unitOfWork:   unitOfWork,
		outboxRepo:   outboxRepo,
		stateRepo:    stateRepo,
		pushClient:   pushClient,
		syncIdentity: syncIdentity,
		batchSize:    batchSize,
		logger:       logger,
	}
}

// PushPending drains the outbox to the cloud in batches. It stops when a batch is
// short (outbox drained) or the cloud acks fewer ops than were sent — the cloud
// applies in order and stops at the first failed op, so the rest is retried next
// tick. Returns how many ops were acked across how many batches.
func (s *SyncPushService) PushPending(ctx context.Context) (*dto.SyncPushResult, error) {
	result := &dto.SyncPushResult{}

	for {
		entries, err := s.outboxRepo.ListUnsynced(ctx, s.syncIdentity.NodeID, s.batchSize)
		if err != nil {
			return result, fmt.Errorf("list unsynced outbox: %w", err)
		}
		if len(entries) == 0 {
			// Detect stale node identity: rows exist for a different origin_node_id but
			// none for ours. This happens when APP_MODE or ORGANIZATION_ID changes between
			// runs (the derived NodeID shifts) or when NODE_ID is set to a new value.
			// Fix: set NODE_ID explicitly to the UUID shown in the sync_outbox table, or
			// UPDATE sync_outbox SET origin_node_id = '<current>' WHERE synced_at IS NULL.
			if orphaned, checkErr := s.outboxRepo.HasUnsyncedFromOtherOrigins(ctx, s.syncIdentity.NodeID); checkErr == nil && orphaned {
				s.logger.Warn("sync outbox has unsynced rows for a different node ID — "+
					"outbox rows will never be pushed until the mismatch is resolved; "+
					"set NODE_ID env var to the UUID stored in sync_outbox.origin_node_id",
					slog.String(logKeyNodeID, s.syncIdentity.NodeID))
			}
			return result, nil
		}

		ops := make([]dto.SyncOutboxEntry, len(entries))
		for i, e := range entries {
			ops[i] = *e
		}

		resp, err := s.pushClient.Push(ctx, &dto.SyncPushRequest{NodeID: s.syncIdentity.NodeID, Ops: ops})
		if err != nil {
			return result, fmt.Errorf("push to cloud: %w", err)
		}

		if len(resp.AckedOpIDs) > 0 {
			if err := s.commitAcks(ctx, resp); err != nil {
				return result, err
			}
			result.Batches++
			result.PushedOps += len(resp.AckedOpIDs)
		}

		// Partial ack: the cloud stopped at a failed op. Stop and retry next tick.
		if len(resp.AckedOpIDs) < len(ops) {
			return result, nil
		}
		// Full batch acked but shorter than the cap means the outbox is drained.
		if len(entries) < s.batchSize {
			return result, nil
		}
	}
}

// commitAcks stamps the acked rows synced and advances the cloud peer's high-water
// mark in one transaction, so progress is recorded atomically.
func (s *SyncPushService) commitAcks(ctx context.Context, resp *dto.SyncPushResponse) error {
	return s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.outboxRepo.MarkSynced(ctx, resp.AckedOpIDs); err != nil {
			return fmt.Errorf("mark outbox synced: %w", err)
		}

		maxSeq := int64(0)
		for _, seq := range resp.AckedSeqs {
			if seq > maxSeq {
				maxSeq = seq
			}
		}
		if maxSeq > 0 {
			if err := s.stateRepo.AdvancePushedSeq(ctx, s.syncIdentity.CloudNodeID, maxSeq); err != nil {
				return fmt.Errorf("advance pushed seq: %w", err)
			}
		}
		return nil
	})
}
