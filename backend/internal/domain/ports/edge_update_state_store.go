package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// EdgeUpdateStateStore is the state directory shared with the appliance updater. The node
// only reads it and drops a request there; the updater is the one that acts.
type EdgeUpdateStateStore interface {
	Read(ctx context.Context) *dto.EdgeUpdateState
	// UpdatePending reports whether a request or an apply exists, even one whose file is unreadable.
	UpdatePending(ctx context.Context) (bool, error)
	WriteRequest(ctx context.Context, req *dto.EdgeUpdateRequest) error
}
