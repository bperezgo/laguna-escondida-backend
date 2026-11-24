package syncstatus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTracker_OfflineBeforeFirstSync(t *testing.T) {
	tr := NewTracker(time.Minute)
	assert.False(t, tr.Online())
}

func TestTracker_OnlineAfterSuccess(t *testing.T) {
	tr := NewTracker(time.Minute)
	tr.RecordSuccess()
	assert.True(t, tr.Online())
}

func TestTracker_OfflineAfterFailure(t *testing.T) {
	tr := NewTracker(time.Minute)
	tr.RecordSuccess()
	tr.RecordFailure()
	assert.False(t, tr.Online())
}

func TestTracker_OfflineWhenStale(t *testing.T) {
	tr := NewTracker(10 * time.Millisecond)
	tr.RecordSuccess()
	time.Sleep(20 * time.Millisecond)
	assert.False(t, tr.Online())
}

func TestTracker_BlockedUntilASyncSucceeds(t *testing.T) {
	tr := NewTracker(time.Minute)
	tr.RecordBlocked(BlockedReplaced)
	assert.Equal(t, BlockedReplaced, tr.Blocked())
	assert.False(t, tr.Online())

	tr.RecordFailure()
	assert.Equal(t, BlockedReplaced, tr.Blocked(), "a network failure does not lift it")

	tr.RecordSuccess()
	assert.Empty(t, tr.Blocked())
}
