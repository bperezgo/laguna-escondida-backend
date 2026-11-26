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

func TestOrganizationAccessReader_GetAccess_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	ctx := context.Background()
	org := newTestOrganization(t, db, "Access reader")
	ends := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	org.Status, org.Plan, org.TrialEndsAt = dto.OrganizationStatusTrial, "einvoicing", &ends
	require.NoError(t, NewOrganizationRepository(db.DB).Create(ctx, org))
	reader := NewOrganizationAccessReader(db.DB)

	access, err := reader.GetAccess(ctx, org.ID)

	require.NoError(t, err, "reads with no organization in the context")
	assert.Equal(t, "trial", access.Status)
	assert.Equal(t, "einvoicing", access.Plan)
	require.NotNil(t, access.TrialEndsAt)
	assert.True(t, ends.Equal(*access.TrialEndsAt))

	_, err = reader.GetAccess(ctx, uuid.NewString())
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
	_, err = reader.GetAccess(ctx, "nope")
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}
