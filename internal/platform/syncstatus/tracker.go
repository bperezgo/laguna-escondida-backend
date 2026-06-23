// Package syncstatus holds the edge node's live connectivity signal: a small in-memory
// record of the most recent sync attempt's outcome. The edge sync scheduler writes to it
// after each pull; the status handler reads it — so the status endpoint can report
// online/offline without issuing its own network probe to the cloud.
package syncstatus

import (
	"sync"
	"time"
)

// Tracker records whether the edge's most recent cloud contact succeeded and when. It is
// safe for concurrent use: the scheduler goroutine records outcomes while HTTP handler
// goroutines read Online.
type Tracker struct {
	mu          sync.RWMutex
	lastAttempt time.Time
	lastOK      bool
	staleWindow time.Duration
}

// NewTracker returns a Tracker that treats the node as offline until the first successful
// sync. staleWindow should comfortably exceed the sync interval (e.g. ~2.5× a 1-minute
// pull cron) so one slow or skipped round doesn't flap the online badge.
func NewTracker(staleWindow time.Duration) *Tracker {
	return &Tracker{staleWindow: staleWindow}
}

// RecordSuccess notes that a sync attempt just reached the cloud successfully.
func (t *Tracker) RecordSuccess() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastAttempt = time.Now()
	t.lastOK = true
}

// RecordFailure notes that a sync attempt just failed (e.g. the cloud was unreachable).
func (t *Tracker) RecordFailure() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastAttempt = time.Now()
	t.lastOK = false
}

// Online reports whether the edge currently considers itself connected to the cloud: the
// most recent attempt succeeded and landed within staleWindow. A node that has never
// synced, whose last attempt failed, or whose last success has gone stale reads offline.
func (t *Tracker) Online() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.lastOK || t.lastAttempt.IsZero() {
		return false
	}
	return time.Since(t.lastAttempt) <= t.staleWindow
}
