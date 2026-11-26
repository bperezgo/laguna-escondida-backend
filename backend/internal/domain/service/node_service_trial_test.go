package service

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNodeService_Authenticate_ExpiredTrialIsRefusedAsSuspended(t *testing.T) {
	const key = "lgn_right"
	past := time.Now().Add(-time.Hour)
	f, _ := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord(key, func(r *dto.NodeAuthRecord) {
		r.OrganizationStatus, r.TrialEndsAt = dto.OrganizationStatusTrial, &past
	}), nil).Once()

	for i := 0; i < 2; i++ {
		_, err := f.service.Authenticate(context.Background(), nodeTestID, key)
		require.ErrorIs(t, err, domainError.ErrNodeOrganizationSuspended, "the same refusal, so the box backs off as for a suspension")
	}
}

func TestNodeService_Authenticate_PendingOrganizationIsRefusedAsSuspended(t *testing.T) {
	const key = "lgn_right"
	f, _ := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord(key, func(r *dto.NodeAuthRecord) {
		r.OrganizationStatus, r.TrialEndsAt = dto.OrganizationStatusPending, nil
	}), nil).Once()

	_, err := f.service.Authenticate(context.Background(), nodeTestID, key)
	require.ErrorIs(t, err, domainError.ErrNodeOrganizationSuspended)
}

func TestNodeService_Authenticate_TrialInProgressIsAccepted(t *testing.T) {
	const key = "lgn_right"
	future := time.Now().Add(time.Hour)
	f := newNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord(key, func(r *dto.NodeAuthRecord) {
		r.OrganizationStatus, r.TrialEndsAt = dto.OrganizationStatusTrial, &future
	}), nil)
	f.nodes.On("TouchLastSeen", mock.Anything, nodeTestID, mock.Anything, nodeSeenInterval).Return(nil)

	orgID, err := f.service.Authenticate(context.Background(), nodeTestID, key)

	require.NoError(t, err)
	assert.Equal(t, testOrgA, orgID)
}

func TestNodeService_Authenticate_ActiveWithAPastTrialEndIsAccepted(t *testing.T) {
	const key = "lgn_right"
	past := time.Now().Add(-48 * time.Hour)
	f := newNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord(key, func(r *dto.NodeAuthRecord) { r.TrialEndsAt = &past }), nil)
	f.nodes.On("TouchLastSeen", mock.Anything, nodeTestID, mock.Anything, nodeSeenInterval).Return(nil)

	orgID, err := f.service.Authenticate(context.Background(), nodeTestID, key)

	require.NoError(t, err)
	assert.Equal(t, testOrgA, orgID)
}

func TestNodeService_Authenticate_ExpiredTrialWithAWrongKeyStaysOpaque(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	f := newNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord("lgn_right", func(r *dto.NodeAuthRecord) {
		r.OrganizationStatus, r.TrialEndsAt = dto.OrganizationStatusTrial, &past
	}), nil)

	_, err := f.service.Authenticate(context.Background(), nodeTestID, "lgn_wrong")

	assert.ErrorIs(t, err, domainError.ErrNodeKeyInvalid)
}
