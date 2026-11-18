package repository

import (
	"context"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type NotificationChannelRepository struct {
	db *gorm.DB
}

func NewNotificationChannelRepository(db *gorm.DB) ports.NotificationChannelRepository {
	return &NotificationChannelRepository{db: db}
}

type notificationChannelModel struct {
	postgres.TenantOwned
	ID         string     `gorm:"column:id;primaryKey"`
	Name       string     `gorm:"column:name"`
	Type       string     `gorm:"column:type"`
	Enabled    bool       `gorm:"column:enabled"`
	Secret     []byte     `gorm:"column:secret"`
	SecretHint string     `gorm:"column:secret_hint"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
	DeletedAt  *time.Time `gorm:"column:deleted_at"`
}

func (notificationChannelModel) TableName() string { return "notification_channels" }

func (r *NotificationChannelRepository) Create(ctx context.Context, channel *dto.NotificationChannel) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Create(&notificationChannelModel{
		ID: channel.ID, Name: channel.Name, Type: string(channel.Type), Enabled: channel.Enabled,
		Secret: channel.Secret, SecretHint: channel.SecretHint, CreatedAt: channel.CreatedAt, UpdatedAt: channel.UpdatedAt,
	}).Error
}

func (r *NotificationChannelRepository) Update(ctx context.Context, channel *dto.NotificationChannel) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&notificationChannelModel{}).
		Where("id = ? AND deleted_at IS NULL", channel.ID).
		Updates(map[string]any{
			"name": channel.Name, "enabled": channel.Enabled, "secret": channel.Secret,
			"secret_hint": channel.SecretHint, "updated_at": channel.UpdatedAt,
		}).Error
}

func (r *NotificationChannelRepository) FindByID(ctx context.Context, id string) (*dto.NotificationChannel, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model notificationChannelModel
	err = db.Where("id = ? AND deleted_at IS NULL", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrNotificationChannelNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDTO(), nil
}

func (r *NotificationChannelRepository) List(ctx context.Context) ([]*dto.NotificationChannel, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var models []notificationChannelModel
	if err := db.Where("deleted_at IS NULL").Order("created_at ASC, id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	channels := make([]*dto.NotificationChannel, len(models))
	for i := range models {
		channels[i] = models[i].toDTO()
	}
	return channels, nil
}

// Delete soft-deletes the channel and its rules together, mirroring the FK cascade of a hard delete.
func (r *NotificationChannelRepository) Delete(ctx context.Context, id string) error {
	return postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&notificationRuleModel{}).
			Where("channel_id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&notificationChannelModel{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
	})
}

func (m *notificationChannelModel) toDTO() *dto.NotificationChannel {
	return &dto.NotificationChannel{
		ID: m.ID, OrganizationID: m.OrganizationID, Name: m.Name, Type: dto.NotificationChannelType(m.Type),
		Enabled: m.Enabled, Secret: m.Secret, SecretHint: m.SecretHint, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
