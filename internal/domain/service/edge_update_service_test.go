package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const edgeUpdateUserID = "019a0000-0000-7000-8000-0000000000bb"

func newEdgeUpdateService(t *testing.T) (*EdgeUpdateService, *mocks.MockEdgeUpdateStateStore) {
	store := mocks.NewMockEdgeUpdateStateStore(t)
	svc := NewEdgeUpdateService(store)
	svc.now = func() time.Time { return time.Date(2026, 10, 19, 15, 0, 0, 0, time.FixedZone("COT", -5*3600)) }
	return svc, store
}

func stagedState(version string) *dto.EdgeUpdateState {
	return &dto.EdgeUpdateState{Available: &dto.EdgeUpdateAvailable{Version: version}}
}

func TestEdgeUpdateService_RequestUpdate_WritesRequest(t *testing.T) {
	svc, store := newEdgeUpdateService(t)
	store.EXPECT().Read(mock.Anything).Return(stagedState("v1.0.1"))
	store.EXPECT().UpdatePending(mock.Anything).Return(false, nil)
	want := &dto.EdgeUpdateRequest{
		Version:     "v1.0.1",
		RequestedBy: edgeUpdateUserID,
		RequestedAt: "2026-10-19T15:00:00-05:00",
	}
	store.EXPECT().WriteRequest(mock.Anything, want).Return(nil)

	req, err := svc.RequestUpdate(context.Background(), "v1.0.1", edgeUpdateUserID)

	require.NoError(t, err)
	assert.Equal(t, want, req)
}

func TestEdgeUpdateService_RequestUpdate_Refusals(t *testing.T) {
	tests := []struct {
		name    string
		state   *dto.EdgeUpdateState
		pending bool
		wantErr error
	}{
		{"nothing staged", &dto.EdgeUpdateState{}, false, domainError.ErrNoStagedUpdate},
		{"version differs", stagedState("v1.0.2"), false, domainError.ErrUpdateVersionMismatch},
		{"already pending", stagedState("v1.0.1"), true, domainError.ErrUpdateAlreadyPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, store := newEdgeUpdateService(t)
			store.EXPECT().Read(mock.Anything).Return(tt.state)
			store.EXPECT().UpdatePending(mock.Anything).Return(tt.pending, nil).Maybe()

			_, err := svc.RequestUpdate(context.Background(), "v1.0.1", edgeUpdateUserID)

			assert.ErrorIs(t, err, tt.wantErr)
			store.AssertNotCalled(t, "WriteRequest", mock.Anything, mock.Anything)
		})
	}
}

func TestEdgeUpdateService_RequestUpdate_WriteError(t *testing.T) {
	svc, store := newEdgeUpdateService(t)
	store.EXPECT().Read(mock.Anything).Return(stagedState("v1.0.1"))
	store.EXPECT().UpdatePending(mock.Anything).Return(false, nil)
	store.EXPECT().WriteRequest(mock.Anything, mock.Anything).Return(errors.New("disk full"))

	_, err := svc.RequestUpdate(context.Background(), "v1.0.1", edgeUpdateUserID)

	assert.ErrorContains(t, err, "disk full")
}

func TestEdgeUpdateService_RequestUpdate_PendingCheckError(t *testing.T) {
	svc, store := newEdgeUpdateService(t)
	store.EXPECT().Read(mock.Anything).Return(stagedState("v1.0.1"))
	store.EXPECT().UpdatePending(mock.Anything).Return(false, errors.New("access denied"))

	_, err := svc.RequestUpdate(context.Background(), "v1.0.1", edgeUpdateUserID)

	assert.ErrorContains(t, err, "access denied")
}

func TestEdgeUpdateService_GetState_PassesStoreState(t *testing.T) {
	svc, store := newEdgeUpdateService(t)
	state := stagedState("v1.0.1")
	store.EXPECT().Read(mock.Anything).Return(state)

	assert.Same(t, state, svc.GetState(context.Background()))
}
