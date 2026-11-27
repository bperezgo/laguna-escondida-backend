package service

import (
	"context"
	"testing"
	"time"

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

func billingPaidUntil(paidUntil time.Time, cancelledAt *time.Time) *dto.OrganizationBilling {
	source, provider := "3891", "wompi"
	return &dto.OrganizationBilling{OrganizationID: testOrgA, Period: "monthly", Provider: &provider, PaymentSourceID: &source,
		PaidUntil: &paidUntil, CancelledAt: cancelledAt, Attempts: 3}
}

// Operator reactivates a lapsed organization: it is never charged and does not stop again.
func TestOrganizationService_Update_ActivatingALapsedOrganizationEndsItsSubscription(t *testing.T) {
	now := time.Date(2027, 1, 25, 12, 0, 0, 0, time.UTC)
	active := dto.OrganizationStatusActive
	cancelledBeforeTheEnd := now.Add(-20 * 24 * time.Hour)
	cases := []struct {
		name    string
		status  string
		billing *dto.OrganizationBilling
		found   bool
		ends    bool
	}{
		{"stopped after three failed renewals", "active", billingPaidUntil(now.Add(-10*24*time.Hour), nil), true, true},
		{"suspended while unpaid", "suspended", billingPaidUntil(now.Add(-10*24*time.Hour), nil), true, true},
		{"cancelled and past its period", "active", billingPaidUntil(now.Add(-24*time.Hour), &cancelledBeforeTheEnd), true, true},
		{"paid and current", "active", billingPaidUntil(now.Add(10*24*time.Hour), nil), true, false},
		{"renewal still in grace", "active", billingPaidUntil(now.Add(-24*time.Hour), nil), true, false},
		{"expired trial that never paid", "trial", &dto.OrganizationBilling{OrganizationID: testOrgA, Period: "monthly"}, true, false},
		{"no billing row", "trial", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, orgRepo, _ := createTestOrganizationService(t)
			svc.now = func() time.Time { return now }
			billing := mocks.NewMockBillingRepository(t)
			svc.ShowBilling(billing)
			billing.EXPECT().Get(mock.Anything, testOrgA).Return(tc.billing, tc.found, nil)
			orgRepo.EXPECT().Update(mock.Anything, testOrgA, mock.Anything).Return(&dto.Organization{ID: testOrgA, Status: active}, nil)
			if tc.ends {
				billing.EXPECT().EndSubscription(mock.Anything, testOrgA, now).Return(nil)
			}
			billing.EXPECT().GetMany(mock.Anything, []string{testOrgA}).Return(map[string]*dto.OrganizationBilling{}, nil)

			org, err := svc.Update(context.Background(), testOrgA, &dto.UpdateOrganizationRequest{Status: &active})

			require.NoError(t, err)
			assert.Equal(t, active, org.Status)
		})
	}
}

func TestOrganizationService_Update_OtherChangesNeverTouchTheSubscription(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	billing := mocks.NewMockBillingRepository(t)
	svc.ShowBilling(billing)
	plan := "full"
	orgRepo.EXPECT().Update(mock.Anything, testOrgA, mock.Anything).Return(&dto.Organization{ID: testOrgA, Plan: plan}, nil)
	billing.EXPECT().GetMany(mock.Anything, []string{testOrgA}).Return(map[string]*dto.OrganizationBilling{}, nil)

	_, err := svc.Update(context.Background(), testOrgA, &dto.UpdateOrganizationRequest{Plan: &plan})

	require.NoError(t, err)
}
