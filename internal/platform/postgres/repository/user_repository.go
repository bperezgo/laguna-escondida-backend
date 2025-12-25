package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) ports.UserRepository {
	return &UserRepository{db: db}
}

type userModel struct {
	ID        string     `gorm:"type:uuid;primaryKey"`
	Username  string     `gorm:"type:varchar(255);not null;uniqueIndex"`
	Name      string     `gorm:"type:varchar(255);not null;default:'undefined'"`
	Password  string     `gorm:"type:varchar(255);not null"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `gorm:"type:timestamp"`
}

func (userModel) TableName() string {
	return "users"
}

func (r *UserRepository) Create(ctx context.Context, user *dto.User) error {
	model := &userModel{
		ID:        user.ID,
		Username:  user.Username,
		Name:      user.Name,
		Password:  user.Password,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	user.ID = model.ID
	return nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*dto.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).
		Where("username = ? AND deleted_at IS NULL", username).
		First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*dto.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *UserRepository) toDTO(model *userModel) *dto.User {
	return &dto.User{
		ID:        model.ID,
		Username:  model.Username,
		Name:      model.Name,
		Password:  model.Password,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
