package repository

import (
	"context"
	"errors"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type nodeModel struct {
	postgres.TenantOwned
	ID   string `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
	Kind string `gorm:"column:kind"`
}

func (nodeModel) TableName() string {
	return "nodes"
}

type NodeRepository struct {
	db *gorm.DB
}

func NewNodeRepository(db *gorm.DB) ports.NodeRepository {
	return &NodeRepository{db: db}
}

// FindByID is how a calling node's organization is learned, so it runs before there is one
// and can't go through TenantDB.
func (r *NodeRepository) FindByID(ctx context.Context, id string) (*dto.Node, error) {
	if uuid.Validate(id) != nil {
		return nil, domainError.ErrNodeNotFound
	}
	var model nodeModel
	err := postgres.GetTxOrDB(ctx, r.db).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrNodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find node: %w", err)
	}
	return &dto.Node{ID: model.ID, Name: model.Name, Kind: model.Kind, OrganizationID: model.OrganizationID}, nil
}
