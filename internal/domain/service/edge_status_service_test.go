package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const edgeStatusNodeID = "22222222-2222-2222-2222-222222222222"

func newEdgeStatusService(t *testing.T) (*EdgeStatusService, *mocks.MockSyncOutboxRepository) {
	outbox := mocks.NewMockSyncOutboxRepository(t)
	svc := NewEdgeStatusService(outbox, dto.SyncIdentity{NodeID: edgeStatusNodeID})
	return svc, outbox
}

// Success: fully drained outbox reports nothing pending and zero lag.
func TestEdgeStatusService_GetSyncHealth_NoPending(t *testing.T) {
	svc, outbox := newEdgeStatusService(t)
	outbox.EXPECT().
		PendingStats(context.Background(), edgeStatusNodeID).
		Return(&dto.SyncOutboxPendingStats{PendingCount: 0, OldestPendingAt: nil}, nil)

	health, err := svc.GetSyncHealth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, health.PendingOps)
	assert.Equal(t, 0, health.SyncLagSeconds)
}

// Calculation: lag is now minus the oldest unsynced row's created_at.
func TestEdgeStatusService_GetSyncHealth_WithBacklog(t *testing.T) {
	svc, outbox := newEdgeStatusService(t)
	oldest := time.Now().Add(-90 * time.Second)
	outbox.EXPECT().
		PendingStats(context.Background(), edgeStatusNodeID).
		Return(&dto.SyncOutboxPendingStats{PendingCount: 3, OldestPendingAt: &oldest}, nil)

	health, err := svc.GetSyncHealth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, health.PendingOps)
	assert.InDelta(t, 90, health.SyncLagSeconds, 2)
}

// Edge case: a future oldest timestamp (clock skew) clamps lag to 0, never negative.
func TestEdgeStatusService_GetSyncHealth_FutureTimestampClampsToZero(t *testing.T) {
	svc, outbox := newEdgeStatusService(t)
	future := time.Now().Add(30 * time.Second)
	outbox.EXPECT().
		PendingStats(context.Background(), edgeStatusNodeID).
		Return(&dto.SyncOutboxPendingStats{PendingCount: 1, OldestPendingAt: &future}, nil)

	health, err := svc.GetSyncHealth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, health.PendingOps)
	assert.Equal(t, 0, health.SyncLagSeconds)
}

// Error: repository failure propagates and returns no health.
func TestEdgeStatusService_GetSyncHealth_RepoError(t *testing.T) {
	svc, outbox := newEdgeStatusService(t)
	outbox.EXPECT().
		PendingStats(context.Background(), edgeStatusNodeID).
		Return(nil, errors.New("db down"))

	health, err := svc.GetSyncHealth(context.Background())

	require.Error(t, err)
	assert.Nil(t, health)
}
