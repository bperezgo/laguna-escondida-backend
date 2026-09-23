package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// StockPullService is the restaurant's daily stock refresh: it adopts the cloud's on-hand
// wholesale, because the cloud owns that number and the restaurant only displays it. Each
// day therefore opens with whatever the office recorded — purchases, corrections, counts —
// and drifts from it during service as the restaurant sells.
//
// It touches only the amounts. Movements already queued for the cloud are untouched and
// still push: what the restaurant has sold is its own to report, and the cloud folds it on
// top of the number this refresh just wrote.
type StockPullService struct {
	unitOfWork   ports.UnitOfWork
	pullClient   ports.SyncStockPullClient
	writer       ports.SyncReferenceWriter
	stateRepo    ports.SyncStateRepository
	syncIdentity dto.SyncIdentity
	logger       *slog.Logger
}

func NewStockPullService(
	unitOfWork ports.UnitOfWork,
	pullClient ports.SyncStockPullClient,
	writer ports.SyncReferenceWriter,
	stateRepo ports.SyncStateRepository,
	syncIdentity dto.SyncIdentity,
	logger *slog.Logger,
) *StockPullService {
	return &StockPullService{
		unitOfWork:   unitOfWork,
		pullClient:   pullClient,
		writer:       writer,
		stateRepo:    stateRepo,
		syncIdentity: syncIdentity,
		logger:       logger,
	}
}

// RefreshStock replaces local amounts with the cloud's and advances the stock cursor, both
// in one transaction so the data and its bookmark move together. With no stored cursor the
// first refresh takes the cloud's numbers whole. An unreachable cloud leaves the previous
// amounts and the cursor exactly as they were.
func (s *StockPullService) RefreshStock(ctx context.Context) (*dto.SyncStockPullResult, error) {
	cursorPtr, err := s.stateRepo.GetStockPulledCursor(ctx, s.syncIdentity.CloudNodeID)
	if err != nil {
		return nil, fmt.Errorf("read stock pulled cursor: %w", err)
	}
	var since time.Time
	if cursorPtr != nil {
		since = *cursorPtr
	}

	resp, err := s.pullClient.PullStock(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("pull stock from cloud: %w", err)
	}

	if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.writer.ReplaceStockAmounts(ctx, resp.Stock); err != nil {
			return fmt.Errorf("replace stock amounts: %w", err)
		}
		// Only move the cursor forward; an unchanged response leaves it where it was.
		if resp.Cursor.After(since) {
			if err := s.stateRepo.AdvanceStockPulledCursor(ctx, s.syncIdentity.CloudNodeID, resp.Cursor); err != nil {
				return fmt.Errorf("advance stock pulled cursor: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &dto.SyncStockPullResult{Stock: len(resp.Stock)}, nil
}
