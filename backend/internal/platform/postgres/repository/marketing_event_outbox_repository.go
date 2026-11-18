package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MarketingEventOutboxRepository is both sides of the marketing-app outbox: services append inside
// their unit of work, and the sender claims and settles rows.
type MarketingEventOutboxRepository struct {
	db *gorm.DB
}

func NewMarketingEventOutboxRepository(db *gorm.DB) *MarketingEventOutboxRepository {
	return &MarketingEventOutboxRepository{db: db}
}

var (
	_ ports.MarketingEventOutbox = (*MarketingEventOutboxRepository)(nil)
	_ ports.MarketingEventStore  = (*MarketingEventOutboxRepository)(nil)
)

type marketingEventModel struct {
	postgres.TenantOwned
	EventID       string     `gorm:"column:event_id;primaryKey"`
	Type          string     `gorm:"column:type"`
	Path          string     `gorm:"column:path"`
	Body          []byte     `gorm:"column:body"`
	Status        string     `gorm:"column:status"`
	Attempts      int        `gorm:"column:attempts"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at"`
	LastError     *string    `gorm:"column:last_error"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	SentAt        *time.Time `gorm:"column:sent_at"`
}

func (marketingEventModel) TableName() string {
	return "marketing_event_outbox"
}

func (r *MarketingEventOutboxRepository) Append(ctx context.Context, event *dto.MarketingEvent) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Create(&marketingEventModel{
		EventID:       event.EventID,
		Type:          string(event.Type),
		Path:          event.Path,
		Body:          event.Body,
		Status:        string(dto.MarketingEventPending),
		NextAttemptAt: event.NextAttemptAt,
		CreatedAt:     event.CreatedAt,
	}).Error
}

func (r *MarketingEventOutboxRepository) ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.MarketingEvent, error) {
	var models []marketingEventModel
	err := postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", string(dto.MarketingEventPending), now).
			Order("next_attempt_at ASC, event_id ASC").
			Limit(limit).
			Find(&models).Error; err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}
		ids := make([]string, len(models))
		for i := range models {
			ids[i] = models[i].EventID
			models[i].Attempts++
		}
		return tx.Model(&marketingEventModel{}).Where("event_id IN ?", ids).Updates(map[string]any{
			"attempts":        gorm.Expr("attempts + 1"),
			"next_attempt_at": leaseUntil,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	events := make([]*dto.MarketingEvent, len(models))
	for i := range models {
		m := models[i]
		events[i] = &dto.MarketingEvent{
			EventID:       m.EventID,
			Type:          dto.MarketingEventType(m.Type),
			Path:          m.Path,
			Body:          m.Body,
			Status:        dto.MarketingEventStatus(m.Status),
			Attempts:      m.Attempts,
			NextAttemptAt: leaseUntil,
			LastError:     m.LastError,
			CreatedAt:     m.CreatedAt,
			SentAt:        m.SentAt,
		}
	}
	return events, nil
}

func (r *MarketingEventOutboxRepository) MarkSent(ctx context.Context, eventID string, at time.Time) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&marketingEventModel{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status": string(dto.MarketingEventSent), "sent_at": at, "last_error": nil,
	}).Error
}

func (r *MarketingEventOutboxRepository) MarkFailed(ctx context.Context, eventID string, status dto.MarketingEventStatus, nextAttemptAt time.Time, lastError string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&marketingEventModel{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status": string(status), "next_attempt_at": nextAttemptAt, "last_error": lastError,
	}).Error
}
