package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type leadModel struct {
	ID                 string     `gorm:"column:id;primaryKey"`
	Name               string     `gorm:"column:name"`
	RestaurantName     string     `gorm:"column:restaurant_name"`
	Phone              string     `gorm:"column:phone"`
	Status             string     `gorm:"column:status"`
	Notes              string     `gorm:"column:notes"`
	ConsentAt          time.Time  `gorm:"column:consent_at"`
	UTMSource          string     `gorm:"column:utm_source"`
	UTMMedium          string     `gorm:"column:utm_medium"`
	UTMCampaign        string     `gorm:"column:utm_campaign"`
	UTMTerm            string     `gorm:"column:utm_term"`
	UTMContent         string     `gorm:"column:utm_content"`
	FBCLID             string     `gorm:"column:fbclid"`
	GCLID              string     `gorm:"column:gclid"`
	LandingURL         string     `gorm:"column:landing_url"`
	Referrer           string     `gorm:"column:referrer"`
	EventID            string     `gorm:"column:event_id"`
	AlertStatus        string     `gorm:"column:alert_status"`
	AlertAttempts      int        `gorm:"column:alert_attempts"`
	AlertNextAttemptAt *time.Time `gorm:"column:alert_next_attempt_at"`
	AlertLastError     string     `gorm:"column:alert_last_error"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (leadModel) TableName() string { return "leads" }

func (m *leadModel) toDTO() *dto.Lead {
	return &dto.Lead{
		ID: m.ID, Name: m.Name, RestaurantName: m.RestaurantName, Phone: m.Phone,
		Status: dto.LeadStatus(m.Status), Notes: m.Notes, ConsentAt: m.ConsentAt,
		LeadAttribution: dto.LeadAttribution{
			UTMSource: m.UTMSource, UTMMedium: m.UTMMedium, UTMCampaign: m.UTMCampaign,
			UTMTerm: m.UTMTerm, UTMContent: m.UTMContent, FBCLID: m.FBCLID, GCLID: m.GCLID,
			LandingURL: m.LandingURL, Referrer: m.Referrer, EventID: m.EventID,
		},
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		AlertStatus: dto.LeadAlertStatus(m.AlertStatus), AlertAttempts: m.AlertAttempts,
		AlertNextAttemptAt: m.AlertNextAttemptAt, AlertLastError: m.AlertLastError,
	}
}

func leadToModel(l *dto.Lead) *leadModel {
	return &leadModel{
		ID: l.ID, Name: l.Name, RestaurantName: l.RestaurantName, Phone: l.Phone,
		Status: string(l.Status), Notes: l.Notes, ConsentAt: l.ConsentAt,
		UTMSource: l.UTMSource, UTMMedium: l.UTMMedium, UTMCampaign: l.UTMCampaign,
		UTMTerm: l.UTMTerm, UTMContent: l.UTMContent, FBCLID: l.FBCLID, GCLID: l.GCLID,
		LandingURL: l.LandingURL, Referrer: l.Referrer, EventID: l.EventID,
		AlertStatus: string(l.AlertStatus), AlertAttempts: l.AlertAttempts,
		AlertNextAttemptAt: l.AlertNextAttemptAt, AlertLastError: l.AlertLastError,
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

// LeadRepository is platform data like OrganizationRepository: the plain connection, no tenant filter.
type LeadRepository struct {
	db *gorm.DB
}

func NewLeadRepository(db *gorm.DB) ports.LeadRepository {
	return &LeadRepository{db: db}
}

func (r *LeadRepository) conn(ctx context.Context) *gorm.DB {
	return postgres.GetTxOrDB(ctx, r.db)
}

func (r *LeadRepository) Create(ctx context.Context, lead *dto.Lead) error {
	if err := r.conn(ctx).Create(leadToModel(lead)).Error; err != nil {
		return fmt.Errorf("create lead: %w", err)
	}
	return nil
}

func (r *LeadRepository) Get(ctx context.Context, id string) (*dto.Lead, error) {
	var model leadModel
	err := r.conn(ctx).Where("id = ?", id).Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrLeadNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}
	return model.toDTO(), nil
}

func (r *LeadRepository) List(ctx context.Context, filter dto.ListLeadsFilter) ([]*dto.Lead, int64, error) {
	query := r.conn(ctx).Model(&leadModel{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count leads: %w", err)
	}
	var models []leadModel
	err := query.Order("created_at DESC, id DESC").
		Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list leads: %w", err)
	}
	leads := make([]*dto.Lead, len(models))
	for i := range models {
		leads[i] = models[i].toDTO()
	}
	return leads, total, nil
}

func (r *LeadRepository) Update(ctx context.Context, lead *dto.Lead) error {
	result := r.conn(ctx).Model(&leadModel{}).Where("id = ?", lead.ID).
		Updates(map[string]any{"status": string(lead.Status), "notes": lead.Notes, "updated_at": lead.UpdatedAt})
	if result.Error != nil {
		return fmt.Errorf("update lead: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrLeadNotFound
	}
	return nil
}

func (r *LeadRepository) Delete(ctx context.Context, id string) error {
	result := r.conn(ctx).Where("id = ?", id).Delete(&leadModel{})
	if result.Error != nil {
		return fmt.Errorf("delete lead: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrLeadNotFound
	}
	return nil
}

func (r *LeadRepository) FindRecentByPhone(ctx context.Context, phone string, since time.Time) (*dto.Lead, error) {
	var model leadModel
	err := r.conn(ctx).Where("phone = ? AND created_at >= ?", phone, since).Order("created_at DESC").Limit(1).Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find recent lead by phone: %w", err)
	}
	return model.toDTO(), nil
}

func (r *LeadRepository) CountSince(ctx context.Context, since time.Time) (int, error) {
	var count int64
	if err := r.conn(ctx).Model(&leadModel{}).Where("created_at >= ?", since).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count leads since: %w", err)
	}
	return int(count), nil
}

func (r *LeadRepository) ClaimDueAlerts(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.Lead, error) {
	var models []leadModel
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("alert_status = ? AND alert_next_attempt_at <= ?", string(dto.LeadAlertPending), now).
			Order("alert_next_attempt_at ASC, id ASC").
			Limit(limit).
			Find(&models).Error; err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}
		ids := make([]string, len(models))
		for i := range models {
			ids[i] = models[i].ID
			models[i].AlertAttempts++
			models[i].AlertNextAttemptAt = &leaseUntil
		}
		return tx.Model(&leadModel{}).Where("id IN ?", ids).Updates(map[string]any{
			"alert_attempts":        gorm.Expr("alert_attempts + 1"),
			"alert_next_attempt_at": leaseUntil,
		}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim due lead alerts: %w", err)
	}
	leads := make([]*dto.Lead, len(models))
	for i := range models {
		leads[i] = models[i].toDTO()
	}
	return leads, nil
}

func (r *LeadRepository) MarkAlert(ctx context.Context, id string, status dto.LeadAlertStatus, nextAttemptAt time.Time, lastError string) error {
	updates := map[string]any{"alert_status": string(status), "alert_last_error": lastError}
	if status == dto.LeadAlertPending {
		updates["alert_next_attempt_at"] = nextAttemptAt
	}
	if err := r.conn(ctx).Model(&leadModel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("mark lead alert: %w", err)
	}
	return nil
}
