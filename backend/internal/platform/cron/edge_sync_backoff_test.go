package cron

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testBoxNodeID = "00000000-0000-0000-0000-0000000000bb"

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type backoffRig struct {
	scheduler  *EdgeSyncScheduler
	clock      *testClock
	pushClient *mocks.MockSyncPushClient
	pullClient *mocks.MockSyncPullClient
	stockPull  *mocks.MockSyncStockPullClient
	tracker    *syncstatus.Tracker
}

func newBackoffRig(t *testing.T) *backoffRig {
	t.Helper()
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	identity := dto.SyncIdentity{NodeID: testBoxNodeID, CloudNodeID: testCloudNodeID}
	r := &backoffRig{
		clock:      &testClock{now: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)},
		pushClient: mocks.NewMockSyncPushClient(t),
		pullClient: mocks.NewMockSyncPullClient(t),
		stockPull:  mocks.NewMockSyncStockPullClient(t),
		tracker:    syncstatus.NewTracker(time.Hour),
	}
	outbox := mocks.NewMockSyncOutboxRepository(t)
	outbox.EXPECT().ListUnsynced(mock.Anything, testBoxNodeID, mock.Anything).
		Return([]*dto.SyncOutboxEntry{{OpID: uuid.NewString()}}, nil).Maybe()
	state := mocks.NewMockSyncStateRepository(t)
	state.EXPECT().GetPulledCursor(mock.Anything, testCloudNodeID).Return(nil, nil).Maybe()
	state.EXPECT().GetStockPulledCursor(mock.Anything, testCloudNodeID).Return(nil, nil).Maybe()
	writer := mocks.NewMockSyncReferenceWriter(t)
	for _, method := range []string{"UpsertProducts", "UpsertProductResponsibilities", "UpsertProductIngredients", "ReplacePlateOptionSlots", "UpsertUsers", "UpsertSuppliers"} {
		writer.On(method, mock.Anything, mock.Anything).Return(nil).Maybe()
	}
	uow := passthroughUnitOfWork(t)

	scheduler, err := NewEdgeSyncScheduler(
		cronOrganizations(t), cronOrgA,
		service.NewSyncPushService(uow, outbox, state, r.pushClient, identity, 10, discard),
		service.NewSyncPullService(uow, r.pullClient, writer, state, identity, discard),
		service.NewStockPullService(uow, r.stockPull, writer, state, identity, discard),
		r.tracker, "* * * * *", "* * * * *", "0 5 * * *", discard,
		WithClock(r.clock.Now),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheduler.Stop() })
	r.scheduler = scheduler
	return r
}

func refusal(reason error, retryAfter time.Duration) error {
	return fmt.Errorf("sync pull returned status 403: %w", &domainError.NodeRefusalError{Reason: reason, RetryAfter: retryAfter})
}

func TestEdgeSyncScheduler_RefusalPausesEveryLoop(t *testing.T) {
	r := newBackoffRig(t)
	r.pullClient.EXPECT().Pull(mock.Anything, mock.Anything).Return(nil, refusal(domainError.ErrNodeOrganizationSuspended, 6*time.Hour)).Once()

	r.scheduler.pullJob()
	r.clock.advance(6*time.Hour - time.Second)
	r.scheduler.pushJob()
	r.scheduler.pullJob()
	r.scheduler.stockPullJob()

	r.pushClient.AssertNotCalled(t, "Push", mock.Anything, mock.Anything)
	r.stockPull.AssertNotCalled(t, "PullStock", mock.Anything, mock.Anything)
	assert.Equal(t, syncstatus.BlockedSuspended, r.tracker.Blocked(), "the POS keeps showing the blocked status")
}

func TestEdgeSyncScheduler_StockPullRefusalPausesTheOtherLoops(t *testing.T) {
	r := newBackoffRig(t)
	r.stockPull.EXPECT().PullStock(mock.Anything, mock.Anything).Return(nil, refusal(domainError.ErrNodeRevoked, time.Hour)).Once()

	r.scheduler.stockPullJob()
	r.scheduler.pushJob()
	r.scheduler.pullJob()

	r.pushClient.AssertNotCalled(t, "Push", mock.Anything, mock.Anything)
	r.pullClient.AssertNotCalled(t, "Pull", mock.Anything, mock.Anything)
}

func TestEdgeSyncScheduler_RefusalPauseIsClamped(t *testing.T) {
	cases := []struct {
		name       string
		retryAfter time.Duration
		want       time.Duration
	}{
		{"excessive delay", 999999 * time.Second, 24 * time.Hour},
		{"missing delay", 0, 6 * time.Hour},
		{"too short", 10 * time.Second, time.Minute},
		{"within bounds", 2 * time.Hour, 2 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newBackoffRig(t)

			r.scheduler.recordRefusal(refusal(domainError.ErrNodeOrganizationSuspended, tc.retryAfter))

			assert.Equal(t, r.clock.Now().Add(tc.want), r.tracker.PausedUntil())
		})
	}
}

func TestEdgeSyncScheduler_InvalidKeyPausesWithoutBlockedReason(t *testing.T) {
	r := newBackoffRig(t)

	assert.False(t, r.scheduler.recordRefusal(refusal(domainError.ErrNodeKeyInvalid, time.Hour)))

	assert.Equal(t, syncstatus.Blocked(""), r.tracker.Blocked(), "the POS keeps showing offline")
	assert.Equal(t, r.clock.Now().Add(time.Hour), r.tracker.PausedUntil())
}

func TestEdgeSyncScheduler_OneCallAfterThePause(t *testing.T) {
	r := newBackoffRig(t)
	r.pullClient.EXPECT().Pull(mock.Anything, mock.Anything).Return(nil, refusal(domainError.ErrNodeOrganizationSuspended, time.Hour)).Twice()

	r.scheduler.pullJob()
	r.clock.advance(time.Hour)
	r.scheduler.pullJob()
	r.scheduler.pushJob()

	r.pushClient.AssertNotCalled(t, "Push", mock.Anything, mock.Anything)
	assert.Equal(t, r.clock.Now().Add(time.Hour), r.tracker.PausedUntil(), "a new refusal starts a new pause")
}

func TestEdgeSyncScheduler_SuccessAfterThePauseResumes(t *testing.T) {
	r := newBackoffRig(t)
	r.pullClient.EXPECT().Pull(mock.Anything, mock.Anything).Return(nil, refusal(domainError.ErrNodeOrganizationSuspended, time.Hour)).Once()
	r.pullClient.EXPECT().Pull(mock.Anything, mock.Anything).Return(&dto.SyncPullResponse{}, nil).Once()
	r.pushClient.EXPECT().Push(mock.Anything, mock.Anything).Return(&dto.SyncPushResponse{}, nil).Once()

	r.scheduler.pullJob()
	r.clock.advance(time.Hour)
	r.scheduler.pullJob()
	r.scheduler.pushJob()

	assert.True(t, r.tracker.PausedUntil().IsZero())
	assert.Equal(t, syncstatus.Blocked(""), r.tracker.Blocked())
}

func TestEdgeSyncScheduler_NetworkFailureDoesNotPause(t *testing.T) {
	r := newBackoffRig(t)
	r.pullClient.EXPECT().Pull(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("send sync pull request: %w", assert.AnError)).Twice()

	r.scheduler.pullJob()
	r.scheduler.pullJob()

	assert.True(t, r.tracker.PausedUntil().IsZero())
	assert.False(t, r.tracker.Online())
}

func TestEdgeSyncScheduler_OffsetIsStableAndBounded(t *testing.T) {
	r := newBackoffRig(t)
	WithNodeID(testBoxNodeID)(r.scheduler)

	first := r.scheduler.offset(minuteLoopSpread)
	assert.Equal(t, first, r.scheduler.offset(minuteLoopSpread), "same node, same offset")
	assert.Less(t, first, minuteLoopSpread)
	assert.Less(t, r.scheduler.offset(dailyLoopSpread), dailyLoopSpread)
}

func TestEdgeSyncScheduler_OffsetsSpreadAcrossTheMinute(t *testing.T) {
	r := newBackoffRig(t)
	perSecond := map[int]int{}
	const boxes = 1000
	for i := 0; i < boxes; i++ {
		WithNodeID(uuid.NewString())(r.scheduler)
		offset := r.scheduler.offset(minuteLoopSpread)
		require.Less(t, offset, minuteLoopSpread)
		perSecond[int(offset/time.Second)]++
	}

	assert.Len(t, perSecond, 45, "every second of the spread is used")
	for second, n := range perSecond {
		assert.LessOrEqual(t, n, boxes*5/100, "second %d holds %d boxes", second, n)
	}
}

func TestEdgeSyncScheduler_StopInterruptsTheOffsetWait(t *testing.T) {
	r := newBackoffRig(t)
	WithNodeID(testBoxNodeID)(r.scheduler)
	ran := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		r.scheduler.spread(time.Hour, func() { ran <- struct{}{} })()
		close(done)
	}()

	start := time.Now()
	require.NoError(t, r.scheduler.Stop())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not interrupt the offset wait")
	}
	assert.Less(t, time.Since(start), time.Second)
	assert.Empty(t, ran, "the job does not run after Stop")
}
