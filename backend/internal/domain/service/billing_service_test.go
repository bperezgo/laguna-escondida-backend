package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type billingServiceFixture struct {
	*chargeFixture
	users   userServiceFixture
	setup   *BillingSetupService
	service *BillingService
}

// newBillingServiceFixture is the cloud with all four billing values; charging false is the keys alone.
func newBillingServiceFixture(t *testing.T, cardEntry, charging bool) *billingServiceFixture {
	t.Helper()
	f := &billingServiceFixture{chargeFixture: newChargeFixture(t), users: newUserServiceFixture(t, tenant.AnyOrganization())}
	billing := chargeBilling{f.store}
	f.setup = NewBillingSetupService(billing, f.store, f.users.userRepo, f.users.userRoleRepo, f.users.service, passthroughUnitOfWork{})
	f.setup.now = func() time.Time { return f.now }
	if cardEntry {
		f.setup.EnableCardEntry(f.provider, dto.BillingProviderPublic{PublicKey: "pub_test_abc", APIURL: "https://sandbox.wompi.co/v1"})
	}
	f.service = NewBillingService(billing, f.store, f.users.userRoleRepo, f.users.service, f.setup, passthroughUnitOfWork{}, f.access)
	f.service.now = func() time.Time { return f.now }
	if charging {
		f.service.EnableCharging(f.chargeFixture.service)
	}
	email := "rosa@example.com"
	f.users.userRepo.On("FindByID", mock.Anything, "rosa-id").
		Return(&dto.User{ID: "rosa-id", Username: email, Name: "Rosa", Email: &email, Active: true}, nil).Maybe()
	f.users.userRoleRepo.On("FindRolesByUserID", mock.Anything, "rosa-id").
		Return([]*dto.Role{{ID: permissions.RoleAdmin, Name: "admin"}}, nil).Maybe()
	return f
}

func newCardRequest() *dto.SaveCardRequest {
	return &dto.SaveCardRequest{CardToken: "tok_test_new", AcceptanceToken: "acc-1", PersonalDataAcceptanceToken: "acc-2"}
}

// "During the trial": einvoicing, monthly, a Visa ending in 4242.
func TestBillingService_Read_DuringTheTrial(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.at(-4 * day)

	sub, err := f.service.Read(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.Equal(t, "einvoicing", sub.Plan)
	assert.Equal(t, "monthly", sub.Period)
	assert.Equal(t, "trial", sub.Status)
	require.NotNil(t, sub.DueDate)
	assert.True(t, sub.DueDate.Equal(chargeDue), "the trial's end is the due date")
	require.NotNil(t, sub.NextChargeAmount)
	assert.Equal(t, 40000, *sub.NextChargeAmount)
	assert.Equal(t, "COP", sub.Currency)
	assert.False(t, sub.Cancelled)
	assert.False(t, sub.PaymentFailed)
	assert.Equal(t, &dto.SavedCard{Brand: "VISA", LastFour: "4242"}, sub.Card)
	assert.Nil(t, sub.LatestCharge)
	assert.Empty(t, sub.Charges)
	require.NotNil(t, sub.AccessEndsAt)
	assert.True(t, sub.AccessEndsAt.Equal(chargeDue.Add(3*day)))

	body, err := json.Marshal(sub)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "3891", "the payment source's identifier never leaves")
	assert.NotContains(t, string(body), "payment_source")
}

func TestBillingService_Read_WithoutACard(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *billingServiceFixture)
	}{
		{"activated by hand, no billing row", func(f *billingServiceFixture) {
			f.store.orgs[testOrgA] = &dto.Organization{ID: testOrgA, Status: "active", Plan: "full"}
		}},
		{"a trial the operator started", func(f *billingServiceFixture) {
			end := chargeDue
			f.store.orgs[testOrgA] = &dto.Organization{ID: testOrgA, Status: "trial", Plan: "full", TrialEndsAt: &end}
			f.store.billing[testOrgA] = &dto.OrganizationBilling{OrganizationID: testOrgA, Period: "annual"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newBillingServiceFixture(t, true, true)
			tc.setup(f)
			f.at(-day)

			sub, err := f.service.Read(context.Background(), testOrgA)

			require.NoError(t, err)
			assert.Equal(t, "full", sub.Plan)
			assert.Nil(t, sub.Card)
			assert.Nil(t, sub.NextChargeAmount)
			assert.Nil(t, sub.LatestCharge)
			assert.NotNil(t, sub.Charges, "an empty list, not null")
			assert.Empty(t, sub.Charges)
			assert.False(t, sub.Cancelled)
		})
	}
}

func TestBillingService_Read_ActivatedByHandWithACardIsNeverCharged(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "full", "monthly")
	f.store.orgs[testOrgA].Status = "active"
	f.at(40 * day)

	sub, err := f.service.Read(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.Nil(t, sub.NextChargeAmount)
	assert.Nil(t, sub.DueDate)
	assert.Nil(t, sub.AccessEndsAt, "it never stops")
	assert.NotNil(t, sub.Card)
}

func TestBillingService_Read_ChargesNewestFirstWithWhatTheyCover(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.answer = dto.BillingChargeDeclined
	f.chargeDue(t)
	f.at(day)
	f.provider.answer = dto.BillingChargeApproved
	f.chargeDue(t)

	sub, err := f.service.Read(context.Background(), testOrgA)

	require.NoError(t, err)
	require.Len(t, sub.Charges, 2)
	assert.Equal(t, "approved", sub.Charges[0].Status)
	assert.Equal(t, "declined", sub.Charges[1].Status)
	assert.Equal(t, 40000, sub.Charges[0].Amount)
	assert.True(t, sub.Charges[0].Date.Equal(chargeDue.Add(day)))
	require.NotNil(t, sub.Charges[0].CoversFrom)
	assert.True(t, sub.Charges[0].CoversFrom.Equal(chargeDue))
	assert.True(t, sub.Charges[0].CoversUntil.Equal(time.Date(2027, 1, 15, 14, 0, 0, 0, time.UTC)))
	assert.Nil(t, sub.Charges[1].CoversFrom)
	assert.Equal(t, &sub.Charges[0], sub.LatestCharge)
	assert.Equal(t, "active", sub.Status)
	assert.False(t, sub.PaymentFailed)
	assert.True(t, sub.DueDate.Equal(time.Date(2027, 1, 15, 14, 0, 0, 0, time.UTC)))
}

func TestBillingService_Summary_PaymentFailedWhileStillWorking(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.provider.answer = dto.BillingChargeDeclined
	f.chargeDue(t)

	f.at(day - time.Hour)
	summary, err := f.service.Summary(context.Background(), testOrgA)
	require.NoError(t, err)
	assert.True(t, summary.PaymentFailed)
	assert.True(t, summary.AccessEndsAt.Equal(chargeDue.Add(3*day)), "the day access ends")
	assert.True(t, summary.HasCard)
	assert.True(t, summary.CardEntry)

	f.at(10 * day)
	stopped, err := f.service.Summary(context.Background(), testOrgA)
	require.NoError(t, err)
	assert.False(t, stopped.PaymentFailed, "a stopped organization shows no notice: nobody is signed in")
}

// "New card after a declined charge": the new card is saved and charged at once.
func TestBillingService_ChangeCard_AfterADeclinedChargeChargesAtOnce(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.provider.answer = dto.BillingChargeDeclined
	f.chargeDue(t)
	f.provider.answer = dto.BillingChargePending

	f.at(day - time.Hour)
	sub, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", newCardRequest())

	require.NoError(t, err)
	assert.Equal(t, &dto.SavedCard{Brand: "MASTERCARD", LastFour: "5100"}, sub.Card)
	require.Len(t, f.provider.requests, 2, "one charge for the unpaid period")
	assert.Equal(t, "5001", f.provider.requests[1].PaymentSourceID, "with the new card")
	assert.Equal(t, int64(4000000), f.provider.requests[1].AmountInCents)
	assert.Equal(t, "rosa@example.com", f.provider.requests[1].CustomerEmail)
	assert.Equal(t, "pending", sub.LatestCharge.Status)
	assert.Equal(t, 1, f.store.billing[testOrgA].Attempts, "it is not one of the three scheduled attempts")
	assert.Equal(t, dto.PaymentSourceRequest{CardToken: "tok_test_new", CustomerEmail: "rosa@example.com",
		AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2"}, f.provider.sources[0])
}

func TestBillingService_ChangeCard_BeforeTheDueDateChargesNothing(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.at(-5 * day)

	sub, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", newCardRequest())

	require.NoError(t, err)
	assert.Equal(t, "5100", sub.Card.LastFour)
	assert.Empty(t, f.provider.requests)
	assert.Equal(t, "trial", f.store.orgs[testOrgA].Status, "changing the card starts no trial")
}

// "New card refused": 402 card_refused, and the previous card is still the saved one.
func TestBillingService_ChangeCard_RefusedKeepsThePreviousCard(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.sourceErr = fmt.Errorf("%w: 422", domainError.ErrCardRefused)

	_, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", newCardRequest())

	require.ErrorIs(t, err, domainError.ErrCardRefused)
	assert.Equal(t, "3891", *f.store.billing[testOrgA].PaymentSourceID)
	assert.Equal(t, "4242", *f.store.billing[testOrgA].CardLastFour)
	assert.Empty(t, f.provider.requests)
}

func TestBillingService_ChangeCard_SameRulesAsTheFirstSave(t *testing.T) {
	t.Run("a missing token names its field", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.trialWithCard(testOrgA, "einvoicing", "monthly")
		req := newCardRequest()
		req.AcceptanceToken = " "

		_, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", req)

		var validation *domainError.BillingValidationError
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, "acceptance_token", validation.Field)
		assert.Empty(t, f.provider.sources)
	})
	t.Run("card entry off", func(t *testing.T) {
		f := newBillingServiceFixture(t, false, false)
		f.trialWithCard(testOrgA, "einvoicing", "monthly")

		_, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", newCardRequest())

		assert.ErrorIs(t, err, domainError.ErrCardEntryOff)
	})
	t.Run("charging off saves the card and charges nothing", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, false)
		f.trialWithCard(testOrgA, "einvoicing", "monthly")
		f.at(day)

		sub, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", newCardRequest())

		require.NoError(t, err)
		assert.Equal(t, "5100", sub.Card.LastFour)
		assert.Empty(t, f.provider.requests)
	})
	t.Run("a charge already waiting is not doubled", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.trialWithCard(testOrgA, "einvoicing", "monthly")
		f.chargeDue(t)
		f.at(time.Hour)

		sub, err := f.service.ChangeCard(context.Background(), testOrgA, "rosa-id", newCardRequest())

		require.NoError(t, err)
		assert.Equal(t, "5100", sub.Card.LastFour)
		assert.Len(t, f.provider.requests, 1)
	})
}

func TestBillingService_CardSetup(t *testing.T) {
	on, err := newBillingServiceFixture(t, true, true).service.CardSetup(context.Background())
	require.NoError(t, err)
	assert.True(t, on.CardEntry)
	assert.Equal(t, &dto.BillingSetupWompi{PublicKey: "pub_test_abc", APIURL: "https://sandbox.wompi.co/v1",
		TermsURL: "https://wompi.example/terms.pdf", PersonalDataURL: "https://wompi.example/data.pdf",
		AcceptanceToken: "acc-fresh", PersonalDataAcceptanceToken: "pda-fresh"}, on.Wompi)

	off, err := newBillingServiceFixture(t, false, false).service.CardSetup(context.Background())
	require.NoError(t, err)
	assert.False(t, off.CardEntry)
	assert.Nil(t, off.Wompi)
}

// "Cancelling on day 10 of the trial" and "Changing their mind".
func TestBillingService_CancelAndResume_DuringTheTrial(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")

	f.at(-4 * day)
	cancelled, err := f.service.Cancel(context.Background(), testOrgA)
	require.NoError(t, err)
	assert.True(t, cancelled.Cancelled)
	assert.Nil(t, cancelled.NextChargeAmount, "nothing will be charged")
	assert.NotNil(t, cancelled.Card, "the card is kept")
	assert.True(t, cancelled.AccessEndsAt.Equal(chargeDue), "it works until the due date, with no grace")
	assert.Equal(t, []string{testOrgA}, f.access.dropped)

	again, err := f.service.Cancel(context.Background(), testOrgA)
	require.NoError(t, err)
	assert.True(t, again.Cancelled)

	f.at(0)
	f.chargeDue(t)
	assert.Empty(t, f.provider.requests, "no charge is made for a cancelled subscription")
	assert.ErrorIs(t, StoppedReason(&dto.OrganizationAccess{Status: "trial", TrialEndsAt: f.store.orgs[testOrgA].TrialEndsAt,
		HasCard: true, CancelledAt: f.store.billing[testOrgA].CancelledAt}, f.now), domainError.ErrTrialExpired, "and it stops on day 14")

	f.at(-2 * day)
	resumed, err := f.service.Resume(context.Background(), testOrgA)
	require.NoError(t, err)
	assert.False(t, resumed.Cancelled)
	require.NotNil(t, resumed.NextChargeAmount)

	f.at(0)
	f.chargeDue(t)
	assert.Len(t, f.provider.requests, 1, "the first charge is made on day 14 as if they had never cancelled")
}

func TestBillingService_Resume_AfterTheDueDateIsLapsed(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.at(-day)
	_, err := f.service.Cancel(context.Background(), testOrgA)
	require.NoError(t, err)

	f.at(0)
	_, err = f.service.Resume(context.Background(), testOrgA)

	assert.ErrorIs(t, err, domainError.ErrSubscriptionLapsed)
	assert.NotNil(t, f.store.billing[testOrgA].CancelledAt)
}

func TestBillingService_Resume_WhenNotCancelledChangesNothing(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.at(-day)

	sub, err := f.service.Resume(context.Background(), testOrgA)

	require.NoError(t, err)
	assert.False(t, sub.Cancelled)
}

func TestBillingService_CancelAndResume_WithoutACardIsNoSubscription(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.store.orgs[testOrgA] = &dto.Organization{ID: testOrgA, Status: "active", Plan: "full"}
	f.store.orgs[testOrgB] = &dto.Organization{ID: testOrgB, Status: "trial", Plan: "full"}
	f.store.billing[testOrgB] = &dto.OrganizationBilling{OrganizationID: testOrgB, Period: "monthly"}

	for _, orgID := range []string{testOrgA, testOrgB} {
		_, err := f.service.Cancel(context.Background(), orgID)
		assert.ErrorIs(t, err, domainError.ErrNoSubscription)
		_, err = f.service.Resume(context.Background(), orgID)
		assert.ErrorIs(t, err, domainError.ErrNoSubscription)
	}
}

func (f *billingServiceFixture) stoppedAfterThreeFailedRenewals(t *testing.T) {
	t.Helper()
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.store.orgs[testOrgA].Name = "Doña Rosa"
	f.provider.answer = dto.BillingChargeDeclined
	for _, after := range []time.Duration{0, day, 3 * day} {
		f.at(after)
		f.chargeDue(t)
	}
	require.Len(t, f.provider.requests, 3)
	f.at(5 * day)
}

func TestBillingService_RecoveryState_ShowsWhatIsOwed(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.stoppedAfterThreeFailedRenewals(t)

	state, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")

	require.NoError(t, err)
	assert.Nil(t, signIn)
	assert.Equal(t, "Doña Rosa", state.OrganizationName)
	assert.Equal(t, "payment_required", state.Reason)
	assert.Equal(t, "einvoicing", state.Plan)
	assert.Equal(t, "monthly", state.Period)
	assert.Equal(t, 40000, state.AmountDue)
	assert.Equal(t, &dto.SavedCard{Brand: "VISA", LastFour: "4242"}, state.Card)
	require.NotNil(t, state.LatestCharge)
	assert.Equal(t, "declined", state.LatestCharge.Status)
	assert.True(t, state.CardEntry)
	require.NotNil(t, state.Wompi)
	assert.Equal(t, "acc-fresh", state.Wompi.AcceptanceToken)
	body, _ := json.Marshal(state)
	assert.NotContains(t, string(body), "3891")
}

func TestBillingService_RecoveryState_ExpiredTrialWithoutACard(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	end := chargeDue
	f.store.orgs[testOrgA] = &dto.Organization{ID: testOrgA, Name: "Doña Rosa", Status: "trial", Plan: "starter", TrialEndsAt: &end}
	f.at(7 * day)

	state, _, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")

	require.NoError(t, err)
	assert.Equal(t, "trial_expired", state.Reason)
	assert.Equal(t, 25000, state.AmountDue)
	assert.Equal(t, "monthly", state.Period)
	assert.Nil(t, state.Card)
	assert.Nil(t, state.LatestCharge)
}

func TestBillingService_RecoveryState_WithoutChargingShowsNoCardEntry(t *testing.T) {
	for _, cardEntry := range []bool{false, true} {
		f := newBillingServiceFixture(t, cardEntry, false)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.at(7 * day)

		state, _, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")

		require.NoError(t, err)
		assert.False(t, state.CardEntry, "a card that cannot be charged pays nothing")
		assert.Nil(t, state.Wompi)
	}
}

// "The saved card works again", and the sign-in body returned once after approval.
func TestBillingService_RecoveryPay_WithTheSavedCardSignsInOnce(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.stoppedAfterThreeFailedRenewals(t)
	f.provider.answer = dto.BillingChargePending

	paid, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})
	require.NoError(t, err)
	assert.Equal(t, "pending", paid.Charge.Status)
	assert.Equal(t, 40000, paid.Charge.Amount)
	assert.Equal(t, "3891", f.provider.requests[3].PaymentSourceID)
	assert.Empty(t, f.provider.sources, "no new card was sent")

	state, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")
	require.NoError(t, err)
	assert.Nil(t, signIn, "still stopped while the charge is pending")
	assert.Equal(t, "pending", state.LatestCharge.Status)

	_, err = f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})
	assert.ErrorIs(t, err, domainError.ErrChargePending)
	assert.Len(t, f.provider.requests, 4)

	f.report(t, f.lastCharge(t), dto.BillingChargeApproved)
	assert.Equal(t, "active", f.store.orgs[testOrgA].Status)

	state, signIn, err = f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")
	require.NoError(t, err)
	assert.Nil(t, state)
	require.NotNil(t, signIn, "the same body as a successful sign-in")
	assert.NotEmpty(t, signIn.Token)
	assert.Equal(t, "rosa@example.com", signIn.Username)
	claims, err := f.users.jwt.ValidateToken(signIn.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
	assert.Equal(t, "rosa-id", claims.UserID)

	_, again, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")
	assert.Nil(t, again, "once")
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotStopped)
}

// "Paying after the trial ended without a card": starter, monthly, a new card, a week after the end.
func TestBillingService_RecoveryPay_NewCardAfterATrialWithoutOne(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	end := chargeDue
	f.store.orgs[testOrgA] = &dto.Organization{ID: testOrgA, Status: "trial", Plan: "starter", TrialEndsAt: &end}
	f.provider.answer = dto.BillingChargeApproved
	f.at(7 * day)

	paid, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id",
		&dto.RecoveryPayRequest{CardToken: "tok_test_new", AcceptanceToken: "acc-1", PersonalDataAcceptanceToken: "acc-2"})

	require.NoError(t, err)
	assert.Equal(t, "approved", paid.Charge.Status)
	assert.Equal(t, 25000, paid.Charge.Amount)
	assert.Equal(t, "active", f.store.orgs[testOrgA].Status)
	approvedAt := chargeDue.Add(7 * day)
	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(AddBillingPeriod(approvedAt, "monthly")), "paid for one month from the approval")
	assert.Equal(t, "5001", *f.store.billing[testOrgA].PaymentSourceID)

	_, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")
	require.NoError(t, err)
	require.NotNil(t, signIn, "the admin is signed in")
}

// "Declined again": the organization stays stopped and the state shows the declined charge.
func TestBillingService_RecoveryPay_DeclinedStaysStopped(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.stoppedAfterThreeFailedRenewals(t)

	paid, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})
	require.NoError(t, err)
	assert.Equal(t, "declined", paid.Charge.Status)

	state, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")
	require.NoError(t, err)
	assert.Nil(t, signIn)
	assert.Equal(t, "payment_required", state.Reason)
	assert.Equal(t, "declined", state.LatestCharge.Status)

	_, err = f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})
	require.NoError(t, err, "another try is allowed once the last one is settled")
}

func TestBillingService_RecoveryPay_Refusals(t *testing.T) {
	t.Run("without a saved card and without a new one names the card token", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		end := chargeDue
		f.store.orgs[testOrgA] = &dto.Organization{ID: testOrgA, Status: "trial", Plan: "starter", TrialEndsAt: &end}
		f.at(7 * day)

		_, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})

		var validation *domainError.BillingValidationError
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, "card_token", validation.Field)
		assert.Empty(t, f.provider.requests)
	})
	t.Run("a working organization is not stopped", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.at(-day)

		_, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})

		assert.ErrorIs(t, err, domainError.ErrOrganizationNotStopped)
		assert.Empty(t, f.provider.requests)
	})
	t.Run("a new card that is refused charges nothing and keeps the old one", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.stoppedAfterThreeFailedRenewals(t)
		f.provider.sourceErr = fmt.Errorf("%w: 422", domainError.ErrCardRefused)

		_, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id",
			&dto.RecoveryPayRequest{CardToken: "tok_test_bad", AcceptanceToken: "acc-1", PersonalDataAcceptanceToken: "acc-2"})

		require.ErrorIs(t, err, domainError.ErrCardRefused)
		assert.Len(t, f.provider.requests, 3)
		assert.Equal(t, "3891", *f.store.billing[testOrgA].PaymentSourceID)
	})
	t.Run("charging off", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, false)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.at(7 * day)

		_, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})

		assert.ErrorIs(t, err, domainError.ErrChargingOff)
	})
}

func TestBillingService_RecoveryState_AWorkingOrganizationThatNeverPaidIsNotStopped(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "starter", "monthly")
	f.at(-day)

	state, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")

	assert.Nil(t, state)
	assert.Nil(t, signIn, "a setup token is no sign-in for an organization that simply works")
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotStopped)
}

func TestBillingService_RecoveryState_AScheduledApprovalSignsNobodyIn(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.trialWithCard(testOrgA, "starter", "monthly")
	f.provider.answer = dto.BillingChargeApproved
	f.chargeDue(t)

	_, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")

	assert.Nil(t, signIn)
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotStopped)
}

func TestBillingService_RecoveryState_AnOldApprovalNoLongerSignsIn(t *testing.T) {
	f := newBillingServiceFixture(t, true, true)
	f.stoppedAfterThreeFailedRenewals(t)
	f.provider.answer = dto.BillingChargeApproved
	_, err := f.service.RecoveryPay(context.Background(), testOrgA, "rosa-id", &dto.RecoveryPayRequest{})
	require.NoError(t, err)

	f.now = f.now.Add(2 * time.Hour)
	_, signIn, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")

	assert.Nil(t, signIn)
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotStopped)
}

func TestBillingService_RecoverySelect(t *testing.T) {
	t.Run("changes plan and period of a stopped organization", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.stoppedAfterThreeFailedRenewals(t)

		selection, err := f.service.RecoverySelect(context.Background(), testOrgA, &dto.BillingSelectionRequest{Plan: "starter", Period: "annual"})

		require.NoError(t, err)
		assert.Equal(t, &dto.BillingSelection{Plan: "starter", Period: "annual", Amount: 250000, Currency: "COP"}, selection)
		assert.Equal(t, "annual", f.store.billing[testOrgA].Period)

		state, _, err := f.service.RecoveryState(context.Background(), testOrgA, "rosa-id")
		require.NoError(t, err)
		assert.Equal(t, "annual", state.Period)
	})
	t.Run("refuses a working organization", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.at(-day)

		_, err := f.service.RecoverySelect(context.Background(), testOrgA, &dto.BillingSelectionRequest{Plan: "full", Period: "annual"})

		assert.ErrorIs(t, err, domainError.ErrOrganizationNotStopped)
		assert.Equal(t, "monthly", f.store.billing[testOrgA].Period)
	})
	t.Run("validates as signup does", func(t *testing.T) {
		f := newBillingServiceFixture(t, true, true)
		f.stoppedAfterThreeFailedRenewals(t)

		_, err := f.service.RecoverySelect(context.Background(), testOrgA, &dto.BillingSelectionRequest{Plan: "gold", Period: "annual"})

		var validation *domainError.BillingValidationError
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, "plan", validation.Field)
	})
}
