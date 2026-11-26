package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationRepository_StartTrial_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	ctx := context.Background()
	org := newTestOrganization(t, db, "Start trial")
	org.Status, org.Plan = dto.OrganizationStatusPending, "einvoicing"
	require.NoError(t, repo.Create(ctx, org))
	ends := time.Now().Add(14 * 24 * time.Hour).UTC().Truncate(time.Microsecond)

	got, err := repo.StartTrial(ctx, org.ID, ends)

	require.NoError(t, err)
	assert.Equal(t, dto.OrganizationStatusTrial, got.Status)
	assert.Equal(t, "einvoicing", got.Plan)
	require.NotNil(t, got.TrialEndsAt)
	assert.True(t, ends.Equal(*got.TrialEndsAt))

	_, err = repo.StartTrial(ctx, org.ID, ends.Add(time.Hour))
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotPending, "only one start succeeds")
	again, err := repo.Get(ctx, org.ID)
	require.NoError(t, err)
	assert.True(t, ends.Equal(*again.TrialEndsAt), "the second start changes nothing")

	_, err = repo.StartTrial(ctx, uuid.Must(uuid.NewV7()).String(), ends)
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationRepository_ListByStatus_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	ctx := context.Background()
	pending := newTestOrganization(t, db, "AAA pending")
	pending.Status = dto.OrganizationStatusPending
	require.NoError(t, repo.Create(ctx, pending))
	active := newTestOrganization(t, db, "AAB active")
	require.NoError(t, repo.Create(ctx, active))

	orgs, err := repo.ListByStatus(ctx, dto.OrganizationStatusPending)

	require.NoError(t, err)
	ids := map[string]bool{}
	for _, o := range orgs {
		assert.Equal(t, dto.OrganizationStatusPending, o.Status)
		ids[o.ID] = true
	}
	assert.True(t, ids[pending.ID])
	assert.False(t, ids[active.ID])
}
