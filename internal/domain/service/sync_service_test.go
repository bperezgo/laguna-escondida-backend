package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testOpenBillOp(opID, entityID string, seq int64) dto.SyncOutboxEntry {
	return dto.SyncOutboxEntry{
		OpID:         opID,
		OriginNodeID: testNodeID,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     entityID,
		Operation:    dto.SyncOperationCreate,
		Payload:      json.RawMessage(`{"id":"` + entityID + `"}`),
		Seq:          seq,
	}
}

// TestSyncService_ApplyPush_Idempotent is the headline Step 7 guarantee: pushing the
// same batch twice applies it once (inbox dedup) but acks it both times.
func TestSyncService_ApplyPush_Idempotent(t *testing.T) {
	ctx := context.Background()
	mockUoW := createMockUnitOfWork(t)
	mockInbox := mocks.NewMockSyncInboxRepository(t)
	mockApplier := mocks.NewMockSyncApplier(t)

	appliers := map[dto.SyncEntityType]ports.SyncApplier{dto.SyncEntityOpenBill: mockApplier}
	svc := NewSyncService(mockUoW, mockInbox, appliers, slog.Default())

	req := &dto.SyncPushRequest{NodeID: testNodeID, Ops: []dto.SyncOutboxEntry{testOpenBillOp("op-1", "bill-1", 7)}}

	// First push: op is new → applier runs. Second push: already applied → skipped.
	mockInbox.EXPECT().MarkApplied(mock.Anything, "op-1").Return(false, nil).Once()
	mockInbox.EXPECT().MarkApplied(mock.Anything, "op-1").Return(true, nil).Once()
	mockApplier.EXPECT().Apply(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).Return(nil).Once()

	resp1, err := svc.ApplyPush(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, []string{"op-1"}, resp1.AckedOpIDs)
	assert.Equal(t, []int64{7}, resp1.AckedSeqs)

	resp2, err := svc.ApplyPush(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, []string{"op-1"}, resp2.AckedOpIDs)
	assert.Equal(t, []int64{7}, resp2.AckedSeqs)

	mockApplier.AssertNumberOfCalls(t, "Apply", 1)
}

func TestSyncService_ApplyPush_NoApplierForEntity(t *testing.T) {
	ctx := context.Background()
	mockUoW := createMockUnitOfWork(t)
	mockInbox := mocks.NewMockSyncInboxRepository(t)
	svc := NewSyncService(mockUoW, mockInbox, map[dto.SyncEntityType]ports.SyncApplier{}, slog.Default())

	op := dto.SyncOutboxEntry{OpID: "op-2", EntityType: dto.SyncEntityType("unknown"), Operation: dto.SyncOperationCreate, Payload: json.RawMessage(`{}`)}
	mockInbox.EXPECT().MarkApplied(mock.Anything, "op-2").Return(false, nil).Once()

	resp, err := svc.ApplyPush(ctx, &dto.SyncPushRequest{Ops: []dto.SyncOutboxEntry{op}})

	require.Error(t, err)
	assert.Empty(t, resp.AckedOpIDs)
	assert.Contains(t, err.Error(), "no applier")
}

func TestSyncService_ApplyPush_StopsOnApplyFailure(t *testing.T) {
	ctx := context.Background()
	mockUoW := createMockUnitOfWork(t)
	mockInbox := mocks.NewMockSyncInboxRepository(t)
	mockApplier := mocks.NewMockSyncApplier(t)

	appliers := map[dto.SyncEntityType]ports.SyncApplier{dto.SyncEntityOpenBill: mockApplier}
	svc := NewSyncService(mockUoW, mockInbox, appliers, slog.Default())

	req := &dto.SyncPushRequest{Ops: []dto.SyncOutboxEntry{
		testOpenBillOp("op-1", "bill-1", 1),
		testOpenBillOp("op-2", "bill-2", 2),
	}}

	mockInbox.EXPECT().MarkApplied(mock.Anything, "op-1").Return(false, nil).Once()
	mockInbox.EXPECT().MarkApplied(mock.Anything, "op-2").Return(false, nil).Once()
	mockApplier.EXPECT().Apply(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).Return(nil).Once()
	mockApplier.EXPECT().Apply(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).Return(errors.New("boom")).Once()

	resp, err := svc.ApplyPush(ctx, req)

	require.Error(t, err)
	assert.Equal(t, []string{"op-1"}, resp.AckedOpIDs, "only the op before the failure is acked")
	assert.Equal(t, []int64{1}, resp.AckedSeqs)
}

func TestSyncService_ApplyPush_EmptyBatch(t *testing.T) {
	ctx := context.Background()
	mockUoW := createMockUnitOfWork(t)
	mockInbox := mocks.NewMockSyncInboxRepository(t)
	svc := NewSyncService(mockUoW, mockInbox, map[dto.SyncEntityType]ports.SyncApplier{}, slog.Default())

	resp, err := svc.ApplyPush(ctx, &dto.SyncPushRequest{Ops: nil})

	require.NoError(t, err)
	assert.Empty(t, resp.AckedOpIDs)
	assert.Empty(t, resp.AckedSeqs)
}
