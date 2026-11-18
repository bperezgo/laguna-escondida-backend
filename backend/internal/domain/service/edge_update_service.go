package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

// EdgeUpdateService lets an admin ask the appliance updater to apply the staged release.
type EdgeUpdateService struct {
	store ports.EdgeUpdateStateStore
	now   func() time.Time
	// Serializes the pending check and the write so two clicks cannot both get a 202.
	mu sync.Mutex
}

func NewEdgeUpdateService(store ports.EdgeUpdateStateStore) *EdgeUpdateService {
	return &EdgeUpdateService{store: store, now: time.Now}
}

func (s *EdgeUpdateService) GetState(ctx context.Context) *dto.EdgeUpdateState {
	return s.store.Read(ctx)
}

func (s *EdgeUpdateService) RequestUpdate(ctx context.Context, version, userID string) (*dto.EdgeUpdateRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.store.Read(ctx)
	if state.Available == nil {
		return nil, domainError.ErrNoStagedUpdate
	}
	if state.Available.Version != version {
		return nil, domainError.ErrUpdateVersionMismatch
	}

	pending, err := s.store.UpdatePending(ctx)
	if err != nil {
		return nil, fmt.Errorf("check pending update: %w", err)
	}
	if pending {
		return nil, domainError.ErrUpdateAlreadyPending
	}

	req := &dto.EdgeUpdateRequest{
		Version:     version,
		RequestedBy: userID,
		RequestedAt: s.now().Format(time.RFC3339),
	}
	if err := s.store.WriteRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("write update request: %w", err)
	}
	return req, nil
}
