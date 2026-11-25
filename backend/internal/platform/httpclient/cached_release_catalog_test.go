package httpclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newCachedCatalog(t *testing.T) (*mocks.MockEdgeReleaseCatalog, *testClock, ports.EdgeReleaseCatalog) {
	upstream := mocks.NewMockEdgeReleaseCatalog(t)
	clock := &testClock{now: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return upstream, clock, NewCachedReleaseCatalog(upstream, logger, clock.Now)
}

var cachedTestReleases = []dto.EdgeRelease{{Version: "v1.4.0"}}

func TestCachedReleaseCatalog_FiftyCallsInTenMinutesReadUpstreamOnce(t *testing.T) {
	upstream, clock, catalog := newCachedCatalog(t)
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(cachedTestReleases, nil).Once()

	for range 50 {
		got, err := catalog.ListEdgeReleases(context.Background())
		require.NoError(t, err)
		assert.Equal(t, cachedTestReleases, got)
		clock.Advance(10 * time.Second)
	}
}

func TestCachedReleaseCatalog_RefreshesAfterTenMinutes(t *testing.T) {
	upstream, clock, catalog := newCachedCatalog(t)
	newer := []dto.EdgeRelease{{Version: "v1.5.0"}, {Version: "v1.4.0"}}
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(cachedTestReleases, nil).Once()
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(newer, nil).Once()

	_, err := catalog.ListEdgeReleases(context.Background())
	require.NoError(t, err)
	clock.Advance(10 * time.Minute)
	got, err := catalog.ListEdgeReleases(context.Background())

	require.NoError(t, err)
	assert.Equal(t, newer, got)
}

func TestCachedReleaseCatalog_ConcurrentCallsOnAnExpiredCacheReadUpstreamOnce(t *testing.T) {
	upstream, clock, catalog := newCachedCatalog(t)
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(cachedTestReleases, nil).Once()
	_, err := catalog.ListEdgeReleases(context.Background())
	require.NoError(t, err)
	clock.Advance(11 * time.Minute)

	var reads atomic.Int32
	upstream.EXPECT().ListEdgeReleases(mock.Anything).RunAndReturn(func(context.Context) ([]dto.EdgeRelease, error) {
		reads.Add(1)
		time.Sleep(20 * time.Millisecond)
		return cachedTestReleases, nil
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := catalog.ListEdgeReleases(context.Background())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), reads.Load())
}

func TestCachedReleaseCatalog_FailedRefreshReturnsThePreviousList(t *testing.T) {
	upstream, clock, catalog := newCachedCatalog(t)
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(cachedTestReleases, nil).Once()
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(nil, errors.New("github down"))

	_, err := catalog.ListEdgeReleases(context.Background())
	require.NoError(t, err)
	for range 3 {
		clock.Advance(11 * time.Minute)
		got, err := catalog.ListEdgeReleases(context.Background())
		require.NoError(t, err)
		assert.Equal(t, cachedTestReleases, got)
	}
}

func TestCachedReleaseCatalog_FailureBeforeAnySuccessReturnsTheError(t *testing.T) {
	upstream, clock, catalog := newCachedCatalog(t)
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(nil, errors.New("github down")).Once()
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(cachedTestReleases, nil).Once()

	_, err := catalog.ListEdgeReleases(context.Background())
	assert.Error(t, err)
	_, err = catalog.ListEdgeReleases(context.Background())
	assert.Error(t, err, "a failure is retried after a pause, not on every request")

	clock.Advance(time.Minute)
	got, err := catalog.ListEdgeReleases(context.Background())
	require.NoError(t, err)
	assert.Equal(t, cachedTestReleases, got)
}

func TestCachedReleaseCatalog_ManifestsAreCachedPerVersion(t *testing.T) {
	upstream, clock, catalog := newCachedCatalog(t)
	m14 := &dto.EdgeManifest{MinCloud: "v1.3.0", InstallerURL: "https://github.com/o/r/releases/download/edge/v1.4.0/K.exe"}
	m15 := &dto.EdgeManifest{MinCloud: "v1.4.0"}
	upstream.EXPECT().GetEdgeManifest(mock.Anything, "v1.4.0").Return(m14, nil).Once()
	upstream.EXPECT().GetEdgeManifest(mock.Anything, "v1.5.0").Return(m15, nil).Once()
	upstream.EXPECT().GetEdgeManifest(mock.Anything, "v1.4.0").Return(nil, errors.New("github down")).Once()

	for range 10 {
		got, err := catalog.GetEdgeManifest(context.Background(), "v1.4.0")
		require.NoError(t, err)
		assert.Equal(t, m14, got)
		got, err = catalog.GetEdgeManifest(context.Background(), "v1.5.0")
		require.NoError(t, err)
		assert.Equal(t, m15, got)
	}
	clock.Advance(11 * time.Minute)
	got, err := catalog.GetEdgeManifest(context.Background(), "v1.4.0")
	require.NoError(t, err)
	assert.Equal(t, m14, got, "a failed refresh keeps the last manifest")
}

func TestCachedReleaseCatalog_ACallerThatGaveUpDoesNotPauseTheNextOne(t *testing.T) {
	upstream, _, catalog := newCachedCatalog(t)
	ctx, cancel := context.WithCancel(context.Background())
	upstream.EXPECT().ListEdgeReleases(mock.Anything).RunAndReturn(func(context.Context) ([]dto.EdgeRelease, error) {
		cancel()
		return nil, context.Canceled
	}).Once()
	upstream.EXPECT().ListEdgeReleases(mock.Anything).Return(cachedTestReleases, nil).Once()

	_, err := catalog.ListEdgeReleases(ctx)
	assert.Error(t, err)
	got, err := catalog.ListEdgeReleases(context.Background())

	require.NoError(t, err)
	assert.Equal(t, cachedTestReleases, got)
}
