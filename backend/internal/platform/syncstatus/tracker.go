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
	blocked     Blocked
	pausedUntil time.Time
	staleWindow time.Duration
}

// Blocked is why the cloud refuses this box, as the status endpoint reports it; empty when it doesn't.
type Blocked string

const (
	BlockedReplaced  Blocked = "replaced"
	BlockedSuspended Blocked = "suspended"
)

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
	t.blocked = ""
	t.pausedUntil = time.Time{}
}

// Pause holds the box's sync calls until the given time, as a refusal's Retry-After asked; memory only.
func (t *Tracker) Pause(until time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pausedUntil = until
}

// PausedUntil is zero when sync isn't paused.
func (t *Tracker) PausedUntil() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.pausedUntil
}

// RecordBlocked notes that the cloud answered but refused this box; it stays until a sync succeeds.
func (t *Tracker) RecordBlocked(reason Blocked) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastAttempt = time.Now()
	t.lastOK = false
	t.blocked = reason
}

func (t *Tracker) Blocked() Blocked {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.blocked
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
