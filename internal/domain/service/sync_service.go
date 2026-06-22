package service

import (
	"context"
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// SyncService applies a batch of ops pushed by a peer node. Each op is processed in
// its own transaction so one bad op doesn't undo earlier ones; the sender retries
// whatever was not acked. Apply is idempotent via the inbox, so retries are safe.
type SyncService struct {
	unitOfWork ports.UnitOfWork
	inboxRepo  ports.SyncInboxRepository
	appliers   map[dto.SyncEntityType]ports.SyncApplier
	logger     *slog.Logger
}

func NewSyncService(
	unitOfWork ports.UnitOfWork,
	inboxRepo ports.SyncInboxRepository,
	appliers map[dto.SyncEntityType]ports.SyncApplier,
	logger *slog.Logger,
) *SyncService {
	return &SyncService{
		unitOfWork: unitOfWork,
		inboxRepo:  inboxRepo,
		appliers:   appliers,
		logger:     logger,
	}
}

// ApplyPush applies each op idempotently and returns the ops that were acked. It
// stops at the first op that fails to apply, acking everything before it; the
// sender retries from the first un-acked op.
func (s *SyncService) ApplyPush(ctx context.Context, req *dto.SyncPushRequest) (*dto.SyncPushResponse, error) {
	resp := &dto.SyncPushResponse{}

	for i := range req.Ops {
		op := req.Ops[i]

		if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
			alreadyApplied, err := s.inboxRepo.MarkApplied(ctx, op.OpID)
			if err != nil {
				return fmt.Errorf("record op in inbox: %w", err)
			}
			if alreadyApplied {
				return nil // idempotent: applied on an earlier push, just ack
			}

			applier, ok := s.appliers[op.EntityType]
			if !ok {
				return fmt.Errorf("no applier registered for entity type %q", op.EntityType)
			}
			return applier.Apply(ctx, &op)
		}); err != nil {
			return resp, fmt.Errorf("apply op %s: %w", op.OpID, err)
		}

		resp.AckedOpIDs = append(resp.AckedOpIDs, op.OpID)
		resp.AckedSeqs = append(resp.AckedSeqs, op.Seq)
	}

	return resp, nil
}
