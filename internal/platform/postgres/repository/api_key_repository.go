package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type APIKeyRepository struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) ports.APIKeyRepository {
	return &APIKeyRepository{db: db}
}

type apiKeyModel struct {
	postgres.TenantOwned
	ID         string     `gorm:"type:uuid;primaryKey"`
	UserID     string     `gorm:"type:uuid;not null"`
	Name       string     `gorm:"type:varchar(255);not null"`
	Prefix     string     `gorm:"type:varchar(16);not null"`
	Hash       string     `gorm:"type:char(64);not null"`
	CreatedAt  time.Time  `gorm:"type:timestamptz;not null"`
	LastUsedAt *time.Time `gorm:"type:timestamptz"`
	RevokedAt  *time.Time `gorm:"type:timestamptz"`
}

func (apiKeyModel) TableName() string {
	return "api_keys"
}

func (r *APIKeyRepository) Create(ctx context.Context, key *dto.APIKey, hash string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	model := &apiKeyModel{
		ID:        key.ID,
		UserID:    key.UserID,
		Name:      key.Name,
		Prefix:    key.Prefix,
		Hash:      hash,
		CreatedAt: key.CreatedAt,
	}
	if err := db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	key.OrganizationID = model.OrganizationID
	return nil
}

func (r *APIKeyRepository) List(ctx context.Context) ([]*dto.APIKey, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var models []apiKeyModel
	if err := db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	keys := make([]*dto.APIKey, len(models))
	for i := range models {
		keys[i] = toAPIKeyDTO(&models[i])
	}
	return keys, nil
}

func (r *APIKeyRepository) Revoke(ctx context.Context, id string, at time.Time) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).
		Model(&apiKeyModel{}).
		Where("id = ?", id).
		Update("revoked_at", gorm.Expr("COALESCE(revoked_at, ?)", at))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainError.ErrAPIKeyNotFound
	}
	return nil
}

type apiKeyRecordRow struct {
	KeyID              string
	KeyOrganizationID  string
	RevokedAt          *time.Time
	UserID             string
	Username           string
	UserOrganizationID string
	UserActive         bool
	UserDeleted        bool
	OrganizationStatus string
}

// FindByHash runs without the org scope on purpose: the key is what names the organization.
// When RLS lands, api_keys needs the same exemption as the email sign-in lookup.
func (r *APIKeyRepository) FindByHash(ctx context.Context, hash string) (*dto.APIKeyRecord, error) {
	db := postgres.GetTxOrDB(ctx, r.db).WithContext(ctx)
	var row apiKeyRecordRow
	result := db.Raw(`
		SELECT k.id AS key_id, k.organization_id AS key_organization_id, k.revoked_at,
		       u.id AS user_id, u.username, u.organization_id AS user_organization_id,
		       u.active AS user_active, u.deleted_at IS NOT NULL AS user_deleted,
		       o.status AS organization_status
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		JOIN organizations o ON o.id = k.organization_id
		WHERE k.hash = ?`, hash).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, domainError.ErrAPIKeyNotFound
	}
	var roleIDs []int
	if err := db.Raw(`SELECT role_id FROM user_roles WHERE user_id = ? AND organization_id = ? ORDER BY role_id`,
		row.UserID, row.UserOrganizationID).Scan(&roleIDs).Error; err != nil {
		return nil, err
	}
	return &dto.APIKeyRecord{
		KeyID:              row.KeyID,
		KeyOrganizationID:  row.KeyOrganizationID,
		RevokedAt:          row.RevokedAt,
		UserID:             row.UserID,
		Username:           row.Username,
		UserOrganizationID: row.UserOrganizationID,
		UserActive:         row.UserActive,
		UserDeleted:        row.UserDeleted,
		OrganizationStatus: row.OrganizationStatus,
		RoleIDs:            roleIDs,
	}, nil
}

func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).
		Model(&apiKeyModel{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", id, at).
		Update("last_used_at", at).Error
}

func toAPIKeyDTO(m *apiKeyModel) *dto.APIKey {
	return &dto.APIKey{
		ID:             m.ID,
		OrganizationID: m.OrganizationID,
		UserID:         m.UserID,
		Name:           m.Name,
		Prefix:         m.Prefix,
		CreatedAt:      m.CreatedAt,
		LastUsedAt:     m.LastUsedAt,
		RevokedAt:      m.RevokedAt,
	}
}
