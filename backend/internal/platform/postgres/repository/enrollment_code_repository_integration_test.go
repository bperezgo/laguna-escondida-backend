package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentCodeRepository_Lifecycle_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewEnrollmentCodeRepository(db.DB)
	ctx := lagunaCtx()
	admin := seedUser(t, db)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM edge_enrollment_codes WHERE created_by = ?", admin) })
	now := time.Now().UTC().Truncate(time.Second)
	first := &dto.EnrollmentCode{ID: uuid.NewString(), CreatedBy: admin, ExpiresAt: now.Add(30 * time.Minute), CreatedAt: now}
	firstHash := "first-" + first.ID[:8]
	require.NoError(t, repo.Create(ctx, first, firstHash))

	require.NoError(t, repo.InvalidateUnused(ctx, now))
	second := &dto.EnrollmentCode{ID: uuid.NewString(), CreatedBy: admin, ExpiresAt: now.Add(30 * time.Minute), CreatedAt: now}
	secondHash := "second-" + second.ID[:8]
	require.NoError(t, repo.Create(ctx, second, secondHash))

	found, err := repo.FindByHashForUpdate(context.Background(), firstHash)
	require.NoError(t, err, "found with no organization in the context")
	assert.NotNil(t, found.UsedAt, "issuing a new code used up the first")

	require.NoError(t, postgres.NewUnitOfWork(db.DB).Do(context.Background(), func(txCtx context.Context) error {
		locked, lockErr := repo.FindByHashForUpdate(txCtx, secondHash)
		require.NoError(t, lockErr)
		assert.Nil(t, locked.UsedAt)
		assert.Equal(t, lagunaOrgID, locked.OrganizationID)
		return repo.MarkUsed(txCtx, locked.ID, now)
	}))
	assert.ErrorIs(t, repo.MarkUsed(context.Background(), second.ID, now), domainError.ErrEnrollmentCodeInvalid, "a used code is not used twice")

	_, err = repo.FindByHashForUpdate(context.Background(), "nope")
	assert.ErrorIs(t, err, domainError.ErrEnrollmentCodeInvalid)
}
