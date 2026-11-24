package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

type NodeRepository interface {
	// FindForAuth runs without an organization in ctx: the node is what names it.
	// Returns error.ErrNodeNotFound when no node has the id.
	FindForAuth(ctx context.Context, id string) (*dto.NodeAuthRecord, error)
	// FindActiveEdgeForUpdate locks the context organization's non-revoked edge node.
	// Returns error.ErrNodeNotFound when it has none.
	FindActiveEdgeForUpdate(ctx context.Context) (*dto.Node, error)
	// Insert registers an edge node for the context organization with its key's hash and
	// recognition prefix. Returns error.ErrBoxAlreadyEnrolled when the organization already has an active one.
	Insert(ctx context.Context, node *dto.Node, keyHash, keyPrefix string) error
	// SetKey gives an existing node its key and name.
	SetKey(ctx context.Context, id, name, keyHash, keyPrefix string, enrolledAt time.Time) error
	Revoke(ctx context.Context, id string, at time.Time) error
	// TouchLastSeen writes last_seen_at only when it is unset or older than the interval.
	TouchLastSeen(ctx context.Context, id string, at time.Time, minInterval time.Duration) error
}
