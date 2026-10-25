package repository

import (
	"context"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const userEmailUniqueIndex = "uq_users_email_lower"

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) ports.UserRepository {
	return &UserRepository{db: db}
}

type userModel struct {
	postgres.TenantOwned
	ID        string     `gorm:"type:uuid;primaryKey"`
	Username  string     `gorm:"type:varchar(255);not null;uniqueIndex"`
	Name      string     `gorm:"type:varchar(255);not null"`
	Email     *string    `gorm:"type:varchar(255)"`
	Password  string     `gorm:"type:varchar(255);not null"`
	Active    bool       `gorm:"type:boolean;not null;default:true"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `gorm:"type:timestamp"`
}

func (userModel) TableName() string {
	return "users"
}

func (r *UserRepository) Create(ctx context.Context, user *dto.User) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	model := &userModel{
		ID:        user.ID,
		Username:  user.Username,
		Name:      user.Name,
		Email:     user.Email,
		Password:  user.Password,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	if err := db.WithContext(ctx).Create(model).Error; err != nil {
		return mapUserWriteError(err)
	}

	user.ID = model.ID
	return nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*dto.User, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model userModel
	if err := db.WithContext(ctx).
		Where("username = ? AND deleted_at IS NULL", username).
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrUserNotFound
		}
		return nil, err
	}

	return r.toDTO(&model), nil
}

// FindByEmail runs without the org scope on purpose: the email is what names the organization.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*dto.UserInOrganization, error) {
	var model userModel
	if err := postgres.GetTxOrDB(ctx, r.db).WithContext(ctx).
		Where("lower(email) = lower(?) AND deleted_at IS NULL", email).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainError.ErrUserNotFound
		}
		return nil, err
	}

	return &dto.UserInOrganization{User: r.toDTO(&model), OrganizationID: model.OrganizationID}, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*dto.User, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model userModel
	if err := db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrUserNotFound
		}
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]*dto.User, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var models []userModel
	if err := db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	users := make([]*dto.User, len(models))
	for i := range models {
		users[i] = r.toDTO(&models[i])
	}

	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, user *dto.User) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).
		Model(&userModel{}).
		Where("id = ? AND deleted_at IS NULL", user.ID).
		Updates(map[string]interface{}{
			"name":       user.Name,
			"email":      user.Email,
			"active":     user.Active,
			"updated_at": user.UpdatedAt,
		})
	if result.Error != nil {
		return mapUserWriteError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id, hashedPassword string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).
		Model(&userModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainError.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) SoftDelete(ctx context.Context, id string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	now := time.Now()
	result := db.WithContext(ctx).
		Model(&userModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainError.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) toDTO(model *userModel) *dto.User {
	return &dto.User{
		ID:        model.ID,
		Username:  model.Username,
		Name:      model.Name,
		Email:     model.Email,
		Password:  model.Password,
		Active:    model.Active,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func mapUserWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == userEmailUniqueIndex {
		return domainError.ErrUserEmailTaken
	}
	return err
}
