package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type UserRoleRepository struct {
	db *gorm.DB
}

func NewUserRoleRepository(db *gorm.DB) ports.UserRoleRepository {
	return &UserRoleRepository{db: db}
}

type userRoleModel struct {
	ID        int       `gorm:"type:serial;primaryKey"`
	UserID    string    `gorm:"type:uuid;not null"`
	RoleID    int       `gorm:"type:integer;not null"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (userRoleModel) TableName() string {
	return "user_roles"
}

func (r *UserRoleRepository) Create(ctx context.Context, userRole *dto.UserRole) error {
	model := &userRoleModel{
		UserID:    userRole.UserID,
		RoleID:    userRole.RoleID,
		CreatedAt: userRole.CreatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *UserRoleRepository) FindByUserID(ctx context.Context, userID string) ([]*dto.UserRole, error) {
	var models []userRoleModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&models).Error; err != nil {
		return nil, err
	}

	userRoles := make([]*dto.UserRole, len(models))
	for i, model := range models {
		userRoles[i] = r.toDTO(&model)
	}

	return userRoles, nil
}

func (r *UserRoleRepository) FindRolesByUserID(ctx context.Context, userID string) ([]*dto.Role, error) {
	var models []roleModel
	if err := r.db.WithContext(ctx).
		Table("roles").
		Joins("INNER JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&models).Error; err != nil {
		return nil, err
	}

	roles := make([]*dto.Role, len(models))
	for i, model := range models {
		roles[i] = r.roleToDTO(&model)
	}

	return roles, nil
}

func (r *UserRoleRepository) toDTO(model *userRoleModel) *dto.UserRole {
	return &dto.UserRole{
		ID:        model.ID,
		UserID:    model.UserID,
		RoleID:    model.RoleID,
		CreatedAt: model.CreatedAt,
	}
}

func (r *UserRoleRepository) roleToDTO(model *roleModel) *dto.Role {
	return &dto.Role{
		ID:        model.ID,
		Name:      model.Name,
		CreatedAt: model.CreatedAt,
	}
}
