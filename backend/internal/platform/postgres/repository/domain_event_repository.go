package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DomainEventRepository is both sides of the durable event bus: services publish inside their unit of
// work, and the relay claims and settles deliveries.
type DomainEventRepository struct {
	db        *gorm.DB
	consumers *eventconsumer.Registry
}

func NewDomainEventRepository(db *gorm.DB, consumers *eventconsumer.Registry) *DomainEventRepository {
	return &DomainEventRepository{db: db, consumers: consumers}
}

var (
	_ ports.EventPublisher     = (*DomainEventRepository)(nil)
	_ ports.EventDeliveryStore = (*DomainEventRepository)(nil)
)

type domainEventModel struct {
	postgres.TenantOwned
	ID         string    `gorm:"column:id;primaryKey"`
	Name       string    `gorm:"column:name"`
	SubjectID  string    `gorm:"column:subject_id"`
	Payload    string    `gorm:"column:payload;type:jsonb"`
	OccurredAt time.Time `gorm:"column:occurred_at"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (domainEventModel) TableName() string { return "domain_events" }

type domainEventDeliveryModel struct {
	postgres.TenantOwned
	EventID       string     `gorm:"column:event_id;primaryKey"`
	Consumer      string     `gorm:"column:consumer;primaryKey"`
	Status        string     `gorm:"column:status"`
	Attempts      int        `gorm:"column:attempts"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at"`
	LeaseUntil    *time.Time `gorm:"column:lease_until"`
	LastError     *string    `gorm:"column:last_error"`
	DoneAt        *time.Time `gorm:"column:done_at"`
}

func (domainEventDeliveryModel) TableName() string { return "domain_event_deliveries" }

func (r *DomainEventRepository) Publish(ctx context.Context, event ports.DomainEvent) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", event.EventName(), err)
	}
	now := time.Now().UTC()
	if err := db.Create(&domainEventModel{
		ID: event.EventID(), Name: event.EventName(), SubjectID: event.SubjectID(),
		Payload: string(payload), OccurredAt: event.OccurredAt(), CreatedAt: now,
	}).Error; err != nil {
		return fmt.Errorf("store %s event: %w", event.EventName(), err)
	}
	for _, consumer := range r.consumers.ConsumersFor(event.EventName()) {
		if err := db.Create(&domainEventDeliveryModel{
			EventID: event.EventID(), Consumer: consumer, Status: string(dto.EventDeliveryPending), NextAttemptAt: now,
		}).Error; err != nil {
			return fmt.Errorf("store %s delivery for %s: %w", event.EventName(), consumer, err)
		}
	}
	return nil
}

func (r *DomainEventRepository) ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.ClaimedDelivery, error) {
	var deliveries []domainEventDeliveryModel
	var events []domainEventModel
	err := postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", string(dto.EventDeliveryPending), now).
			Order("next_attempt_at ASC, event_id ASC, consumer ASC").
			Limit(limit).
			Find(&deliveries).Error; err != nil {
			return err
		}
		if len(deliveries) == 0 {
			return nil
		}
		ids := make([]string, 0, len(deliveries))
		for i := range deliveries {
			ids = append(ids, deliveries[i].EventID)
			if err := tx.Model(&domainEventDeliveryModel{}).
				Where("event_id = ? AND consumer = ?", deliveries[i].EventID, deliveries[i].Consumer).
				Updates(map[string]any{
					"attempts":        gorm.Expr("attempts + 1"),
					"next_attempt_at": leaseUntil,
					"lease_until":     leaseUntil,
				}).Error; err != nil {
				return err
			}
			deliveries[i].Attempts++
		}
		return tx.Where("id IN ?", ids).Find(&events).Error
	})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]dto.StoredEvent, len(events))
	for _, e := range events {
		byID[e.ID] = dto.StoredEvent{
			ID: e.ID, Name: e.Name, OrganizationID: e.OrganizationID, SubjectID: e.SubjectID,
			OccurredAt: e.OccurredAt, Payload: []byte(e.Payload),
		}
	}
	claimed := make([]*dto.ClaimedDelivery, 0, len(deliveries))
	for _, d := range deliveries {
		claimed = append(claimed, &dto.ClaimedDelivery{Event: byID[d.EventID], Consumer: d.Consumer, Attempts: d.Attempts})
	}
	return claimed, nil
}

func (r *DomainEventRepository) MarkDone(ctx context.Context, eventID, consumer string, at time.Time) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&domainEventDeliveryModel{}).Where("event_id = ? AND consumer = ?", eventID, consumer).Updates(map[string]any{
		"status": string(dto.EventDeliveryDone), "done_at": at, "last_error": nil, "lease_until": nil,
	}).Error
}

func (r *DomainEventRepository) MarkFailed(ctx context.Context, eventID, consumer string, status dto.EventDeliveryStatus, nextAttemptAt time.Time, lastError string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&domainEventDeliveryModel{}).Where("event_id = ? AND consumer = ?", eventID, consumer).Updates(map[string]any{
		"status": string(status), "next_attempt_at": nextAttemptAt, "last_error": lastError, "lease_until": nil,
	}).Error
}
