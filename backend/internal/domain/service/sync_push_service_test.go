package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testCloudNodeID = "00000000-0000-0000-0000-0000000000cc"

func pushOutboxEntry(opID string, seq int64) *dto.SyncOutboxEntry {
	return &dto.SyncOutboxEntry{
		OpID:         opID,
		OriginNodeID: testNodeID,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     "entity-" + opID,
		Operation:    dto.SyncOperationCreate,
		Payload:      json.RawMessage(`{}`),
		Seq:          seq,
	}
}

func newPushService(t *testing.T, outbox *mocks.MockSyncOutboxRepository, state *mocks.MockSyncStateRepository, client *mocks.MockSyncPushClient, batchSize int) *SyncPushService {
	return NewSyncPushService(
		createMockUnitOfWork(t), outbox, state, client,
		dto.SyncIdentity{NodeID: testNodeID, CloudNodeID: testCloudNodeID}, batchSize, slog.Default(),
	)
}

// TestPushPending_DrainsAndMarksSynced is the headline: a short batch is pushed, all
// ops are acked, and the acked op_ids are stamped synced while the peer's high-water
// mark advances to the max acked seq — in one pass.
func TestPushPending_DrainsAndMarksSynced(t *testing.T) {
	ctx := context.Background()
	outbox := mocks.NewMockSyncOutboxRepository(t)
	state := mocks.NewMockSyncStateRepository(t)
	client := mocks.NewMockSyncPushClient(t)

	entries := []*dto.SyncOutboxEntry{pushOutboxEntry("op-1", 1), pushOutboxEntry("op-2", 2)}
	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 100).Return(entries, nil).Once()

	var sentReq *dto.SyncPushRequest
	client.EXPECT().Push(mock.Anything, mock.AnythingOfType("*dto.SyncPushRequest")).
		Run(func(_ context.Context, req *dto.SyncPushRequest) { sentReq = req }).
		Return(&dto.SyncPushResponse{AckedOpIDs: []string{"op-1", "op-2"}, AckedSeqs: []int64{1, 2}}, nil).
		Once()

	outbox.EXPECT().MarkSynced(mock.Anything, []string{"op-1", "op-2"}).Return(nil).Once()
	state.EXPECT().AdvancePushedSeq(mock.Anything, testCloudNodeID, int64(2)).Return(nil).Once()

	result, err := newPushService(t, outbox, state, client, 100).PushPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Batches)
	assert.Equal(t, 2, result.PushedOps)

	require.NotNil(t, sentReq)
	assert.Equal(t, testNodeID, sentReq.NodeID, "push request carries this node's id")
	assert.Len(t, sentReq.Ops, 2)
}

func TestPushPending_EmptyOutbox_NoOp(t *testing.T) {
	ctx := context.Background()
	outbox := mocks.NewMockSyncOutboxRepository(t)
	state := mocks.NewMockSyncStateRepository(t)
	client := mocks.NewMockSyncPushClient(t)

	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 100).Return([]*dto.SyncOutboxEntry{}, nil).Once()
	// Genuinely empty outbox: no orphaned rows under a different node ID.
	outbox.EXPECT().HasUnsyncedFromOtherOrigins(mock.Anything, testNodeID).Return(false, nil).Once()

	result, err := newPushService(t, outbox, state, client, 100).PushPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, result.PushedOps)
	// No push, no mark, no state advance when there is nothing to send.
	client.AssertNotCalled(t, "Push", mock.Anything, mock.Anything)
}

func TestPushPending_EmptyOutbox_OrphanedRows_LogsWarning(t *testing.T) {
	ctx := context.Background()
	outbox := mocks.NewMockSyncOutboxRepository(t)
	state := mocks.NewMockSyncStateRepository(t)
	client := mocks.NewMockSyncPushClient(t)

	// My rows return empty, but orphaned rows exist under a different node ID.
	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 100).Return([]*dto.SyncOutboxEntry{}, nil).Once()
	outbox.EXPECT().HasUnsyncedFromOtherOrigins(mock.Anything, testNodeID).Return(true, nil).Once()

	result, err := newPushService(t, outbox, state, client, 100).PushPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, result.PushedOps)
	// Still no push — but the warning log would have fired.
	client.AssertNotCalled(t, "Push", mock.Anything, mock.Anything)
}

// TestPushPending_PartialAck_StopsAfterCommitting verifies the cloud stopping at a bad
// op (acking only a prefix) marks just the acked ops synced and does not loop again —
// the unacked tail is left for the next tick.
func TestPushPending_PartialAck_StopsAfterCommitting(t *testing.T) {
	ctx := context.Background()
	outbox := mocks.NewMockSyncOutboxRepository(t)
	state := mocks.NewMockSyncStateRepository(t)
	client := mocks.NewMockSyncPushClient(t)

	entries := []*dto.SyncOutboxEntry{pushOutboxEntry("op-1", 1), pushOutboxEntry("op-2", 2)}
	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 100).Return(entries, nil).Once()

	client.EXPECT().Push(mock.Anything, mock.Anything).
		Return(&dto.SyncPushResponse{AckedOpIDs: []string{"op-1"}, AckedSeqs: []int64{1}}, nil).Once()

	outbox.EXPECT().MarkSynced(mock.Anything, []string{"op-1"}).Return(nil).Once()
	state.EXPECT().AdvancePushedSeq(mock.Anything, testCloudNodeID, int64(1)).Return(nil).Once()

	result, err := newPushService(t, outbox, state, client, 100).PushPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Batches)
	assert.Equal(t, 1, result.PushedOps)
	// ListUnsynced ran exactly once: a partial ack does not trigger another batch.
	outbox.AssertNumberOfCalls(t, "ListUnsynced", 1)
}

// TestPushPending_MultipleBatches drains a full batch, loops, then stops on the short
// final batch — exercising the loop boundary.
func TestPushPending_MultipleBatches(t *testing.T) {
	ctx := context.Background()
	outbox := mocks.NewMockSyncOutboxRepository(t)
	state := mocks.NewMockSyncStateRepository(t)
	client := mocks.NewMockSyncPushClient(t)

	full := []*dto.SyncOutboxEntry{pushOutboxEntry("op-1", 1), pushOutboxEntry("op-2", 2)}
	short := []*dto.SyncOutboxEntry{pushOutboxEntry("op-3", 3)}
	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 2).Return(full, nil).Once()
	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 2).Return(short, nil).Once()

	client.EXPECT().Push(mock.Anything, mock.Anything).
		Return(&dto.SyncPushResponse{AckedOpIDs: []string{"op-1", "op-2"}, AckedSeqs: []int64{1, 2}}, nil).Once()
	client.EXPECT().Push(mock.Anything, mock.Anything).
		Return(&dto.SyncPushResponse{AckedOpIDs: []string{"op-3"}, AckedSeqs: []int64{3}}, nil).Once()

	outbox.EXPECT().MarkSynced(mock.Anything, []string{"op-1", "op-2"}).Return(nil).Once()
	outbox.EXPECT().MarkSynced(mock.Anything, []string{"op-3"}).Return(nil).Once()
	state.EXPECT().AdvancePushedSeq(mock.Anything, testCloudNodeID, int64(2)).Return(nil).Once()
	state.EXPECT().AdvancePushedSeq(mock.Anything, testCloudNodeID, int64(3)).Return(nil).Once()

	result, err := newPushService(t, outbox, state, client, 2).PushPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Batches)
	assert.Equal(t, 3, result.PushedOps)
}

func TestPushPending_PushError_Propagates(t *testing.T) {
	ctx := context.Background()
	outbox := mocks.NewMockSyncOutboxRepository(t)
	state := mocks.NewMockSyncStateRepository(t)
	client := mocks.NewMockSyncPushClient(t)

	entries := []*dto.SyncOutboxEntry{pushOutboxEntry("op-1", 1)}
	outbox.EXPECT().ListUnsynced(mock.Anything, testNodeID, 100).Return(entries, nil).Once()
	client.EXPECT().Push(mock.Anything, mock.Anything).Return(nil, errors.New("network down")).Once()

	result, err := newPushService(t, outbox, state, client, 100).PushPending(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, result.PushedOps)
	// A failed push must not stamp anything synced.
	outbox.AssertNotCalled(t, "MarkSynced", mock.Anything, mock.Anything)
}
