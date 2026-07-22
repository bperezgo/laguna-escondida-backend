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
)

// pendingInvoiceModel maps the pending_invoices queue. Rows are created by
// BillRepository.Create and drained by the cloud submission service.
// Consecutive and RequestPayload are NULL until the cloud cron assigns them
// right before the first submission attempt.
type pendingInvoiceModel struct {
	ID             string     `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	BillID         string     `gorm:"column:bill_id"`
	PaymentCode    string     `gorm:"column:payment_code"`
	Prefix         string     `gorm:"column:prefix"`
	Consecutive    *int       `gorm:"column:consecutive"`
	RequestPayload *string    `gorm:"column:request_payload;type:jsonb"`
	Status         string     `gorm:"column:status"`
	Attempts       int        `gorm:"column:attempts"`
	LastAttemptAt  *time.Time `gorm:"column:last_attempt_at"`
	NextAttemptAt  *time.Time `gorm:"column:next_attempt_at"`
	LastError      *string    `gorm:"column:last_error"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (pendingInvoiceModel) TableName() string {
	return "pending_invoices"
}

type PendingInvoiceRepository struct {
	db *gorm.DB
}

func NewPendingInvoiceRepository(db *gorm.DB) ports.PendingInvoiceRepository {
	return &PendingInvoiceRepository{db: db}
}

// Create persists a new pending invoice row exactly as the caller constructed it.
func (r *PendingInvoiceRepository) Create(ctx context.Context, p *dto.PendingInvoice) error {
	db := postgres.GetTxOrDB(ctx, r.db)
	m := &pendingInvoiceModel{
		ID:          p.ID,
		BillID:      p.BillID,
		PaymentCode: string(p.PaymentCode),
		Status:      string(p.Status),
	}
	return db.Create(m).Error
}

// ListDue returns due pending submissions ordered by consecutive ascending (NULLS FIRST),
// so unassigned rows are processed before already-assigned ones, and submitted numbers are
// issued in sequence. A row is due when it has never been attempted (next_attempt_at IS NULL)
// or its backoff timer has elapsed.
func (r *PendingInvoiceRepository) ListDue(ctx context.Context, limit int) ([]*dto.PendingInvoice, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []pendingInvoiceModel
	if err := db.
		Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)",
			string(dto.PendingInvoiceStatusPending), time.Now()).
		Order("consecutive ASC NULLS FIRST").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query due pending invoices: %w", err)
	}

	pendings := make([]*dto.PendingInvoice, len(models))
	for i := range models {
		pendings[i] = toPendingInvoice(&models[i])
	}
	return pendings, nil
}

// AssignConsecutive stores the cloud-assigned consecutive and the built request payload.
// The WHERE consecutive IS NULL guard makes it idempotent — a second concurrent call is
// a no-op so concurrent cron ticks cannot double-assign.
func (r *PendingInvoiceRepository) AssignConsecutive(ctx context.Context, id string, consecutive int, requestPayload json.RawMessage) error {
	db := postgres.GetTxOrDB(ctx, r.db)
	payload := string(requestPayload)
	if err := db.Model(&pendingInvoiceModel{}).
		Where("id = ? AND consecutive IS NULL", id).
		Updates(map[string]any{
			"consecutive":     consecutive,
			"request_payload": payload,
			"updated_at":      time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("assign consecutive to pending invoice: %w", err)
	}
	return nil
}

// UpdateRequestPayload overwrites request_payload for a row while leaving its consecutive
// untouched. Used to rebuild a stale/broken payload (e.g. after an operator sets
// request_payload = NULL) so the invoice re-submits with the SAME fiscal number — no gap.
func (r *PendingInvoiceRepository) UpdateRequestPayload(ctx context.Context, id string, requestPayload json.RawMessage) error {
	db := postgres.GetTxOrDB(ctx, r.db)
	payload := string(requestPayload)
	if err := db.Model(&pendingInvoiceModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"request_payload": payload,
			"updated_at":      time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("update pending invoice request payload: %w", err)
	}
	return nil
}

// MarkSubmitted flips a row to submitted after the provider accepts it.
func (r *PendingInvoiceRepository) MarkSubmitted(ctx context.Context, id string) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	now := time.Now()
	if err := db.Model(&pendingInvoiceModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          string(dto.PendingInvoiceStatusSubmitted),
			"last_attempt_at": now,
			"updated_at":      now,
		}).Error; err != nil {
		return fmt.Errorf("mark pending invoice submitted: %w", err)
	}
	return nil
}

// MarkFailed records a failed attempt: it bumps the attempt counter, stores the error, and
// sets the backoff timer. The row stays pending (never auto-fails on a transient outage), so
// the submitter keeps retrying until the provider answers.
func (r *PendingInvoiceRepository) MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	now := time.Now()
	if err := db.Model(&pendingInvoiceModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":        gorm.Expr("attempts + 1"),
			"last_attempt_at": now,
			"next_attempt_at": nextAttemptAt,
			"last_error":      errMsg,
			"updated_at":      now,
		}).Error; err != nil {
		return fmt.Errorf("mark pending invoice failed: %w", err)
	}
	return nil
}

func toPendingInvoice(m *pendingInvoiceModel) *dto.PendingInvoice {
	var payload json.RawMessage
	if m.RequestPayload != nil {
		payload = json.RawMessage(*m.RequestPayload)
	}
	return &dto.PendingInvoice{
		ID:             m.ID,
		BillID:         m.BillID,
		PaymentCode:    dto.ElectronicInvoicePaymentCode(m.PaymentCode),
		Prefix:         m.Prefix,
		Consecutive:    m.Consecutive,
		RequestPayload: payload,
		Status:         dto.PendingInvoiceStatus(m.Status),
		Attempts:       m.Attempts,
		LastAttemptAt:  m.LastAttemptAt,
		NextAttemptAt:  m.NextAttemptAt,
		LastError:      m.LastError,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
