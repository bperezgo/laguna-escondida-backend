package repository_test

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func backendPID(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var pid int64
	require.NoError(t, db.Raw("SELECT pg_backend_pid()").Scan(&pid).Error)
	return pid
}

func orgSetting(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var org string
	require.NoError(t, db.Raw("SELECT COALESCE(current_setting('app.org_id', true), '')").Scan(&org).Error)
	return org
}

func TestSession_PinsOneConnectionWithOrg_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	orgID := uuid.NewString()

	ctx, release := postgres.WithSession(context.Background(), db.DB, orgID)
	defer release()

	first := backendPID(t, postgres.GetTxOrDB(ctx, db.DB))
	assert.Equal(t, first, backendPID(t, postgres.GetTxOrDB(ctx, db.DB)))
	assert.Equal(t, orgID, orgSetting(t, postgres.GetTxOrDB(ctx, db.DB)))

	require.NoError(t, postgres.NewUnitOfWork(db.DB).Do(ctx, func(ctx context.Context) error {
		tx := postgres.GetTxOrDB(ctx, db.DB)
		assert.Equal(t, first, backendPID(t, tx), "the transaction begins on the pinned connection")
		assert.Equal(t, orgID, orgSetting(t, tx))
		return nil
	}))
}

func TestSession_ReleaseResetsOrgAndFallsBackToPool_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	sqlDB, err := db.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	ctx, release := postgres.WithSession(context.Background(), db.DB, uuid.NewString())
	pinned := backendPID(t, postgres.GetTxOrDB(ctx, db.DB))
	release()

	after := postgres.GetTxOrDB(ctx, db.DB)
	assert.Equal(t, pinned, backendPID(t, after), "with one pooled connection the released one is reused")
	assert.Empty(t, orgSetting(t, after), "the org must not survive on a pooled connection")
}

func TestSession_DetachLeavesThePinnedConnection_Integration(t *testing.T) {
	db := newSyncTestDB(t)

	ctx, release := postgres.WithSession(context.Background(), db.DB, uuid.NewString())
	defer release()
	pinned := backendPID(t, postgres.GetTxOrDB(ctx, db.DB))

	detached := postgres.GetTxOrDB(postgres.Detach(ctx), db.DB)
	assert.NotEqual(t, pinned, backendPID(t, detached))
	assert.Empty(t, orgSetting(t, detached))
}
