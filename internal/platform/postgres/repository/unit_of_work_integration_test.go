package repository_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func outboxEntry(origin string) *dto.SyncOutboxEntry {
	return &dto.SyncOutboxEntry{
		OpID:         uuid.Must(uuid.NewV7()).String(),
		OriginNodeID: origin,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     uuid.NewString(),
		Operation:    dto.SyncOperationCreate,
		Payload:      json.RawMessage(`{}`),
	}
}

func countOutbox(t *testing.T, db *repository.Database, origin string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.DB.Raw("SELECT COUNT(*) FROM sync_outbox WHERE origin_node_id = ?", origin).Scan(&count).Error)
	return count
}

func TestUnitOfWork_Do_NestedJoinsOuterTransaction_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	var outerTxID, innerTxID int64
	boom := fmt.Errorf("outer fails after inner succeeded")
	err := uow.Do(lagunaCtx(), func(ctx context.Context) error {
		require.NoError(t, postgres.GetTxOrDB(ctx, db.DB).Raw("SELECT txid_current()").Scan(&outerTxID).Error)
		if innerErr := uow.Do(ctx, func(ctx context.Context) error {
			require.NoError(t, postgres.GetTxOrDB(ctx, db.DB).Raw("SELECT txid_current()").Scan(&innerTxID).Error)
			return repo.Append(ctx, outboxEntry(origin))
		}); innerErr != nil {
			return innerErr
		}
		return boom
	})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, outerTxID, innerTxID)
	assert.Equal(t, int64(0), countOutbox(t, db, origin), "inner write must roll back with the outer transaction")
}

func TestUnitOfWork_Do_NestedFailureRollsBackOnlyInner_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	boom := fmt.Errorf("inner fails")
	require.NoError(t, uow.Do(lagunaCtx(), func(ctx context.Context) error {
		if err := repo.Append(ctx, outboxEntry(origin)); err != nil {
			return err
		}
		innerErr := uow.Do(ctx, func(ctx context.Context) error {
			if err := repo.Append(ctx, outboxEntry(origin)); err != nil {
				return err
			}
			return boom
		})
		require.ErrorIs(t, innerErr, boom)
		return nil
	}))

	assert.Equal(t, int64(1), countOutbox(t, db, origin), "only the inner savepoint rolls back")
}

func TestInTx_JoinsUnitOfWorkTransaction_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)
	uow := postgres.NewUnitOfWork(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	boom := fmt.Errorf("outer fails after InTx succeeded")
	err := uow.Do(lagunaCtx(), func(ctx context.Context) error {
		if txErr := postgres.InTx(ctx, db.DB, func(tx *gorm.DB) error {
			return repo.Append(ctx, outboxEntry(origin))
		}); txErr != nil {
			return txErr
		}
		return boom
	})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, int64(0), countOutbox(t, db, origin))
}

func TestUnitOfWork_Do_WithoutOuterTransaction_Commits_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncOutboxRepository(db.DB)

	origin := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_outbox WHERE origin_node_id = ?", origin) })

	require.NoError(t, postgres.NewUnitOfWork(db.DB).Do(lagunaCtx(), func(ctx context.Context) error {
		return repo.Append(ctx, outboxEntry(origin))
	}))
	assert.Equal(t, int64(1), countOutbox(t, db, origin))
}
