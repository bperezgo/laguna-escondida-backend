package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func pendingOrganization() *dto.Organization {
	return &dto.Organization{ID: "org-1", Status: dto.OrganizationStatusPending, Plan: "einvoicing"}
}

func TestOrganizationService_StartTrial_Pending(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	bogota := time.FixedZone("COT", -5*3600)
	svc.now = func() time.Time { return time.Date(2026, 12, 1, 10, 0, 0, 0, bogota) }
	want := time.Date(2026, 12, 15, 10, 0, 0, 0, bogota)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(pendingOrganization(), nil)
	orgRepo.EXPECT().StartTrial(mock.Anything, "org-1", mock.MatchedBy(want.Equal)).
		Return(&dto.Organization{ID: "org-1", Status: "trial", Plan: "einvoicing", TrialEndsAt: &want}, nil)

	org, err := svc.StartTrial(context.Background(), "org-1")

	require.NoError(t, err)
	assert.Equal(t, "trial", org.Status)
	assert.Equal(t, "einvoicing", org.Plan)
	assert.True(t, want.Equal(*org.TrialEndsAt))
}

func TestOrganizationService_StartTrial_NotPending(t *testing.T) {
	ends := time.Now().Add(time.Hour)
	for _, current := range []*dto.Organization{
		{ID: "org-1", Status: "trial", TrialEndsAt: &ends},
		{ID: "org-1", Status: "active"},
		{ID: "org-1", Status: "suspended"},
	} {
		t.Run(current.Status, func(t *testing.T) {
			svc, orgRepo, _ := createTestOrganizationService(t)
			orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(current, nil)

			_, err := svc.StartTrial(context.Background(), "org-1")

			assert.ErrorIs(t, err, domainError.ErrOrganizationNotPending)
			orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestOrganizationService_StartTrial_ConcurrentStartLosesTheRace(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(pendingOrganization(), nil)
	orgRepo.EXPECT().StartTrial(mock.Anything, "org-1", mock.Anything).Return(nil, domainError.ErrOrganizationNotPending)

	_, err := svc.StartTrial(context.Background(), "org-1")

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotPending)
}

func TestOrganizationService_StartTrial_NotFound(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "missing").Return(nil, domainError.ErrOrganizationNotFound)

	_, err := svc.StartTrial(context.Background(), "missing")

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationService_Update_PendingIntoTrialIsRefused(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(pendingOrganization(), nil)

	_, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{
		Status: strPtr("trial"), TrialEndsAt: strPtr("2026-12-31T23:59:59-05:00")})

	assertValidationError(t, err, "status")
	orgRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestOrganizationService_Update_TrialEndOnAPendingOrganizationIsRefused(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(pendingOrganization(), nil)

	_, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{TrialEndsAt: strPtr("2026-12-31T23:59:59-05:00")})

	assertValidationError(t, err, "trial_ends_at")
	orgRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestOrganizationService_Update_SettingPendingIsRefused(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)

	_, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{Status: strPtr("pending")})
	assertValidationError(t, err, "status")

	_, err = svc.Create(context.Background(), &dto.CreateOrganizationRequest{Name: "X", Slug: "restaurante-x", Status: "pending"})
	assertValidationError(t, err, "status")

	orgRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
	orgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestOrganizationService_Update_PendingToActiveOrSuspended(t *testing.T) {
	for _, status := range []string{"active", "suspended"} {
		t.Run(status, func(t *testing.T) {
			svc, orgRepo, _ := createTestOrganizationService(t)
			req := &dto.UpdateOrganizationRequest{Status: strPtr(status)}
			orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{ID: "org-1", Status: status}, nil)

			org, err := svc.Update(context.Background(), "org-1", req)

			require.NoError(t, err)
			assert.Equal(t, status, org.Status)
		})
	}
}

func TestOrganizationService_Update_PlanChangeKeepsItPending(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	req := &dto.UpdateOrganizationRequest{Plan: strPtr("full")}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{ID: "org-1", Status: "pending", Plan: "full"}, nil)

	org, err := svc.Update(context.Background(), "org-1", req)

	require.NoError(t, err)
	assert.Equal(t, "pending", org.Status)
	assert.Equal(t, "full", org.Plan)
}

func TestOrganizationService_List_ByStatus(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().ListByStatus(mock.Anything, "pending").Return([]*dto.Organization{pendingOrganization()}, nil)

	orgs, err := svc.List(context.Background(), "pending")

	require.NoError(t, err)
	assert.Len(t, orgs, 1)
	orgRepo.AssertNotCalled(t, "List", mock.Anything)
}

func TestOrganizationService_List_UnknownStatus(t *testing.T) {
	svc, _, _ := createTestOrganizationService(t)

	_, err := svc.List(context.Background(), "waiting")

	assertValidationError(t, err, "status")
}

func TestOrganizationService_List_ByStatusError(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().ListByStatus(mock.Anything, "pending").Return(nil, errors.New("db down"))

	_, err := svc.List(context.Background(), "pending")

	assert.Error(t, err)
}
