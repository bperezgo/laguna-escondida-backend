package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StaffNotificationOutboxRepository stores the messages to send and leases them to the sender.
type StaffNotificationOutboxRepository struct {
	db *gorm.DB
}

func NewStaffNotificationOutboxRepository(db *gorm.DB) ports.StaffNotificationRepository {
	return &StaffNotificationOutboxRepository{db: db}
}

type staffNotificationModel struct {
	postgres.TenantOwned
	ID            string     `gorm:"column:id;primaryKey"`
	RuleID        string     `gorm:"column:rule_id"`
	ChannelID     string     `gorm:"column:channel_id"`
	EventID       string     `gorm:"column:event_id"`
	Message       string     `gorm:"column:message;type:jsonb"`
	Status        string     `gorm:"column:status"`
	Attempts      int        `gorm:"column:attempts"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at"`
	ExpiresAt     time.Time  `gorm:"column:expires_at"`
	LastError     *string    `gorm:"column:last_error"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	SentAt        *time.Time `gorm:"column:sent_at"`
}

func (staffNotificationModel) TableName() string { return "staff_notifications" }

func (r *StaffNotificationOutboxRepository) Insert(ctx context.Context, n *dto.StaffNotification) (bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return false, err
	}
	message, err := json.Marshal(n.Message)
	if err != nil {
		return false, fmt.Errorf("marshal staff message: %w", err)
	}
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&staffNotificationModel{
		ID: n.ID, RuleID: n.RuleID, ChannelID: n.ChannelID, EventID: n.EventID, Message: string(message),
		Status: string(n.Status), Attempts: n.Attempts, NextAttemptAt: n.NextAttemptAt, ExpiresAt: n.ExpiresAt,
		LastError: n.LastError, CreatedAt: n.CreatedAt, SentAt: n.SentAt,
	})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *StaffNotificationOutboxRepository) ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.ClaimedStaffNotification, error) {
	var rows []staffNotificationModel
	rules := map[string]notificationRuleModel{}
	channels := map[string]notificationChannelModel{}
	err := postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", string(dto.StaffNotificationPending), now).
			Order("next_attempt_at ASC, id ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ruleIDs := make([]string, len(rows))
		channelIDs := make([]string, len(rows))
		ids := make([]string, len(rows))
		for i := range rows {
			ids[i], ruleIDs[i], channelIDs[i] = rows[i].ID, rows[i].RuleID, rows[i].ChannelID
			rows[i].Attempts++
		}
		if err := tx.Model(&staffNotificationModel{}).Where("id IN ?", ids).Updates(map[string]any{
			"attempts":        gorm.Expr("attempts + 1"),
			"next_attempt_at": leaseUntil,
		}).Error; err != nil {
			return err
		}
		var ruleModels []notificationRuleModel
		if err := tx.Where("id IN ?", ruleIDs).Find(&ruleModels).Error; err != nil {
			return err
		}
		for _, m := range ruleModels {
			rules[m.ID] = m
		}
		var channelModels []notificationChannelModel
		if err := tx.Where("id IN ?", channelIDs).Find(&channelModels).Error; err != nil {
			return err
		}
		for _, m := range channelModels {
			channels[m.ID] = m
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	claimed := make([]*dto.ClaimedStaffNotification, 0, len(rows))
	for i := range rows {
		notification, err := rows[i].toDTO(leaseUntil)
		if err != nil {
			return nil, err
		}
		rule, hasRule := rules[rows[i].RuleID]
		channel, hasChannel := channels[rows[i].ChannelID]
		claimed = append(claimed, &dto.ClaimedStaffNotification{
			Notification: notification,
			Live:         hasRule && hasChannel && rule.DeletedAt == nil && rule.Enabled && channel.DeletedAt == nil && channel.Enabled,
			ChannelType:  dto.NotificationChannelType(channel.Type),
			Secret:       channel.Secret,
		})
	}
	return claimed, nil
}

func (r *StaffNotificationOutboxRepository) MarkSent(ctx context.Context, id string, at time.Time) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&staffNotificationModel{}).Where("id = ?", id).Updates(map[string]any{
		"status": string(dto.StaffNotificationSent), "sent_at": at, "last_error": nil,
	}).Error
}

func (r *StaffNotificationOutboxRepository) MarkFailed(ctx context.Context, id string, status dto.StaffNotificationStatus, nextAttemptAt time.Time, lastError string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&staffNotificationModel{}).Where("id = ?", id).Updates(map[string]any{
		"status": string(status), "next_attempt_at": nextAttemptAt, "last_error": lastError,
	}).Error
}

func (m *staffNotificationModel) toDTO(nextAttemptAt time.Time) (*dto.StaffNotification, error) {
	var message dto.StaffMessage
	if err := json.Unmarshal([]byte(m.Message), &message); err != nil {
		return nil, fmt.Errorf("decode message of staff notification %s: %w", m.ID, err)
	}
	return &dto.StaffNotification{
		ID: m.ID, OrganizationID: m.OrganizationID, RuleID: m.RuleID, ChannelID: m.ChannelID, EventID: m.EventID,
		Message: message, Status: dto.StaffNotificationStatus(m.Status), Attempts: m.Attempts,
		NextAttemptAt: nextAttemptAt, ExpiresAt: m.ExpiresAt, LastError: m.LastError, CreatedAt: m.CreatedAt, SentAt: m.SentAt,
	}, nil
}
