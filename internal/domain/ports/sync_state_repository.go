package ports

import (
	"context"
	"time"
)

// SyncStateRepository tracks per-peer high-water marks (one sync_state row per peer
// node). The push loop advances last_pushed_seq for the cloud peer as ops are acked;
// the pull loop advances last_pulled_cursor as reference changes are applied.
type SyncStateRepository interface {
	// AdvancePushedSeq raises last_pushed_seq for peerNodeID to seq, never moving the
	// mark backwards (a stale or out-of-order ack must not lower it).
	AdvancePushedSeq(ctx context.Context, peerNodeID string, seq int64) error
	// GetPulledCursor returns the peer's last_pulled_cursor, or nil if the edge has
	// never pulled from it (so the caller pulls from the beginning of time).
	GetPulledCursor(ctx context.Context, peerNodeID string) (*time.Time, error)
	// AdvancePulledCursor raises last_pulled_cursor for peerNodeID to cursor, never
	// moving it backwards.
	AdvancePulledCursor(ctx context.Context, peerNodeID string, cursor time.Time) error
}
