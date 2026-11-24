package repository

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type BoxSecretRepository struct {
	db *gorm.DB
}

func NewBoxSecretRepository(db *gorm.DB) ports.BoxSecretRepository {
	return &BoxSecretRepository{db: db}
}

func (r *BoxSecretRepository) GetOrCreate(ctx context.Context, name, candidate string) (string, error) {
	db := r.db.WithContext(ctx)
	if err := db.Exec(
		"INSERT INTO box_secrets (name, value) VALUES (?, ?) ON CONFLICT (name) DO NOTHING",
		name, candidate,
	).Error; err != nil {
		return "", fmt.Errorf("store box secret %q: %w", name, err)
	}

	var value string
	if err := db.Raw("SELECT value FROM box_secrets WHERE name = ?", name).Scan(&value).Error; err != nil {
		return "", fmt.Errorf("read box secret %q: %w", name, err)
	}
	return value, nil
}
