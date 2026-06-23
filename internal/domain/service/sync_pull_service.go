package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// SyncPullService is the edge side of replication's pull half: it fetches the cloud's
// reference changes since the local cursor, upserts them, and advances the cursor — all
// in one transaction so the data and the bookmark move together. Re-running is safe:
// the upserts are idempotent and the cursor only moves forward.
type SyncPullService struct {
	unitOfWork   ports.UnitOfWork
	pullClient   ports.SyncPullClient
	writer       ports.SyncReferenceWriter
	stateRepo    ports.SyncStateRepository
	syncIdentity dto.SyncIdentity
	logger       *slog.Logger
}

func NewSyncPullService(
	unitOfWork ports.UnitOfWork,
	pullClient ports.SyncPullClient,
	writer ports.SyncReferenceWriter,
	stateRepo ports.SyncStateRepository,
	syncIdentity dto.SyncIdentity,
	logger *slog.Logger,
) *SyncPullService {
	return &SyncPullService{
		unitOfWork:   unitOfWork,
		pullClient:   pullClient,
		writer:       writer,
		stateRepo:    stateRepo,
		syncIdentity: syncIdentity,
		logger:       logger,
	}
}

// PullChanges fetches reference changes newer than the stored cursor and applies them.
// With no stored cursor (first pull) it pulls from the beginning of time (zero value).
func (s *SyncPullService) PullChanges(ctx context.Context) (*dto.SyncPullResult, error) {
	cursorPtr, err := s.stateRepo.GetPulledCursor(ctx, s.syncIdentity.CloudNodeID)
	if err != nil {
		return nil, fmt.Errorf("read pulled cursor: %w", err)
	}
	var since time.Time
	if cursorPtr != nil {
		since = *cursorPtr
	}

	resp, err := s.pullClient.Pull(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("pull from cloud: %w", err)
	}

	if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.writer.UpsertProducts(ctx, resp.Products); err != nil {
			return fmt.Errorf("upsert products: %w", err)
		}
		if err := s.writer.UpsertUsers(ctx, resp.Users); err != nil {
			return fmt.Errorf("upsert users: %w", err)
		}
		if err := s.writer.UpsertSuppliers(ctx, resp.Suppliers); err != nil {
			return fmt.Errorf("upsert suppliers: %w", err)
		}
		// Only move the cursor forward; an unchanged response leaves it where it was.
		if resp.Cursor.After(since) {
			if err := s.stateRepo.AdvancePulledCursor(ctx, s.syncIdentity.CloudNodeID, resp.Cursor); err != nil {
				return fmt.Errorf("advance pulled cursor: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &dto.SyncPullResult{
		Products:  len(resp.Products),
		Users:     len(resp.Users),
		Suppliers: len(resp.Suppliers),
	}, nil
}
