package repository

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type SyncStateRepository struct {
	db *gorm.DB
}

func NewSyncStateRepository(db *gorm.DB) ports.SyncStateRepository {
	return &SyncStateRepository{db: db}
}

// AdvancePushedSeq upserts the peer's sync_state row, raising last_pushed_seq to seq
// via GREATEST so an out-of-order or replayed ack never moves the mark backwards.
// Joins the ambient transaction via GetTxOrDB so it commits with the synced_at stamps.
func (r *SyncStateRepository) AdvancePushedSeq(ctx context.Context, peerNodeID string, seq int64) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	res := db.Exec(`
		INSERT INTO sync_state (peer_node_id, last_pushed_seq, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (peer_node_id) DO UPDATE
		SET last_pushed_seq = GREATEST(sync_state.last_pushed_seq, EXCLUDED.last_pushed_seq),
		    updated_at = CURRENT_TIMESTAMP
	`, peerNodeID, seq)
	if res.Error != nil {
		return fmt.Errorf("upsert sync_state pushed seq: %w", res.Error)
	}
	return nil
}

// GetPulledCursor returns the peer's last_pulled_cursor, or nil when there is no row
// yet (the edge has never pulled, so it should pull from the beginning of time).
func (r *SyncStateRepository) GetPulledCursor(ctx context.Context, peerNodeID string) (*time.Time, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var row struct {
		LastPulledCursor *time.Time
	}
	if err := db.Raw(
		"SELECT last_pulled_cursor FROM sync_state WHERE peer_node_id = ?",
		peerNodeID,
	).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("read sync_state pulled cursor: %w", err)
	}
	return row.LastPulledCursor, nil
}

// AdvancePulledCursor upserts the peer's sync_state row, raising last_pulled_cursor to
// cursor via GREATEST (which ignores a NULL existing value, so the first pull sets it)
// so a stale response never moves the cursor backwards.
func (r *SyncStateRepository) AdvancePulledCursor(ctx context.Context, peerNodeID string, cursor time.Time) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	res := db.Exec(`
		INSERT INTO sync_state (peer_node_id, last_pulled_cursor, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (peer_node_id) DO UPDATE
		SET last_pulled_cursor = GREATEST(sync_state.last_pulled_cursor, EXCLUDED.last_pulled_cursor),
		    updated_at = CURRENT_TIMESTAMP
	`, peerNodeID, cursor)
	if res.Error != nil {
		return fmt.Errorf("upsert sync_state pulled cursor: %w", res.Error)
	}
	return nil
}
