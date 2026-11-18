package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type NotificationRuleRepository struct {
	db *gorm.DB
}

func NewNotificationRuleRepository(db *gorm.DB) ports.NotificationRuleRepository {
	return &NotificationRuleRepository{db: db}
}

type notificationRuleModel struct {
	postgres.TenantOwned
	ID                  string     `gorm:"column:id;primaryKey"`
	ChannelID           string     `gorm:"column:channel_id"`
	Event               string     `gorm:"column:event"`
	Conditions          string     `gorm:"column:conditions;type:jsonb"`
	TitlePrefix         string     `gorm:"column:title_prefix"`
	ExpiresAfterMinutes int        `gorm:"column:expires_after_minutes"`
	Enabled             bool       `gorm:"column:enabled"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at"`
	DeletedAt           *time.Time `gorm:"column:deleted_at"`
}

func (notificationRuleModel) TableName() string { return "notification_rules" }

func (r *NotificationRuleRepository) Create(ctx context.Context, rule *dto.NotificationRule) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	conditions, err := json.Marshal(rule.Conditions)
	if err != nil {
		return fmt.Errorf("marshal rule conditions: %w", err)
	}
	return db.Create(&notificationRuleModel{
		ID: rule.ID, ChannelID: rule.ChannelID, Event: string(rule.Event), Conditions: string(conditions),
		TitlePrefix: rule.TitlePrefix, ExpiresAfterMinutes: rule.ExpiresAfterMinutes, Enabled: rule.Enabled,
		CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}).Error
}

func (r *NotificationRuleRepository) Update(ctx context.Context, rule *dto.NotificationRule) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	conditions, err := json.Marshal(rule.Conditions)
	if err != nil {
		return fmt.Errorf("marshal rule conditions: %w", err)
	}
	return db.Model(&notificationRuleModel{}).
		Where("id = ? AND deleted_at IS NULL", rule.ID).
		Updates(map[string]any{
			"channel_id": rule.ChannelID, "conditions": string(conditions), "title_prefix": rule.TitlePrefix,
			"expires_after_minutes": rule.ExpiresAfterMinutes, "enabled": rule.Enabled, "updated_at": rule.UpdatedAt,
		}).Error
}

func (r *NotificationRuleRepository) FindByID(ctx context.Context, id string) (*dto.NotificationRule, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model notificationRuleModel
	err = db.Where("id = ? AND deleted_at IS NULL", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrNotificationRuleNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDTO()
}

func (r *NotificationRuleRepository) List(ctx context.Context, channelID string) ([]*dto.NotificationRule, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	query := db.Where("deleted_at IS NULL")
	if channelID != "" {
		query = query.Where("channel_id = ?", channelID)
	}
	var models []notificationRuleModel
	if err := query.Order("created_at ASC, id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	return rulesToDTO(models)
}

func (r *NotificationRuleRepository) Delete(ctx context.Context, id string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return db.Model(&notificationRuleModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
}

func (r *NotificationRuleRepository) ListEnabledForEvent(ctx context.Context, event dto.NotificationEventType) ([]*dto.RuleWithChannel, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var rules []notificationRuleModel
	if findErr := db.Where("event = ? AND enabled AND deleted_at IS NULL", string(event)).
		Order("created_at ASC, id ASC").Find(&rules).Error; findErr != nil {
		return nil, findErr
	}
	if len(rules) == 0 {
		return nil, nil
	}
	channelIDs := make([]string, len(rules))
	for i := range rules {
		channelIDs[i] = rules[i].ChannelID
	}
	channelDB, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var channels []notificationChannelModel
	if err := channelDB.Where("id IN ? AND enabled AND deleted_at IS NULL", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]*notificationChannelModel, len(channels))
	for i := range channels {
		byID[channels[i].ID] = &channels[i]
	}
	result := make([]*dto.RuleWithChannel, 0, len(rules))
	for i := range rules {
		channel, ok := byID[rules[i].ChannelID]
		if !ok {
			continue
		}
		ruleDTO, err := rules[i].toDTO()
		if err != nil {
			return nil, err
		}
		result = append(result, &dto.RuleWithChannel{Rule: *ruleDTO, Channel: *channel.toDTO()})
	}
	return result, nil
}

func rulesToDTO(models []notificationRuleModel) ([]*dto.NotificationRule, error) {
	rules := make([]*dto.NotificationRule, len(models))
	for i := range models {
		rule, err := models[i].toDTO()
		if err != nil {
			return nil, err
		}
		rules[i] = rule
	}
	return rules, nil
}

func (m *notificationRuleModel) toDTO() (*dto.NotificationRule, error) {
	var conditions dto.NotificationConditions
	if err := json.Unmarshal([]byte(m.Conditions), &conditions); err != nil {
		return nil, fmt.Errorf("decode conditions of rule %s: %w", m.ID, err)
	}
	return &dto.NotificationRule{
		ID: m.ID, OrganizationID: m.OrganizationID, ChannelID: m.ChannelID, Event: dto.NotificationEventType(m.Event),
		Conditions: conditions, TitlePrefix: m.TitlePrefix, ExpiresAfterMinutes: m.ExpiresAfterMinutes,
		Enabled: m.Enabled, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}, nil
}
