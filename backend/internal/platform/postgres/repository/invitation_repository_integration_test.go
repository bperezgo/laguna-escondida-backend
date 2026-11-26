package repository

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/platform/postgres"

	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvitationRepository_Lifecycle_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewInvitationRepository(db.DB)
	ctx := lagunaCtx()
	inviter := seedUser(t, db)
	email := "inv-" + uuid.NewString()[:8] + "@example.com"
	now := time.Now().UTC().Truncate(time.Second)
	inv := &dto.Invitation{ID: uuid.NewString(), Email: email, Name: "Ana", RoleIDs: []int{3, 5}, InvitedBy: inviter,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	t.Cleanup(func() { db.DB.Exec("DELETE FROM invitations WHERE invited_by = ?", inviter) })

	require.NoError(t, repo.Create(ctx, inv, "hash-1"))
	dup := *inv
	dup.ID = uuid.NewString()
	assert.ErrorIs(t, repo.Create(ctx, &dup, "hash-2"), domainError.ErrInvitationPending, "one pending invitation per email")

	list, err := repo.ListPending(ctx)
	require.NoError(t, err)
	var found *dto.Invitation
	for _, i := range list {
		if i.ID == inv.ID {
			found = i
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, []int{3, 5}, found.RoleIDs)
	assert.NotEmpty(t, found.InvitedByName)

	byToken, err := repo.FindByTokenHash(context.Background(), "hash-1", false)
	require.NoError(t, err, "found with no organization in the context")
	assert.Equal(t, lagunaOrgID, byToken.OrganizationID)

	require.NoError(t, repo.ReplaceToken(ctx, inv.ID, "hash-3", now.Add(2*time.Hour)))
	_, err = repo.FindByTokenHash(context.Background(), "hash-1", false)
	assert.ErrorIs(t, err, domainError.ErrInvitationNotFound, "the old link no longer works")

	require.NoError(t, postgres.NewUnitOfWork(db.DB).Do(context.Background(), func(txCtx context.Context) error {
		locked, lockErr := repo.FindByTokenHash(txCtx, "hash-3", true)
		require.NoError(t, lockErr)
		return repo.MarkAccepted(txCtx, locked.ID, now)
	}))
	assert.ErrorIs(t, repo.MarkAccepted(context.Background(), inv.ID, now), domainError.ErrInvitationInvalidOrExpired)
	_, err = repo.FindPending(ctx, inv.ID)
	assert.ErrorIs(t, err, domainError.ErrInvitationNotFound, "accepted is no longer pending")
	assert.ErrorIs(t, repo.Revoke(ctx, inv.ID, now), domainError.ErrInvitationNotFound)

	again := *inv
	again.ID = uuid.NewString()
	require.NoError(t, repo.Create(ctx, &again, "hash-4"), "the email can be invited again once the first was accepted")
}
