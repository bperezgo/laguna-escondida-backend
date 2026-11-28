package repository

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

// boxOrganizationName is a placeholder: nothing on the box reads the name.
const boxOrganizationName = "Kuenta"

type BoxOrganizationRepository struct {
	db *gorm.DB
}

func NewBoxOrganizationRepository(db *gorm.DB) ports.BoxOrganizationRepository {
	return &BoxOrganizationRepository{db: db}
}

func (r *BoxOrganizationRepository) Ensure(ctx context.Context, organizationID string) error {
	// The slug follows the backfill in migration 000068 ('org-' || id), which fits its 40 characters.
	if err := r.db.WithContext(ctx).Exec(
		"INSERT INTO organizations (id, name, slug) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING",
		organizationID, boxOrganizationName, "org-"+organizationID,
	).Error; err != nil {
		return fmt.Errorf("ensure the box's organization row: %w", err)
	}
	return nil
}
