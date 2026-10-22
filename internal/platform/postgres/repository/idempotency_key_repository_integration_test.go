package repository_test

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyKeyRepository_Lifecycle_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewIdempotencyKeyRepository(db.DB)
	ctx := context.Background()

	key := &dto.IdempotencyKey{Scope: "order.create", Key: uuid.NewString(), UserID: uuid.NewString(), RequestHash: "a1b2"}
	t.Cleanup(func() { db.DB.Exec("DELETE FROM idempotency_keys WHERE key = ?", key.Key) })

	claimed, err := repo.Claim(ctx, key)
	require.NoError(t, err)
	assert.True(t, claimed)

	claimed, err = repo.Claim(ctx, key)
	require.NoError(t, err)
	assert.False(t, claimed, "a second claim on the same key loses")

	pending, err := repo.Get(ctx, key.Scope, key.Key)
	require.NoError(t, err)
	require.NotNil(t, pending)
	assert.Equal(t, key.UserID, pending.UserID)
	assert.Nil(t, pending.Response)

	released, err := repo.ReleaseStale(ctx, key.Scope, key.Key, time.Hour)
	require.NoError(t, err)
	assert.False(t, released, "a fresh claim is not stale")

	require.NoError(t, repo.Complete(ctx, key.Scope, key.Key, []byte(`{"id":"order-1"}`)))
	done, err := repo.Get(ctx, key.Scope, key.Key)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"order-1"}`, string(done.Response))

	require.NoError(t, repo.Release(ctx, key.Scope, key.Key))
	kept, err := repo.Get(ctx, key.Scope, key.Key)
	require.NoError(t, err)
	assert.NotNil(t, kept, "a completed key is never released")
}

func TestIdempotencyKeyRepository_ReleaseStaleAndRelease_Integration(t *testing.T) {
	db := newSyncTestDB(t)
	repo := repository.NewIdempotencyKeyRepository(db.DB)
	ctx := context.Background()

	stale := &dto.IdempotencyKey{Scope: "order.create", Key: uuid.NewString(), UserID: uuid.NewString(), RequestHash: "a1b2"}
	fresh := &dto.IdempotencyKey{Scope: "order.create", Key: uuid.NewString(), UserID: uuid.NewString(), RequestHash: "a1b2"}
	t.Cleanup(func() { db.DB.Exec("DELETE FROM idempotency_keys WHERE key IN ?", []string{stale.Key, fresh.Key}) })

	for _, k := range []*dto.IdempotencyKey{stale, fresh} {
		claimed, err := repo.Claim(ctx, k)
		require.NoError(t, err)
		require.True(t, claimed)
	}
	require.NoError(t, db.DB.Exec("UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '10 minutes' WHERE key = ?", stale.Key).Error)

	released, err := repo.ReleaseStale(ctx, stale.Scope, stale.Key, 2*time.Minute)
	require.NoError(t, err)
	assert.True(t, released)
	missing, err := repo.Get(ctx, stale.Scope, stale.Key)
	require.NoError(t, err)
	assert.Nil(t, missing)

	require.NoError(t, repo.Release(ctx, fresh.Scope, fresh.Key))
	missing, err = repo.Get(ctx, fresh.Scope, fresh.Key)
	require.NoError(t, err)
	assert.Nil(t, missing, "an in-flight key is released after a failed create")
}
