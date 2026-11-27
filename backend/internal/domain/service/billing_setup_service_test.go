package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var billingNow = time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)

type billingSetupFixture struct {
	userServiceFixture
	billing    *mocks.MockBillingRepository
	provider   *mocks.MockBillingProvider
	service    *BillingSetupService
	rolledBack bool
}

func newBillingSetupFixture(t *testing.T, cardEntry bool) *billingSetupFixture {
	t.Helper()
	f := &billingSetupFixture{
		userServiceFixture: newUserServiceFixture(t, tenant.AnyOrganization()),
		billing:            mocks.NewMockBillingRepository(t),
		provider:           mocks.NewMockBillingProvider(t),
	}
	f.service = NewBillingSetupService(f.billing, f.orgRepo, f.userRepo, f.userRoleRepo, f.userServiceFixture.service,
		rollbackUnitOfWork{rolledBack: &f.rolledBack})
	f.service.now = func() time.Time { return billingNow }
	if cardEntry {
		f.service.EnableCardEntry(f.provider, dto.BillingProviderPublic{PublicKey: "pub_test_abc", APIURL: "https://sandbox.wompi.co/v1"})
	}
	return f
}

func (f *billingSetupFixture) orgIs(org *dto.Organization) {
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(org, nil)
}

func (f *billingSetupFixture) adminIsRosa() {
	email := "rosa@example.com"
	f.userRepo.On("FindByID", mock.MatchedBy(inOrg(testOrgA)), "rosa-id").
		Return(&dto.User{ID: "rosa-id", Username: email, Name: "Rosa", Email: &email, Active: true}, nil)
}

func cardRequest() *dto.SaveCardRequest {
	return &dto.SaveCardRequest{CardToken: "tok_test_1", AcceptanceToken: "acc-1", PersonalDataAcceptanceToken: "acc-2"}
}

func TestBillingSetupService_State_CardEntryOff(t *testing.T) {
	f := newBillingSetupFixture(t, false)
	f.orgIs(pendingOrg())
	f.billing.EXPECT().Get(mock.Anything, testOrgA).Return(nil, false, nil)

	state, err := f.service.State(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.Equal(t, &dto.BillingSetupState{OrganizationName: "", Plan: "einvoicing", Period: "monthly", CardEntry: false}, state)
	assert.Nil(t, state.Wompi)
}

func TestBillingSetupService_State_ReturnsTheChosenPeriod(t *testing.T) {
	f := newBillingSetupFixture(t, false)
	f.orgIs(pendingOrg())
	f.billing.EXPECT().Get(mock.Anything, testOrgA).Return(&dto.OrganizationBilling{OrganizationID: testOrgA, Period: "annual"}, true, nil)

	state, err := f.service.State(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.Equal(t, "annual", state.Period)
}

func TestBillingSetupService_State_CardEntryOn(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.billing.EXPECT().Get(mock.Anything, testOrgA).Return(nil, false, nil)
	f.provider.EXPECT().Terms(mock.Anything).Return(dto.WompiMerchantTerms{
		TermsURL: "https://wompi.co/terms.pdf", PersonalDataURL: "https://wompi.co/data.pdf", Email: "merchant@example.com",
		AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2",
	}, nil)

	state, err := f.service.State(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.True(t, state.CardEntry)
	assert.Equal(t, &dto.BillingSetupWompi{
		PublicKey: "pub_test_abc", APIURL: "https://sandbox.wompi.co/v1",
		TermsURL: "https://wompi.co/terms.pdf", PersonalDataURL: "https://wompi.co/data.pdf",
		AcceptanceToken: "acc-1", PersonalDataAcceptanceToken: "acc-2",
	}, state.Wompi)
}

func TestBillingSetupService_State_ProviderDown(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.billing.EXPECT().Get(mock.Anything, testOrgA).Return(nil, false, nil)
	f.provider.EXPECT().Terms(mock.Anything).Return(dto.WompiMerchantTerms{}, errors.New("dial tcp: timeout"))

	_, err := f.service.State(context.Background(), testOrgA)

	assert.ErrorIs(t, err, domainError.ErrPaymentProviderUnavailable)
}

func TestBillingSetupService_Select(t *testing.T) {
	cases := []struct {
		name         string
		plan, period string
		amount       int
		changesPlan  bool
	}{
		{"annual on the same plan", "einvoicing", "annual", 400000, false},
		{"another plan, monthly", "full", "monthly", 100000, true},
		{"another plan, annual", "starter", "annual", 250000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newBillingSetupFixture(t, false)
			f.orgIs(pendingOrg())
			if tc.changesPlan {
				f.orgRepo.On("Update", mock.MatchedBy(inUnitOfWork), testOrgA, mock.MatchedBy(func(req *dto.UpdateOrganizationRequest) bool {
					return req.Plan != nil && *req.Plan == tc.plan && req.Status == nil && req.TrialEndsAt == nil
				})).Return(pendingOrg(), nil)
			}
			f.billing.EXPECT().SetPeriod(mock.MatchedBy(inUnitOfWork), testOrgA, tc.period).Return(nil)

			selection, err := f.service.Select(context.Background(), testOrgA, &dto.BillingSelectionRequest{Plan: tc.plan, Period: tc.period})

			require.NoError(t, err)
			assert.Equal(t, &dto.BillingSelection{Plan: tc.plan, Period: tc.period, Amount: tc.amount, Currency: "COP"}, selection)
			f.orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
			if !tc.changesPlan {
				f.orgRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestBillingSetupService_Select_InvalidFieldStoresNothing(t *testing.T) {
	cases := []struct{ plan, period, field string }{
		{"einvoicing", "weekly", "period"},
		{"einvoicing", "", "period"},
		{"gold", "monthly", "plan"},
	}
	for _, tc := range cases {
		t.Run(tc.plan+"/"+tc.period, func(t *testing.T) {
			f := newBillingSetupFixture(t, false)

			_, err := f.service.Select(context.Background(), testOrgA, &dto.BillingSelectionRequest{Plan: tc.plan, Period: tc.period})

			var validation *domainError.BillingValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tc.field, validation.Field)
		})
	}
}

func TestBillingSetupService_Select_AfterTheTrialStarted(t *testing.T) {
	f := newBillingSetupFixture(t, false)
	f.orgIs(trialOrg(13 * 24 * time.Hour))

	_, err := f.service.Select(context.Background(), testOrgA, &dto.BillingSelectionRequest{Plan: "full", Period: "annual"})

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotPending)
	assert.True(t, f.rolledBack)
}

func TestBillingSetupService_SaveCard_StoresTheCardAndStartsTheTrial(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.adminIsRosa()
	f.provider.EXPECT().SavePaymentSource(mock.Anything, dto.PaymentSourceRequest{
		CardToken: "tok_test_1", CustomerEmail: "rosa@example.com", AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2",
	}).Return(dto.PaymentSource{ID: "3891", Brand: "VISA", LastFour: "4242"}, nil)
	f.billing.EXPECT().SaveCard(mock.MatchedBy(inUnitOfWork), testOrgA, dto.BillingCard{
		Provider: "wompi", PaymentSourceID: "3891", Brand: "VISA", LastFour: "4242", TermsAcceptedAt: billingNow,
	}).Return(nil)
	f.orgRepo.On("StartTrial", mock.MatchedBy(inUnitOfWork), testOrgA, billingNow.Add(14*24*time.Hour)).Return(trialOrg(14*24*time.Hour), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.MatchedBy(inOrg(testOrgA)), "rosa-id").
		Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)

	signIn, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	require.NoError(t, err)
	claims, err := f.jwt.ValidateToken(signIn.Token)
	require.NoError(t, err, "the answer is a sign-in token, not a setup token")
	assert.Equal(t, testOrgA, claims.OrgID)
	assert.Equal(t, "rosa-id", claims.UserID)
	assert.Equal(t, "rosa@example.com", signIn.Username)
	assert.NotEmpty(t, signIn.Permissions)
	assert.False(t, f.rolledBack)
}

func TestBillingSetupService_SaveCard_RefusedStoresNothing(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.adminIsRosa()
	f.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, domainError.ErrCardRefused)

	signIn, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	assert.Nil(t, signIn)
	assert.ErrorIs(t, err, domainError.ErrCardRefused)
	f.billing.AssertNotCalled(t, "SaveCard", mock.Anything, mock.Anything, mock.Anything)
	f.orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
}

func TestBillingSetupService_SaveCard_ProviderDownStoresNothing(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.adminIsRosa()
	f.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, domainError.ErrPaymentProviderUnavailable)

	_, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	assert.ErrorIs(t, err, domainError.ErrPaymentProviderUnavailable)
	f.orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
}

func TestBillingSetupService_SaveCard_DatabaseFailureAfterTheProviderLeavesItPending(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.adminIsRosa()
	f.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{ID: "3891", Brand: "VISA", LastFour: "4242"}, nil)
	f.billing.EXPECT().SaveCard(mock.Anything, testOrgA, mock.Anything).Return(errors.New("connection reset"))

	signIn, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	assert.Nil(t, signIn)
	require.Error(t, err)
	assert.True(t, f.rolledBack, "the card and the trial are stored together or not at all")
	f.orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
}

// The loser of two concurrent saves: its card is stored in the transaction the trial start then rolls back.
func TestBillingSetupService_SaveCard_TrialStartedMeanwhileRollsTheCardBack(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(pendingOrg())
	f.adminIsRosa()
	f.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{ID: "3892"}, nil)
	f.billing.EXPECT().SaveCard(mock.Anything, testOrgA, mock.Anything).Return(nil)
	f.orgRepo.On("StartTrial", mock.Anything, testOrgA, mock.Anything).Return(nil, domainError.ErrOrganizationNotPending)

	_, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotPending)
	assert.True(t, f.rolledBack)
}

func TestBillingSetupService_SaveCard_NotPendingNeverReachesTheProvider(t *testing.T) {
	f := newBillingSetupFixture(t, true)
	f.orgIs(trialOrg(13 * 24 * time.Hour))

	_, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotPending)
	f.provider.AssertNotCalled(t, "SavePaymentSource", mock.Anything, mock.Anything)
}

func TestBillingSetupService_SaveCard_MissingToken(t *testing.T) {
	cases := []struct {
		field  string
		mutate func(*dto.SaveCardRequest)
	}{
		{"card_token", func(r *dto.SaveCardRequest) { r.CardToken = " " }},
		{"acceptance_token", func(r *dto.SaveCardRequest) { r.AcceptanceToken = "" }},
		{"personal_data_acceptance_token", func(r *dto.SaveCardRequest) { r.PersonalDataAcceptanceToken = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			f := newBillingSetupFixture(t, true)
			req := cardRequest()
			tc.mutate(req)

			_, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", req)

			var validation *domainError.BillingValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tc.field, validation.Field)
		})
	}
}

func TestBillingSetupService_SaveCard_CardEntryOff(t *testing.T) {
	f := newBillingSetupFixture(t, false)

	_, err := f.service.SaveCard(context.Background(), testOrgA, "rosa-id", cardRequest())

	assert.ErrorIs(t, err, domainError.ErrCardEntryOff)
}
