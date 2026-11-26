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

type enrollmentCodeModel struct {
	ID             string     `gorm:"column:id;primaryKey"`
	OrganizationID string     `gorm:"column:organization_id"`
	CodeHash       string     `gorm:"column:code_hash"`
	CreatedBy      string     `gorm:"column:created_by"`
	ExpiresAt      time.Time  `gorm:"column:expires_at"`
	UsedAt         *time.Time `gorm:"column:used_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

func (enrollmentCodeModel) TableName() string { return "edge_enrollment_codes" }

type EnrollmentCodeRepository struct {
	db *gorm.DB
}

func NewEnrollmentCodeRepository(db *gorm.DB) ports.EnrollmentCodeRepository {
	return &EnrollmentCodeRepository{db: db}
}

func (r *EnrollmentCodeRepository) Create(ctx context.Context, code *dto.EnrollmentCode, codeHash string) error {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return err
	}
	model := &enrollmentCodeModel{ID: code.ID, OrganizationID: orgID, CodeHash: codeHash, CreatedBy: code.CreatedBy,
		ExpiresAt: code.ExpiresAt, CreatedAt: code.CreatedAt}
	if err := postgres.GetTxOrDB(ctx, r.db).Create(model).Error; err != nil {
		return fmt.Errorf("create enrollment code: %w", err)
	}
	code.OrganizationID = orgID
	return nil
}

func (r *EnrollmentCodeRepository) InvalidateUnused(ctx context.Context, at time.Time) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	if err := db.Model(&enrollmentCodeModel{}).Where("used_at IS NULL").Update("used_at", at).Error; err != nil {
		return fmt.Errorf("invalidate enrollment codes: %w", err)
	}
	return nil
}

func (r *EnrollmentCodeRepository) FindByHashForUpdate(ctx context.Context, codeHash string) (*dto.EnrollmentCode, error) {
	var model enrollmentCodeModel
	err := postgres.GetTxOrDB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code_hash = ?", codeHash).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrEnrollmentCodeInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("find enrollment code: %w", err)
	}
	return &dto.EnrollmentCode{ID: model.ID, OrganizationID: model.OrganizationID, CreatedBy: model.CreatedBy,
		ExpiresAt: model.ExpiresAt, UsedAt: model.UsedAt, CreatedAt: model.CreatedAt}, nil
}

func (r *EnrollmentCodeRepository) MarkUsed(ctx context.Context, id string, at time.Time) error {
	result := postgres.GetTxOrDB(ctx, r.db).Model(&enrollmentCodeModel{}).Where("id = ? AND used_at IS NULL", id).Update("used_at", at)
	if result.Error != nil {
		return fmt.Errorf("use enrollment code: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrEnrollmentCodeInvalid
	}
	return nil
}
