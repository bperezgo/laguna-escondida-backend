package repository_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSyncTestDB connects to the local Postgres the migration was applied to.
// Skips unless RUN_INTEGRATION_TESTS=true, mirroring the storage integration tests.
func newSyncTestDB(t *testing.T) *repository.Database {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}
	loadEnvFromProjectRootForSync()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getenvOr("DB_HOST", "localhost"),
		getenvOr("DB_PORT", "5432"),
		getenvOr("DB_USER", "postgres"),
		getenvOr("DB_PASSWORD", "postgres"),
		getenvOr("DB_NAME", "laguna_escondida"),
		getenvOr("DB_SSLMODE", "disable"),
	)

	db, err := repository.NewDatabase(dsn)
	require.NoError(t, err, "connect to test database")
	return db
}

func getenvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvFromProjectRootForSync() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		envPath := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(envPath); statErr == nil {
			_ = godotenv.Load(envPath)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func TestSyncOutboxRepository_Append_WritesRow_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	entityID := uuid.NewString()
	opID := uuid.Must(uuid.NewV7()).String()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	entry := &dto.SyncOutboxEntry{
		OpID:         opID,
		OriginNodeID: origin,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     entityID,
		Operation:    dto.SyncOperationCreate,
		Payload:      json.RawMessage(`{"id":"` + entityID + `","status":"open"}`),
	}

	err := uow.Do(context.Background(), func(ctx context.Context) error {
		return repo.Append(ctx, entry)
	})
	require.NoError(t, err)

	// OpID is caller-provided; the repository populates the DB-owned fields.
	assert.Equal(t, opID, entry.OpID)
	assert.Equal(t, int64(1), entry.Seq)
	assert.False(t, entry.CreatedAt.IsZero())

	var count int64
	db.DB.Raw("SELECT COUNT(*) FROM sync_outbox WHERE op_id = ?", entry.OpID).Scan(&count)
	assert.Equal(t, int64(1), count, "exactly one row should be committed")

	var operation, entityType string
	db.DB.Raw("SELECT operation, entity_type FROM sync_outbox WHERE op_id = ?", entry.OpID).Row().Scan(&operation, &entityType)
	assert.Equal(t, "create", operation)
	assert.Equal(t, "open_bill", entityType)
}

func TestSyncOutboxRepository_Append_RollbackLeavesNone_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	boom := fmt.Errorf("business change failed after append")
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		if appendErr := repo.Append(ctx, &dto.SyncOutboxEntry{
			OpID:         uuid.Must(uuid.NewV7()).String(),
			OriginNodeID: origin,
			EntityType:   dto.SyncEntityOpenBill,
			EntityID:     uuid.NewString(),
			Operation:    dto.SyncOperationCreate,
			Payload:      json.RawMessage(`{}`),
		}); appendErr != nil {
			return appendErr
		}
		return boom // force the transaction to roll back
	})
	require.ErrorIs(t, err, boom)

	var count int64
	db.DB.Raw("SELECT COUNT(*) FROM sync_outbox WHERE origin_node_id = ?", origin).Scan(&count)
	assert.Equal(t, int64(0), count, "rolled-back append must leave no row")
}

func TestSyncOutboxRepository_Append_SeqIncrementsPerOrigin_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	first := &dto.SyncOutboxEntry{OpID: uuid.Must(uuid.NewV7()).String(), OriginNodeID: origin, EntityType: dto.SyncEntityOpenBill, EntityID: uuid.NewString(), Operation: dto.SyncOperationCreate, Payload: json.RawMessage(`{}`)}
	second := &dto.SyncOutboxEntry{OpID: uuid.Must(uuid.NewV7()).String(), OriginNodeID: origin, EntityType: dto.SyncEntityOpenBill, EntityID: uuid.NewString(), Operation: dto.SyncOperationUpdate, Payload: json.RawMessage(`{}`)}

	err := uow.Do(context.Background(), func(ctx context.Context) error {
		if appendErr := repo.Append(ctx, first); appendErr != nil {
			return appendErr
		}
		return repo.Append(ctx, second)
	})
	require.NoError(t, err)

	assert.Equal(t, int64(1), first.Seq)
	assert.Equal(t, int64(2), second.Seq, "seq must be monotonic per origin")
}

// TestSyncOutboxRepository_ListUnsynced_And_MarkSynced exercises the edge push read
// path: ListUnsynced returns this origin's rows in seq order, and MarkSynced removes
// the acked ones from the next ListUnsynced.
func TestSyncOutboxRepository_ListUnsynced_And_MarkSynced_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	first := &dto.SyncOutboxEntry{OpID: uuid.Must(uuid.NewV7()).String(), OriginNodeID: origin, EntityType: dto.SyncEntityOpenBill, EntityID: uuid.NewString(), Operation: dto.SyncOperationCreate, Payload: json.RawMessage(`{"a":1}`)}
	second := &dto.SyncOutboxEntry{OpID: uuid.Must(uuid.NewV7()).String(), OriginNodeID: origin, EntityType: dto.SyncEntityOpenBill, EntityID: uuid.NewString(), Operation: dto.SyncOperationUpdate, Payload: json.RawMessage(`{"a":2}`)}
	require.NoError(t, uow.Do(context.Background(), func(ctx context.Context) error {
		if err := repo.Append(ctx, first); err != nil {
			return err
		}
		return repo.Append(ctx, second)
	}))

	ctx := context.Background()
	unsynced, err := repo.ListUnsynced(ctx, origin, 10)
	require.NoError(t, err)
	require.Len(t, unsynced, 2)
	assert.Equal(t, first.OpID, unsynced[0].OpID, "rows come back in seq order")
	assert.Equal(t, int64(1), unsynced[0].Seq)
	assert.JSONEq(t, `{"a":1}`, string(unsynced[0].Payload), "payload round-trips")

	// Limit caps the batch.
	limited, err := repo.ListUnsynced(ctx, origin, 1)
	require.NoError(t, err)
	assert.Len(t, limited, 1)

	// Acking the first drops it from the next ListUnsynced.
	require.NoError(t, repo.MarkSynced(ctx, []string{first.OpID}))
	remaining, err := repo.ListUnsynced(ctx, origin, 10)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, second.OpID, remaining[0].OpID, "only the unacked row remains")
}

// TestSyncStateRepository_AdvancePushedSeq upserts the peer high-water mark and never
// moves it backwards.
func TestSyncStateRepository_AdvancePushedSeq_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncStateRepository(db.DB)

	peer := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_state WHERE peer_node_id = ?", peer) })

	ctx := context.Background()
	require.NoError(t, repo.AdvancePushedSeq(ctx, peer, 5))

	var seq int64
	db.DB.Raw("SELECT last_pushed_seq FROM sync_state WHERE peer_node_id = ?", peer).Scan(&seq)
	assert.Equal(t, int64(5), seq)

	// Advancing forward raises the mark.
	require.NoError(t, repo.AdvancePushedSeq(ctx, peer, 9))
	db.DB.Raw("SELECT last_pushed_seq FROM sync_state WHERE peer_node_id = ?", peer).Scan(&seq)
	assert.Equal(t, int64(9), seq)

	// A stale, lower ack must not move it backwards (GREATEST).
	require.NoError(t, repo.AdvancePushedSeq(ctx, peer, 3))
	db.DB.Raw("SELECT last_pushed_seq FROM sync_state WHERE peer_node_id = ?", peer).Scan(&seq)
	assert.Equal(t, int64(9), seq, "high-water mark never decreases")
}

// TestSyncStateRepository_PulledCursor covers the pull bookmark: nil before any pull,
// set on first advance, and monotonic (a stale, earlier cursor never moves it back).
func TestSyncStateRepository_PulledCursor_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncStateRepository(db.DB)

	peer := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_state WHERE peer_node_id = ?", peer) })

	ctx := context.Background()
	cursor, err := repo.GetPulledCursor(ctx, peer)
	require.NoError(t, err)
	assert.Nil(t, cursor, "no cursor before the first pull")

	t1 := time.Now().Truncate(time.Millisecond)
	require.NoError(t, repo.AdvancePulledCursor(ctx, peer, t1))
	got, err := repo.GetPulledCursor(ctx, peer)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.WithinDuration(t, t1, *got, time.Millisecond)

	// Forward advance moves it; a stale earlier cursor does not.
	t2 := t1.Add(time.Hour)
	require.NoError(t, repo.AdvancePulledCursor(ctx, peer, t2))
	require.NoError(t, repo.AdvancePulledCursor(ctx, peer, t1.Add(-time.Hour)))
	got, err = repo.GetPulledCursor(ctx, peer)
	require.NoError(t, err)
	assert.WithinDuration(t, t2, *got, time.Millisecond, "cursor never moves backwards")
}
