package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type NodeRepository interface {
	// FindByID returns error.ErrNodeNotFound when no node has the id.
	FindByID(ctx context.Context, id string) (*dto.Node, error)
}
