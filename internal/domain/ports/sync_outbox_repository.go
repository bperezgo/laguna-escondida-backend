package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// SyncOutboxRepository is the transactional outbox: the write side (Append) used by
// business services, plus the read/mark side (ListUnsynced/MarkSynced) used by the
// edge push loop to drain pending rows to the cloud.
//
// Append MUST be called inside a UnitOfWork transaction so the outbox row and the
// business change it describes commit atomically (Option A). The caller must set
// OpID (a client-generated UUID v7 idempotency key); Append assigns the entry's
// per-origin Seq and CreatedAt before persisting.
type SyncOutboxRepository interface {
	Append(ctx context.Context, entry *dto.SyncOutboxEntry) error
	// ListUnsynced returns this origin's not-yet-acknowledged rows in seq order,
	// capped at limit — the next batch the push loop ships to the cloud.
	ListUnsynced(ctx context.Context, originNodeID string, limit int) ([]*dto.SyncOutboxEntry, error)
	// MarkSynced stamps synced_at on the given op_ids once the cloud has acked them,
	// so a later ListUnsynced no longer returns them. A no-op for an empty slice.
	MarkSynced(ctx context.Context, opIDs []string) error
	// PendingStats summarizes this origin's not-yet-synced rows — the count and the
	// oldest created_at (nil when none) — powering the edge status endpoint's backlog
	// and lag figures.
	PendingStats(ctx context.Context, originNodeID string) (*dto.SyncOutboxPendingStats, error)
	// HasUnsyncedFromOtherOrigins returns true when the outbox contains unsynced rows
	// whose origin_node_id differs from myOriginNodeID. Used by the push loop to detect
	// a stale node-identity: rows written under a previous NODE_ID are silently skipped
	// by ListUnsynced, and this check surfaces that mismatch so it can be logged.
	HasUnsyncedFromOtherOrigins(ctx context.Context, myOriginNodeID string) (bool, error)
}
