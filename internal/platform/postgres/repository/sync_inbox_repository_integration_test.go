package repository_test

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncInboxRepository_MarkApplied_Idempotent_Integration verifies the DB-level
// idempotency: the first MarkApplied records the op (false), the second sees the
// existing row (true). Reuses newSyncTestDB from the outbox integration test.
func TestSyncInboxRepository_MarkApplied_Idempotent_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewSyncInboxRepository(db.DB)

	opID := uuid.NewString()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM sync_inbox WHERE op_id = ?", opID) })

	first, err := repo.MarkApplied(context.Background(), opID)
	require.NoError(t, err)
	assert.False(t, first, "first MarkApplied should report a new op")

	second, err := repo.MarkApplied(context.Background(), opID)
	require.NoError(t, err)
	assert.True(t, second, "second MarkApplied should report already applied")
}
