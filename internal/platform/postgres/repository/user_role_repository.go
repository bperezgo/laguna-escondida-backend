package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

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
	db := postgres.GetTxOrDB(ctx, r.db)
	model := &userRoleModel{
		UserID:    userRole.UserID,
		RoleID:    userRole.RoleID,
		CreatedAt: userRole.CreatedAt,
	}

	return db.WithContext(ctx).Create(model).Error
}

func (r *UserRoleRepository) FindByUserID(ctx context.Context, userID string) ([]*dto.UserRole, error) {
	db := postgres.GetTxOrDB(ctx, r.db)
	var models []userRoleModel
	if err := db.WithContext(ctx).
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
	db := postgres.GetTxOrDB(ctx, r.db)
	var models []roleModel
	if err := db.WithContext(ctx).
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

func (r *UserRoleRepository) DeleteByUserID(ctx context.Context, userID string) error {
	db := postgres.GetTxOrDB(ctx, r.db)
	return db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&userRoleModel{}).Error
}

// CountUsersByRoleID counts active, non-deleted users that hold the given role.
func (r *UserRoleRepository) CountUsersByRoleID(ctx context.Context, roleID int) (int, error) {
	db := postgres.GetTxOrDB(ctx, r.db)
	var count int64
	if err := db.WithContext(ctx).
		Table("user_roles").
		Joins("INNER JOIN users ON users.id = user_roles.user_id").
		Where("user_roles.role_id = ? AND users.deleted_at IS NULL AND users.active = ?", roleID, true).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return int(count), nil
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
