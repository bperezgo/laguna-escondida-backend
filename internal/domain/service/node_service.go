package service

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/ports"
)

// NodeService tells which organization a calling edge node syncs for.
type NodeService struct {
	nodes ports.NodeRepository
}

func NewNodeService(nodes ports.NodeRepository) *NodeService {
	return &NodeService{nodes: nodes}
}

// OrganizationOf returns error.ErrNodeNotFound for an unregistered node.
func (s *NodeService) OrganizationOf(ctx context.Context, nodeID string) (string, error) {
	node, err := s.nodes.FindByID(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("resolve node %s: %w", nodeID, err)
	}
	return node.OrganizationID, nil
}
