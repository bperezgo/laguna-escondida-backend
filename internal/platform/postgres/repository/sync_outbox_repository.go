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

type syncOutboxModel struct {
	OpID         string     `gorm:"column:op_id;primaryKey"`
	OriginNodeID string     `gorm:"column:origin_node_id"`
	EntityType   string     `gorm:"column:entity_type"`
	EntityID     string     `gorm:"column:entity_id"`
	Operation    string     `gorm:"column:operation"`
	Payload      string     `gorm:"column:payload;type:jsonb"`
	Seq          int64      `gorm:"column:seq"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	SyncedAt     *time.Time `gorm:"column:synced_at"`
}

func (syncOutboxModel) TableName() string {
	return "sync_outbox"
}

type SyncOutboxRepository struct {
	db *gorm.DB
}

func NewSyncOutboxRepository(db *gorm.DB) ports.SyncOutboxRepository {
	return &SyncOutboxRepository{db: db}
}

// Append persists one outbox entry, joining the caller's transaction via GetTxOrDB.
// It must run inside a UnitOfWork transaction (Option A): the per-origin advisory
// lock below is transaction-scoped, so it only serializes seq assignment when an
// ambient transaction is present, and the outbox row commits with the business change.
//
// OpID is required: the calling service supplies the client-generated UUID v7
// idempotency key. Identity is a domain concern, not the adapter's.
func (r *SyncOutboxRepository) Append(ctx context.Context, entry *dto.SyncOutboxEntry) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	if entry.OpID == "" {
		return fmt.Errorf("sync outbox entry requires a caller-provided op_id")
	}

	// Serialize seq assignment for this origin so two concurrent appends can't read
	// the same MAX(seq) and collide on UNIQUE(origin_node_id, seq). The lock auto-
	// releases at commit/rollback.
	if err := db.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", entry.OriginNodeID).Error; err != nil {
		return fmt.Errorf("lock outbox origin: %w", err)
	}

	var maxSeq int64
	if err := db.Raw(
		"SELECT COALESCE(MAX(seq), 0) FROM sync_outbox WHERE origin_node_id = ?",
		entry.OriginNodeID,
	).Scan(&maxSeq).Error; err != nil {
		return fmt.Errorf("read outbox max seq: %w", err)
	}
	entry.Seq = maxSeq + 1

	model := &syncOutboxModel{
		OpID:         entry.OpID,
		OriginNodeID: entry.OriginNodeID,
		EntityType:   string(entry.EntityType),
		EntityID:     entry.EntityID,
		Operation:    string(entry.Operation),
		Payload:      string(entry.Payload),
		Seq:          entry.Seq,
	}

	if err := db.Create(model).Error; err != nil {
		return fmt.Errorf("insert outbox row: %w", err)
	}

	entry.CreatedAt = model.CreatedAt
	return nil
}

// ListUnsynced returns this origin's unacknowledged rows in seq order, capped at limit.
func (r *SyncOutboxRepository) ListUnsynced(ctx context.Context, originNodeID string, limit int) ([]*dto.SyncOutboxEntry, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []syncOutboxModel
	if err := db.
		Where("origin_node_id = ? AND synced_at IS NULL", originNodeID).
		Order("seq ASC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query unsynced outbox: %w", err)
	}

	entries := make([]*dto.SyncOutboxEntry, len(models))
	for i := range models {
		entries[i] = toSyncOutboxEntry(&models[i])
	}
	return entries, nil
}

// MarkSynced stamps synced_at on the acked op_ids so they drop out of ListUnsynced.
func (r *SyncOutboxRepository) MarkSynced(ctx context.Context, opIDs []string) error {
	if len(opIDs) == 0 {
		return nil
	}
	db := postgres.GetTxOrDB(ctx, r.db)

	if err := db.Model(&syncOutboxModel{}).
		Where("op_id IN ?", opIDs).
		Update("synced_at", time.Now()).Error; err != nil {
		return fmt.Errorf("mark outbox synced: %w", err)
	}
	return nil
}

// PendingStats returns this origin's unsynced row count and the oldest unsynced
// created_at (nil when none) in a single aggregate query.
func (r *SyncOutboxRepository) PendingStats(ctx context.Context, originNodeID string) (*dto.SyncOutboxPendingStats, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var row struct {
		PendingCount int64
		OldestAt     *time.Time
	}
	if err := db.Model(&syncOutboxModel{}).
		Select("COUNT(*) AS pending_count, MIN(created_at) AS oldest_at").
		Where("origin_node_id = ? AND synced_at IS NULL", originNodeID).
		Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("query outbox pending stats: %w", err)
	}

	return &dto.SyncOutboxPendingStats{
		PendingCount:    int(row.PendingCount),
		OldestPendingAt: row.OldestAt,
	}, nil
}

// HasUnsyncedFromOtherOrigins returns true when any unsynced row in the outbox
// belongs to a node other than myOriginNodeID. A true result with an empty
// ListUnsynced means the current node ID doesn't match what was written —
// typically caused by APP_MODE or ORGANIZATION_ID changing between runs.
func (r *SyncOutboxRepository) HasUnsyncedFromOtherOrigins(ctx context.Context, myOriginNodeID string) (bool, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var exists bool
	if err := db.Raw(
		"SELECT EXISTS(SELECT 1 FROM sync_outbox WHERE origin_node_id != ? AND synced_at IS NULL)",
		myOriginNodeID,
	).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check unsynced from other origins: %w", err)
	}
	return exists, nil
}

func toSyncOutboxEntry(m *syncOutboxModel) *dto.SyncOutboxEntry {
	return &dto.SyncOutboxEntry{
		OpID:         m.OpID,
		OriginNodeID: m.OriginNodeID,
		EntityType:   dto.SyncEntityType(m.EntityType),
		EntityID:     m.EntityID,
		Operation:    dto.SyncOperation(m.Operation),
		Payload:      json.RawMessage(m.Payload),
		Seq:          m.Seq,
		CreatedAt:    m.CreatedAt,
		SyncedAt:     m.SyncedAt,
	}
}
