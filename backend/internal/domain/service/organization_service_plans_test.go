package service

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrganizationService_Create_DefaultsToActiveFull(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	org, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{Name: "Restaurante X S.A.S", Slug: "restaurante-x"})

	require.NoError(t, err)
	assert.Equal(t, "active", org.Status)
	assert.Equal(t, "full", org.Plan)
	assert.Nil(t, org.TrialEndsAt)
}

func TestOrganizationService_Create_Trial(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	org, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{
		Name: "X", Slug: "restaurante-x", Status: "trial", Plan: "starter", TrialEndsAt: "2026-12-09T00:00:00Z",
	})

	require.NoError(t, err)
	assert.Equal(t, "trial", org.Status)
	assert.Equal(t, "starter", org.Plan)
	require.NotNil(t, org.TrialEndsAt)
	assert.Equal(t, time.Date(2026, 12, 9, 0, 0, 0, 0, time.UTC), org.TrialEndsAt.UTC())
}

func TestOrganizationService_Create_InvalidPlanOrTrial(t *testing.T) {
	tests := []struct {
		name  string
		req   dto.CreateOrganizationRequest
		field string
	}{
		{"unknown plan", dto.CreateOrganizationRequest{Name: "X", Slug: "restaurante-x", Plan: "premium"}, "plan"},
		{"trial without an end", dto.CreateOrganizationRequest{Name: "X", Slug: "restaurante-x", Status: "trial"}, "trial_ends_at"},
		{"malformed end", dto.CreateOrganizationRequest{Name: "X", Slug: "restaurante-x", Status: "trial", TrialEndsAt: "next week"}, "trial_ends_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := createTestOrganizationService(t)
			_, err := svc.Create(context.Background(), &tt.req)
			assertValidationError(t, err, tt.field)
		})
	}
}

func TestOrganizationService_Update_TrialWithoutAnEnd(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{ID: "org-1", Status: "active", Plan: "full"}, nil)

	_, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{Status: strPtr("trial")})

	assertValidationError(t, err, "trial_ends_at")
	orgRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestOrganizationService_Update_TrialUsesTheStoredEnd(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	ends := time.Now().Add(time.Hour)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{ID: "org-1", Status: "suspended", TrialEndsAt: &ends}, nil)
	req := &dto.UpdateOrganizationRequest{Status: strPtr("trial")}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{Status: "trial", TrialEndsAt: &ends}, nil)

	org, err := svc.Update(context.Background(), "org-1", req)

	require.NoError(t, err)
	assert.Equal(t, "trial", org.Status)
}

func TestOrganizationService_Update_ClearingTheEndOfATrial(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	ends := time.Now().Add(time.Hour)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{ID: "org-1", Status: "trial", TrialEndsAt: &ends}, nil)

	_, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{TrialEndsAt: strPtr("")})

	assertValidationError(t, err, "trial_ends_at")
}

func TestOrganizationService_Update_InvalidPlanOrTrialEnd(t *testing.T) {
	tests := []struct {
		name  string
		req   dto.UpdateOrganizationRequest
		field string
	}{
		{"unknown plan", dto.UpdateOrganizationRequest{Plan: strPtr("premium")}, "plan"},
		{"empty plan", dto.UpdateOrganizationRequest{Plan: strPtr("")}, "plan"},
		{"malformed end", dto.UpdateOrganizationRequest{TrialEndsAt: strPtr("tomorrow")}, "trial_ends_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := createTestOrganizationService(t)
			_, err := svc.Update(context.Background(), "org-1", &tt.req)
			assertValidationError(t, err, tt.field)
		})
	}
}

func TestOrganizationService_Update_ActivateAPayingCustomer(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	req := &dto.UpdateOrganizationRequest{Status: strPtr("active"), Plan: strPtr("einvoicing")}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{Status: "active", Plan: "einvoicing"}, nil)

	org, err := svc.Update(context.Background(), "org-1", req)

	require.NoError(t, err)
	assert.Equal(t, "active", org.Status)
	assert.Equal(t, "einvoicing", org.Plan)
}

func TestOrganizationService_Update_ExtendATrial(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	later := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Second)
	req := &dto.UpdateOrganizationRequest{TrialEndsAt: strPtr(later.Format(time.RFC3339))}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{Status: "trial", TrialEndsAt: &later}, nil)

	org, err := svc.Update(context.Background(), "org-1", req)

	require.NoError(t, err)
	assert.True(t, later.Equal(*org.TrialEndsAt))
}
