package repository

import (
	"context"

	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type SyncInboxRepository struct {
	db *gorm.DB
}

func NewSyncInboxRepository(db *gorm.DB) ports.SyncInboxRepository {
	return &SyncInboxRepository{db: db}
}

// MarkApplied inserts op_id, ignoring a duplicate. A zero RowsAffected means the
// row already existed, so the op was applied before. Joins the ambient transaction
// via GetTxOrDB so a rolled-back apply also removes the op_id.
func (r *SyncInboxRepository) MarkApplied(ctx context.Context, opID string) (bool, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return false, err
	}

	res := postgres.GetTxOrDB(ctx, r.db).Exec(
		"INSERT INTO sync_inbox (organization_id, op_id) VALUES (?, ?) ON CONFLICT (op_id) DO NOTHING", orgID, opID)
	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected == 0, nil
}
