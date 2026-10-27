package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type organizationSettingsRow struct {
	Settings []byte `gorm:"column:settings"`
	Timezone string `gorm:"column:timezone"`
}

type OrganizationSettingsRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewOrganizationSettingsRepository(db *gorm.DB, logger *slog.Logger) ports.OrganizationSettingsRepository {
	return &OrganizationSettingsRepository{db: db, logger: logger}
}

func (r *OrganizationSettingsRepository) Get(ctx context.Context) (*dto.OrganizationSettingsView, error) {
	return r.get(ctx, nil, false)
}

// get reads through tx when given, locking the row when lock is set.
func (r *OrganizationSettingsRepository) get(ctx context.Context, tx *gorm.DB, lock bool) (*dto.OrganizationSettingsView, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return nil, err
	}
	db := tx
	if db == nil {
		db = postgres.GetTxOrDB(ctx, r.db)
	}
	query := db.Table("organizations").Select("settings, timezone").Where("id = ?", orgID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row organizationSettingsRow
	err = query.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrOrganizationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get organization settings: %w", err)
	}
	return &dto.OrganizationSettingsView{Ordering: r.decode(ctx, orgID, row.Settings).Ordering, Timezone: row.Timezone}, nil
}

// decode treats a hand-edited value that doesn't parse as unconfigured, so a typo opens ordering
// rather than stopping every order.
func (r *OrganizationSettingsRepository) decode(ctx context.Context, orgID string, raw []byte) dto.OrganizationSettings {
	var settings dto.OrganizationSettings
	if len(raw) == 0 {
		return settings
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		r.logger.ErrorContext(ctx, "organization settings are not valid JSON; reading them as unconfigured",
			"organization_id", orgID, "error", err)
		return dto.OrganizationSettings{}
	}
	return settings
}

// UpdateOrdering replaces only the ordering key, leaving any other settings in the bag untouched.
func (r *OrganizationSettingsRepository) UpdateOrdering(ctx context.Context, merge ports.OrderingMerge) (*dto.OrganizationSettingsView, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return nil, err
	}
	var saved *dto.OrganizationSettingsView
	txErr := postgres.InTx(ctx, r.db, func(tx *gorm.DB) error {
		var mergeErr error
		saved, mergeErr = r.mergeLocked(ctx, tx, orgID, merge)
		return mergeErr
	})
	if txErr != nil {
		return nil, txErr
	}
	return saved, nil
}

func (r *OrganizationSettingsRepository) mergeLocked(ctx context.Context, tx *gorm.DB, orgID string, merge ports.OrderingMerge) (*dto.OrganizationSettingsView, error) {
	current, err := r.get(ctx, tx, true)
	if err != nil {
		return nil, err
	}
	next, err := merge(current.Ordering)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, fmt.Errorf("encode ordering settings: %w", err)
	}
	err = tx.Exec(`
		UPDATE organizations
		SET settings = jsonb_set(
				CASE WHEN jsonb_typeof(settings) = 'object' THEN settings ELSE '{}'::jsonb END,
				'{ordering}', ?::jsonb),
			updated_at = NOW()
		WHERE id = ?`, string(raw), orgID).Error
	if err != nil {
		return nil, fmt.Errorf("update ordering settings: %w", err)
	}
	return &dto.OrganizationSettingsView{Ordering: next, Timezone: current.Timezone}, nil
}
