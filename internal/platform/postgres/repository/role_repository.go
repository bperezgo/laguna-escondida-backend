package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) ports.RoleRepository {
	return &RoleRepository{db: db}
}

type roleModel struct {
	ID        int       `gorm:"type:serial;primaryKey"`
	Name      string    `gorm:"type:varchar(50);not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (roleModel) TableName() string {
	return "roles"
}

func (r *RoleRepository) FindByID(ctx context.Context, id int) (*dto.Role, error) {
	var model roleModel
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *RoleRepository) FindByIDs(ctx context.Context, ids []int) ([]*dto.Role, error) {
	if len(ids) == 0 {
		return []*dto.Role{}, nil
	}

	var models []roleModel
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&models).Error; err != nil {
		return nil, err
	}

	roles := make([]*dto.Role, len(models))
	for i, model := range models {
		roles[i] = r.toDTO(&model)
	}

	return roles, nil
}

func (r *RoleRepository) FindAll(ctx context.Context) ([]*dto.Role, error) {
	var models []roleModel
	if err := r.db.WithContext(ctx).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	roles := make([]*dto.Role, len(models))
	for i := range models {
		roles[i] = r.toDTO(&models[i])
	}

	return roles, nil
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*dto.Role, error) {
	var model roleModel
	if err := r.db.WithContext(ctx).
		Where("name = ?", name).
		First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *RoleRepository) toDTO(model *roleModel) *dto.Role {
	return &dto.Role{
		ID:        model.ID,
		Name:      model.Name,
		CreatedAt: model.CreatedAt,
	}
}
