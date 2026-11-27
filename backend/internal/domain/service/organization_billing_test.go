package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrganizationService_ShowBilling_ListCarriesPeriodAndSavedCard(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	billing := mocks.NewMockBillingRepository(t)
	svc.ShowBilling(billing)
	orgRepo.EXPECT().List(mock.Anything).Return([]*dto.Organization{{ID: testOrgA}, {ID: testOrgB}, {ID: "no-row"}}, nil)
	source, provider := "3891", "wompi"
	billing.EXPECT().GetMany(mock.Anything, []string{testOrgA, testOrgB, "no-row"}).Return(map[string]*dto.OrganizationBilling{
		testOrgA: {OrganizationID: testOrgA, Period: "annual"},
		testOrgB: {OrganizationID: testOrgB, Period: "monthly", Provider: &provider, PaymentSourceID: &source},
	}, nil)

	orgs, err := svc.List(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, []string{"annual", "monthly", "monthly"}, []string{orgs[0].BillingPeriod, orgs[1].BillingPeriod, orgs[2].BillingPeriod})
	assert.Equal(t, []bool{false, true, false}, []bool{orgs[0].HasSavedCard, orgs[1].HasSavedCard, orgs[2].HasSavedCard})
}

// The operator approves by hand with or without a card: the answer shows what the admin chose.
func TestOrganizationService_ShowBilling_StartTrialWithoutACard(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	billing := mocks.NewMockBillingRepository(t)
	svc.ShowBilling(billing)
	orgRepo.EXPECT().Get(mock.Anything, testOrgA).Return(pendingOrg(), nil)
	orgRepo.EXPECT().StartTrial(mock.Anything, testOrgA, mock.Anything).Return(&dto.Organization{ID: testOrgA, Status: dto.OrganizationStatusTrial}, nil)
	billing.EXPECT().GetMany(mock.Anything, []string{testOrgA}).Return(map[string]*dto.OrganizationBilling{
		testOrgA: {OrganizationID: testOrgA, Period: "annual"},
	}, nil)

	org, err := svc.StartTrial(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.Equal(t, dto.OrganizationStatusTrial, org.Status)
	assert.Equal(t, "annual", org.BillingPeriod)
	assert.False(t, org.HasSavedCard)
}

func TestOrganizationService_WithoutBilling_AnswersMonthlyAndNoCard(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrgA).Return(&dto.Organization{ID: testOrgA}, nil)

	org, err := svc.Get(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.Equal(t, "monthly", org.BillingPeriod)
	assert.False(t, org.HasSavedCard)
}
