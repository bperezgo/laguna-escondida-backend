package repository

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrganizationAccessReader struct {
	db *gorm.DB
}

func NewOrganizationAccessReader(db *gorm.DB) ports.OrganizationAccessReader {
	return &OrganizationAccessReader{db: db}
}

func (r *OrganizationAccessReader) GetAccess(ctx context.Context, orgID string) (*dto.OrganizationAccess, error) {
	if uuid.Validate(orgID) != nil {
		return nil, domainError.ErrOrganizationNotFound
	}
	var row struct {
		Status      string
		Plan        string
		TrialEndsAt *time.Time
		PaidUntil   *time.Time
		CancelledAt *time.Time
		HasCard     bool
	}
	result := postgres.GetTxOrDB(ctx, r.db).WithContext(ctx).Raw(`
		SELECT o.status, o.plan, o.trial_ends_at, b.paid_until, b.cancelled_at,
		       b.payment_source_id IS NOT NULL AS has_card
		  FROM organizations o
		  LEFT JOIN organization_billing b ON b.organization_id = o.id
		 WHERE o.id = ?`, orgID).Scan(&row)
	if result.Error != nil {
		return nil, fmt.Errorf("read organization access: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, domainError.ErrOrganizationNotFound
	}
	return &dto.OrganizationAccess{Status: row.Status, Plan: row.Plan, TrialEndsAt: row.TrialEndsAt,
		PaidUntil: row.PaidUntil, CancelledAt: row.CancelledAt, HasCard: row.HasCard}, nil
}
