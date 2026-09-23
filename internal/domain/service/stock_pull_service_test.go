package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/stockmovement"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stockPullRig struct {
	service    *StockPullService
	pullClient *mocks.MockSyncStockPullClient
	writer     *mocks.MockSyncReferenceWriter
	stateRepo  *mocks.MockSyncStateRepository
}

func newStockPullRig(t *testing.T) *stockPullRig {
	t.Helper()
	pullClient := mocks.NewMockSyncStockPullClient(t)
	writer := mocks.NewMockSyncReferenceWriter(t)
	stateRepo := mocks.NewMockSyncStateRepository(t)

	return &stockPullRig{
		service: NewStockPullService(
			createMockUnitOfWork(t), pullClient, writer, stateRepo,
			dto.SyncIdentity{NodeID: testNodeID, CloudNodeID: testCloudNodeID},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		),
		pullClient: pullClient,
		writer:     writer,
		stateRepo:  stateRepo,
	}
}

func stockRow(productID string, amount int, updatedAt time.Time) dto.StockSyncPayload {
	return dto.StockSyncPayload{
		ProductID:     productID,
		Version:       1,
		Amount:        amount,
		UnitOfMeasure: "unit",
		CreatedAt:     updatedAt,
		UpdatedAt:     updatedAt,
	}
}

// With no stored cursor the edge asks from the beginning of time and adopts everything.
func TestStockPullService_RefreshStock_FirstFullRefresh(t *testing.T) {
	ctx := context.Background()
	rig := newStockPullRig(t)
	at := time.Now()

	rig.stateRepo.EXPECT().GetStockPulledCursor(ctx, testCloudNodeID).Return(nil, nil).Once()
	rig.pullClient.EXPECT().PullStock(ctx, time.Time{}).Return(&dto.SyncStockPullResponse{
		Stock:  []dto.StockSyncPayload{stockRow("product-1", 40, at), stockRow("product-2", 7, at)},
		Cursor: at,
	}, nil).Once()

	var written []dto.StockSyncPayload
	rig.writer.EXPECT().ReplaceStockAmounts(mock.Anything, mock.Anything).
		Run(func(_ context.Context, stocks []dto.StockSyncPayload) { written = stocks }).
		Return(nil).Once()
	rig.stateRepo.EXPECT().AdvanceStockPulledCursor(mock.Anything, testCloudNodeID, at).Return(nil).Once()

	result, err := rig.service.RefreshStock(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Stock)
	require.Len(t, written, 2)
	assert.Equal(t, 40, written[0].Amount)
}

func TestStockPullService_RefreshStock_IncrementalRefresh(t *testing.T) {
	ctx := context.Background()
	rig := newStockPullRig(t)
	since := time.Now().Add(-24 * time.Hour)
	at := time.Now()

	rig.stateRepo.EXPECT().GetStockPulledCursor(ctx, testCloudNodeID).Return(&since, nil).Once()
	rig.pullClient.EXPECT().PullStock(ctx, since).Return(&dto.SyncStockPullResponse{
		Stock:  []dto.StockSyncPayload{stockRow("product-1", 55, at)},
		Cursor: at,
	}, nil).Once()
	rig.writer.EXPECT().ReplaceStockAmounts(mock.Anything, mock.Anything).Return(nil).Once()
	rig.stateRepo.EXPECT().AdvanceStockPulledCursor(mock.Anything, testCloudNodeID, at).Return(nil).Once()

	result, err := rig.service.RefreshStock(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Stock)
}

// A stock row the office deleted arrives with deleted_at set, so the removal lands here too.
func TestStockPullService_RefreshStock_SoftDeletedRow(t *testing.T) {
	ctx := context.Background()
	rig := newStockPullRig(t)
	at := time.Now()
	deleted := stockRow("product-1", 12, at)
	deleted.DeletedAt = &at

	rig.stateRepo.EXPECT().GetStockPulledCursor(ctx, testCloudNodeID).Return(nil, nil).Once()
	rig.pullClient.EXPECT().PullStock(ctx, time.Time{}).Return(&dto.SyncStockPullResponse{
		Stock:  []dto.StockSyncPayload{deleted},
		Cursor: at,
	}, nil).Once()

	var written []dto.StockSyncPayload
	rig.writer.EXPECT().ReplaceStockAmounts(mock.Anything, mock.Anything).
		Run(func(_ context.Context, stocks []dto.StockSyncPayload) { written = stocks }).
		Return(nil).Once()
	rig.stateRepo.EXPECT().AdvanceStockPulledCursor(mock.Anything, testCloudNodeID, at).Return(nil).Once()

	_, err := rig.service.RefreshStock(ctx)

	require.NoError(t, err)
	require.Len(t, written, 1)
	assert.NotNil(t, written[0].DeletedAt)
}

// An unreachable cloud must leave the restaurant exactly as it was: same amounts, same
// cursor. Selling does not depend on this job, so its failure changes nothing.
func TestStockPullService_RefreshStock_UnreachableCloudChangesNothing(t *testing.T) {
	ctx := context.Background()
	rig := newStockPullRig(t)
	since := time.Now().Add(-24 * time.Hour)

	rig.stateRepo.EXPECT().GetStockPulledCursor(ctx, testCloudNodeID).Return(&since, nil).Once()
	rig.pullClient.EXPECT().PullStock(ctx, since).Return(nil, errors.New("connection refused")).Once()

	result, err := rig.service.RefreshStock(ctx)

	require.Error(t, err)
	assert.Nil(t, result)
	rig.writer.AssertNotCalled(t, "ReplaceStockAmounts", mock.Anything, mock.Anything)
	rig.stateRepo.AssertNotCalled(t, "AdvanceStockPulledCursor", mock.Anything, mock.Anything, mock.Anything)
}

// A response that carries nothing new leaves the bookmark alone.
func TestStockPullService_RefreshStock_UnchangedResponseDoesNotMoveTheCursor(t *testing.T) {
	ctx := context.Background()
	rig := newStockPullRig(t)
	since := time.Now().Add(-time.Hour)

	rig.stateRepo.EXPECT().GetStockPulledCursor(ctx, testCloudNodeID).Return(&since, nil).Once()
	rig.pullClient.EXPECT().PullStock(ctx, since).Return(&dto.SyncStockPullResponse{Cursor: since}, nil).Once()
	rig.writer.EXPECT().ReplaceStockAmounts(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := rig.service.RefreshStock(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Stock)
	rig.stateRepo.AssertNotCalled(t, "AdvanceStockPulledCursor", mock.Anything, mock.Anything, mock.Anything)
}

// A refresh replaces amounts only. Movements already queued for the cloud survive it and
// still push — what the restaurant sold is its own to report, and the cloud folds it on top
// of the number this refresh just wrote. Losing them would silently lose those sales.
func TestStockPullService_RefreshStock_DoesNotDiscardQueuedMovements(t *testing.T) {
	ctx := context.Background()
	rig := newStockPullRig(t)
	at := time.Now()

	// Two local sales queued for the cloud before the refresh runs.
	outbox := mocks.NewMockSyncOutboxRepository(t)
	var queued []*dto.SyncOutboxEntry
	outbox.EXPECT().Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { queued = append(queued, entry) }).
		Return(nil).Twice()

	emitter := stockmovement.NewOutboxEmitter(outbox, testNodeID)
	for _, change := range []int{-2, -3} {
		require.NoError(t, emitter.Emit(ctx, &dto.HistoricStock{
			OpID:          uuid.NewString(),
			ProductID:     "product-1",
			UnitOfMeasure: dto.UnitOfMeasureUnit,
			Change:        change,
			Kind:          dto.StockMovementKindSale,
			CreatedAt:     at,
		}))
	}
	require.Len(t, queued, 2)

	rig.stateRepo.EXPECT().GetStockPulledCursor(ctx, testCloudNodeID).Return(nil, nil).Once()
	rig.pullClient.EXPECT().PullStock(ctx, time.Time{}).Return(&dto.SyncStockPullResponse{
		Stock:  []dto.StockSyncPayload{stockRow("product-1", 40, at)},
		Cursor: at,
	}, nil).Once()
	rig.writer.EXPECT().ReplaceStockAmounts(mock.Anything, mock.Anything).Return(nil).Once()
	rig.stateRepo.EXPECT().AdvanceStockPulledCursor(mock.Anything, testCloudNodeID, at).Return(nil).Once()

	_, err := rig.service.RefreshStock(ctx)
	require.NoError(t, err)

	// The refresh never touches the outbox: no delete, no mark-synced, nothing.
	outbox.AssertNotCalled(t, "MarkSynced", mock.Anything, mock.Anything)
	outbox.AssertNotCalled(t, "FindUnsynced", mock.Anything, mock.Anything, mock.Anything)
	require.Len(t, queued, 2, "both movements are still queued after the refresh")
	for _, entry := range queued {
		assert.Equal(t, dto.SyncEntityHistoricStock, entry.EntityType)
		assert.Nil(t, entry.SyncedAt, "still waiting to be delivered")
	}
}
