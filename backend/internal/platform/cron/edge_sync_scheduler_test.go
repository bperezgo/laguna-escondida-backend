package cron

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testCloudNodeID = "00000000-0000-0000-0000-0000000000cc"

// newStockRefreshScheduler builds an edge scheduler whose only usable job is the stock
// refresh; push and pull stay nil because registerJobs never calls them.
func newStockRefreshScheduler(t *testing.T, stockCron string) (*EdgeSyncScheduler, *mocks.MockSyncStockPullClient, *bytes.Buffer) {
	t.Helper()
	pullClient := mocks.NewMockSyncStockPullClient(t)
	writer := mocks.NewMockSyncReferenceWriter(t)
	stateRepo := mocks.NewMockSyncStateRepository(t)
	stateRepo.EXPECT().GetStockPulledCursor(mock.Anything, testCloudNodeID).Return(nil, nil).Maybe()

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	stockPull := service.NewStockPullService(
		passthroughUnitOfWork(t), pullClient, writer, stateRepo,
		dto.SyncIdentity{CloudNodeID: testCloudNodeID},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	scheduler, err := NewEdgeSyncScheduler(
		cronOrganizations(t), cronOrgA, nil, nil, stockPull,
		syncstatus.NewTracker(0),
		"* * * * *", "* * * * *", stockCron,
		logger,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheduler.Stop() })
	return scheduler, pullClient, logs
}

func passthroughUnitOfWork(t *testing.T) *mocks.MockUnitOfWork {
	t.Helper()
	mockUoW := mocks.NewMockUnitOfWork(t)
	mockUoW.EXPECT().Do(mock.Anything, mock.AnythingOfType("func(context.Context) error")).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		Maybe()
	return mockUoW
}

func TestEdgeSyncScheduler_RegistersStockRefreshJob(t *testing.T) {
	scheduler, _, _ := newStockRefreshScheduler(t, "0 5 * * *")

	require.NoError(t, scheduler.registerJobs())

	assert.Len(t, scheduler.scheduler.Jobs(), 3, "push, pull and the daily stock refresh")
}

func TestEdgeSyncScheduler_RejectsInvalidStockPullCron(t *testing.T) {
	scheduler, _, logs := newStockRefreshScheduler(t, "not a cron")

	assert.Error(t, scheduler.registerJobs())
	assert.Contains(t, logs.String(), "Failed to register stock pull cron job")
}

// An unreachable cloud must not take the job — or anything else — down with it.
func TestEdgeSyncScheduler_StockPullJob_ContainsFailure(t *testing.T) {
	scheduler, pullClient, logs := newStockRefreshScheduler(t, "0 5 * * *")
	pullClient.EXPECT().PullStock(mock.Anything, mock.Anything).Return(nil, assert.AnError).Maybe()

	assert.NotPanics(t, scheduler.stockPullJob)

	assert.Contains(t, logs.String(), "Edge stock refresh job failed")
}

// The box's background work acts for the box's organization; there is no request to name one.
func TestEdgeSyncScheduler_StockPullJob_RunsAsTheBoxOrganization(t *testing.T) {
	scheduler, pullClient, _ := newStockRefreshScheduler(t, "0 5 * * *")
	pullClient.EXPECT().PullStock(mock.MatchedBy(inCronOrg(cronOrgA)), mock.Anything).Return(nil, assert.AnError).Once()

	scheduler.stockPullJob()
}

func TestEdgeSyncScheduler_RecordRefusal(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantRefused bool
		want        syncstatus.Blocked
	}{
		{"replaced", fmt.Errorf("pull: %w", domainError.ErrNodeRevoked), true, syncstatus.BlockedReplaced},
		{"suspended", fmt.Errorf("pull: %w", domainError.ErrNodeOrganizationSuspended), true, syncstatus.BlockedSuspended},
		{"network failure", assert.AnError, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheduler, _, _ := newStockRefreshScheduler(t, "0 5 * * *")

			assert.Equal(t, tc.wantRefused, scheduler.recordRefusal(tc.err))

			assert.Equal(t, tc.want, scheduler.tracker.Blocked())
		})
	}
}

func TestEdgeSyncScheduler_PullClassifiesTheFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want dto.SyncState
	}{
		{"cloud unreachable", errors.New("pull from cloud: dial tcp: no route"), dto.SyncStateOffline},
		{"apply failed", fmt.Errorf("%w: upsert users: fk", domainError.ErrPullApplyFailed), dto.SyncStateFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRepo := mocks.NewMockSyncStateRepository(t)
			stateRepo.EXPECT().GetPulledCursor(mock.Anything, testCloudNodeID).Return(nil, nil).Once()
			client := mocks.NewMockSyncPullClient(t)
			client.EXPECT().Pull(mock.Anything, mock.Anything).Return(nil, tc.err).Once()
			pull := service.NewSyncPullService(passthroughUnitOfWork(t), client, mocks.NewMockSyncReferenceWriter(t), stateRepo,
				dto.SyncIdentity{CloudNodeID: testCloudNodeID}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			tracker := syncstatus.NewTracker(0)
			scheduler, err := NewEdgeSyncScheduler(cronOrganizations(t), cronOrgA, nil, pull, nil, tracker,
				"* * * * *", "* * * * *", "0 5 * * *", slog.New(slog.NewTextHandler(io.Discard, nil)))
			require.NoError(t, err)
			t.Cleanup(func() { _ = scheduler.Stop() })

			scheduler.pull(context.Background())

			assert.Equal(t, tc.want, tracker.SyncState())
		})
	}
}
