package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type idempotencyKeyModel struct {
	postgres.TenantOwned
	Scope       string    `gorm:"column:scope;primaryKey"`
	Key         string    `gorm:"column:key;primaryKey"`
	UserID      string    `gorm:"column:user_id"`
	RequestHash string    `gorm:"column:request_hash"`
	Response    *string   `gorm:"column:response;type:jsonb"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (idempotencyKeyModel) TableName() string {
	return "idempotency_keys"
}

type IdempotencyKeyRepository struct {
	db *gorm.DB
}

func NewIdempotencyKeyRepository(db *gorm.DB) ports.IdempotencyKeyRepository {
	return &IdempotencyKeyRepository{db: db}
}

func (r *IdempotencyKeyRepository) Claim(ctx context.Context, key *dto.IdempotencyKey) (bool, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return false, err
	}
	res := postgres.GetTxOrDB(ctx, r.db).Exec(
		`INSERT INTO idempotency_keys (organization_id, scope, key, user_id, request_hash) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (scope, key) DO NOTHING`,
		orgID, key.Scope, key.Key, key.UserID, key.RequestHash,
	)
	if res.Error != nil {
		return false, fmt.Errorf("insert idempotency key: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *IdempotencyKeyRepository) Get(ctx context.Context, scope, key string) (*dto.IdempotencyKey, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model idempotencyKeyModel
	err = db.Where("scope = ? AND key = ?", scope, key).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find idempotency key: %w", err)
	}

	entry := &dto.IdempotencyKey{
		Scope:       model.Scope,
		Key:         model.Key,
		UserID:      model.UserID,
		RequestHash: model.RequestHash,
		CreatedAt:   model.CreatedAt,
	}
	if model.Response != nil {
		entry.Response = json.RawMessage(*model.Response)
	}
	return entry, nil
}

func (r *IdempotencyKeyRepository) Complete(ctx context.Context, scope, key string, response []byte) error {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return err
	}
	if err := postgres.GetTxOrDB(ctx, r.db).Exec(
		"UPDATE idempotency_keys SET response = ?, completed_at = NOW() WHERE organization_id = ? AND scope = ? AND key = ?",
		string(response), orgID, scope, key,
	).Error; err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}

func (r *IdempotencyKeyRepository) Release(ctx context.Context, scope, key string) error {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return err
	}
	if err := postgres.GetTxOrDB(ctx, r.db).Exec(
		"DELETE FROM idempotency_keys WHERE organization_id = ? AND scope = ? AND key = ? AND completed_at IS NULL", orgID, scope, key,
	).Error; err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}

func (r *IdempotencyKeyRepository) ReleaseStale(ctx context.Context, scope, key string, olderThan time.Duration) (bool, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return false, err
	}
	// Age is measured on the database clock, the one that stamped created_at.
	res := postgres.GetTxOrDB(ctx, r.db).Exec(
		`DELETE FROM idempotency_keys
		 WHERE organization_id = ? AND scope = ? AND key = ? AND completed_at IS NULL AND created_at < NOW() - make_interval(secs => ?)`,
		orgID, scope, key, olderThan.Seconds(),
	)
	if res.Error != nil {
		return false, fmt.Errorf("release stale idempotency key: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}
