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

// pendingInvoiceModel maps the pending_invoices local job queue. It is enqueued by
// BillRepository.Create (same package, same transaction as the bill) and drained by the
// invoice submission service. It is never replicated between nodes.
type pendingInvoiceModel struct {
	ID             string     `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	BillID         string     `gorm:"column:bill_id"`
	Prefix         string     `gorm:"column:prefix"`
	Consecutive    int        `gorm:"column:consecutive"`
	RequestPayload string     `gorm:"column:request_payload;type:jsonb"`
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

// ListDue returns due pending submissions in consecutive order (lowest-first), so fiscal
// numbers are issued in sequence. A row is due when it has never been attempted
// (next_attempt_at IS NULL) or its backoff timer has elapsed.
func (r *PendingInvoiceRepository) ListDue(ctx context.Context, limit int) ([]*dto.PendingInvoice, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []pendingInvoiceModel
	if err := db.
		Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)",
			string(dto.PendingInvoiceStatusPending), time.Now()).
		Order("consecutive ASC").
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
	return &dto.PendingInvoice{
		ID:             m.ID,
		BillID:         m.BillID,
		Prefix:         m.Prefix,
		Consecutive:    m.Consecutive,
		RequestPayload: json.RawMessage(m.RequestPayload),
		Status:         dto.PendingInvoiceStatus(m.Status),
		Attempts:       m.Attempts,
		LastAttemptAt:  m.LastAttemptAt,
		NextAttemptAt:  m.NextAttemptAt,
		LastError:      m.LastError,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
