package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chargeStore is billing and organizations in memory, with the two rules the database enforces:
// one open charge per organization and one approved charge per due date.
type chargeStore struct {
	ports.OrganizationRepository
	orgs    map[string]*dto.Organization
	billing map[string]*dto.OrganizationBilling
	emails  map[string]string
	charges []*dto.BillingCharge
	claimed map[string]bool
}

func newChargeStore() *chargeStore {
	return &chargeStore{orgs: map[string]*dto.Organization{}, billing: map[string]*dto.OrganizationBilling{}, emails: map[string]string{},
		claimed: map[string]bool{}}
}

func (s *chargeStore) Get(_ context.Context, id string) (*dto.Organization, error) {
	org, ok := s.orgs[id]
	if !ok {
		return nil, domainError.ErrOrganizationNotFound
	}
	copied := *org
	return &copied, nil
}

func (s *chargeStore) Update(_ context.Context, id string, req *dto.UpdateOrganizationRequest) (*dto.Organization, error) {
	org, ok := s.orgs[id]
	if !ok {
		return nil, domainError.ErrOrganizationNotFound
	}
	if req.Status != nil {
		org.Status = *req.Status
	}
	return org, nil
}

type chargeBilling struct{ *chargeStore }

func (s chargeBilling) Get(_ context.Context, organizationID string) (*dto.OrganizationBilling, bool, error) {
	row, ok := s.billing[organizationID]
	if !ok {
		return nil, false, nil
	}
	copied := *row
	return &copied, true, nil
}

func (s chargeBilling) GetMany(context.Context, []string) (map[string]*dto.OrganizationBilling, error) {
	return nil, errors.New("not used")
}

func (s chargeBilling) SetPeriod(_ context.Context, organizationID, period string) error {
	if row, ok := s.billing[organizationID]; ok {
		row.Period = period
		return nil
	}
	s.billing[organizationID] = &dto.OrganizationBilling{OrganizationID: organizationID, Period: period}
	return nil
}

func (s chargeBilling) SaveCard(_ context.Context, organizationID string, card dto.BillingCard) error {
	row, ok := s.billing[organizationID]
	if !ok {
		row = &dto.OrganizationBilling{OrganizationID: organizationID, Period: "monthly"}
		s.billing[organizationID] = row
	}
	row.Provider, row.PaymentSourceID, row.CardBrand, row.CardLastFour = &card.Provider, &card.PaymentSourceID, &card.Brand, &card.LastFour
	return nil
}

func (s chargeBilling) hasOpenCharge(organizationID string) bool {
	for _, c := range s.charges {
		if c.OrganizationID == organizationID && c.Status.Open() {
			return true
		}
	}
	return false
}

func (s chargeBilling) LockDue(_ context.Context, now time.Time) ([]dto.DueSubscription, error) {
	var due []dto.DueSubscription
	for id, row := range s.billing {
		org := s.orgs[id]
		if !row.HasCard() || row.Attempts >= 3 {
			continue
		}
		trial := org.Status == dto.OrganizationStatusTrial && row.PaidUntil == nil && org.TrialEndsAt != nil
		paid := org.Status == dto.OrganizationStatusActive && row.PaidUntil != nil
		if !trial && !paid {
			continue
		}
		dueOn, _ := DueDate(org, row)
		if dueOn.After(now) {
			continue
		}
		due = append(due, dto.DueSubscription{OrganizationID: id, OrganizationStatus: org.Status, Plan: org.Plan, Period: row.Period,
			PaymentSourceID: *row.PaymentSourceID, CustomerEmail: s.emails[id], DueOn: dueOn, Attempts: row.Attempts,
			CancelledAt: row.CancelledAt, HasOpenCharge: s.hasOpenCharge(id)})
	}
	sort.Slice(due, func(i, j int) bool { return due[i].OrganizationID < due[j].OrganizationID })
	return due, nil
}

func (s chargeBilling) LockSubscription(ctx context.Context, organizationID string) (*dto.OrganizationBilling, bool, error) {
	return s.Get(ctx, organizationID)
}

func (s chargeBilling) InsertCharge(_ context.Context, charge *dto.BillingCharge) error {
	if charge.Status.Open() && s.hasOpenCharge(charge.OrganizationID) {
		return domainError.ErrChargePending
	}
	for _, c := range s.charges {
		if c.Reference == charge.Reference {
			return errors.New("duplicate reference")
		}
	}
	copied := *charge
	s.charges = append(s.charges, &copied)
	return nil
}

func (s chargeBilling) UpdateCharge(_ context.Context, charge *dto.BillingCharge) error {
	for i, c := range s.charges {
		if c.ID != charge.ID {
			continue
		}
		if charge.Status == dto.BillingChargeApproved {
			for _, other := range s.charges {
				if other.ID != c.ID && other.OrganizationID == c.OrganizationID && other.Status == dto.BillingChargeApproved && other.DueOn.Equal(c.DueOn) {
					return errors.New("a second approved charge for the due date")
				}
			}
		}
		copied := *charge
		s.charges[i] = &copied
		return nil
	}
	return errors.New("charge not found")
}

func (s chargeBilling) IncrementAttempts(_ context.Context, organizationID string) error {
	s.billing[organizationID].Attempts++
	return nil
}

func (s chargeBilling) LockChargeByReference(_ context.Context, reference string) (*dto.BillingCharge, bool, error) {
	for _, c := range s.charges {
		if c.Reference == reference {
			copied := *c
			return &copied, true, nil
		}
	}
	return nil, false, nil
}

func (s chargeBilling) FindChargeByTransactionID(_ context.Context, transactionID string) (*dto.BillingCharge, bool, error) {
	for _, c := range s.charges {
		if c.TransactionID != nil && *c.TransactionID == transactionID {
			copied := *c
			return &copied, true, nil
		}
	}
	return nil, false, nil
}

func (s chargeBilling) OpenChargesCreatedBefore(_ context.Context, before time.Time) ([]*dto.BillingCharge, error) {
	var open []*dto.BillingCharge
	for _, c := range s.charges {
		if c.Status.Open() && c.CreatedAt.Before(before) {
			copied := *c
			open = append(open, &copied)
		}
	}
	return open, nil
}

func (s chargeBilling) ListCharges(_ context.Context, organizationID string) ([]*dto.BillingCharge, error) {
	var list []*dto.BillingCharge
	for i := len(s.charges) - 1; i >= 0; i-- {
		if s.charges[i].OrganizationID == organizationID {
			copied := *s.charges[i]
			list = append(list, &copied)
		}
	}
	return list, nil
}

func (s chargeBilling) MarkPaid(_ context.Context, organizationID string, paidUntil time.Time, resume bool) error {
	row := s.billing[organizationID]
	row.PaidUntil, row.Attempts = &paidUntil, 0
	if resume {
		row.CancelledAt = nil
	}
	return nil
}

func (s chargeBilling) SetCancelled(_ context.Context, organizationID string, cancelledAt *time.Time) error {
	s.billing[organizationID].CancelledAt = cancelledAt
	return nil
}

func (s chargeBilling) ClaimRecoverySignIn(_ context.Context, organizationID string, approvedSince, _ time.Time) (bool, error) {
	for i := len(s.charges) - 1; i >= 0; i-- {
		c := s.charges[i]
		if c.OrganizationID == organizationID && c.Attempt == 0 && c.Status == dto.BillingChargeApproved &&
			!s.claimed[c.ID] && !c.UpdatedAt.Before(approvedSince) {
			s.claimed[c.ID] = true
			return true, nil
		}
	}
	return false, nil
}

func (s chargeBilling) EndSubscription(_ context.Context, organizationID string, cancelledAt time.Time) error {
	row := s.billing[organizationID]
	row.PaidUntil, row.Attempts, row.CancelledAt = nil, 0, &cancelledAt
	return nil
}

// chargeProvider is the provider: it answers every charge with `answer` and remembers what it took.
type chargeProvider struct {
	ports.BillingProvider
	answer    dto.BillingChargeStatus
	chargeErr error
	lookupErr error
	requests  []dto.BillingChargeRequest
	taken     map[string]dto.BillingTransaction
	sourceErr error
	sources   []dto.PaymentSourceRequest
}

func (p *chargeProvider) Terms(context.Context) (dto.WompiMerchantTerms, error) {
	return dto.WompiMerchantTerms{TermsURL: "https://wompi.example/terms.pdf", PersonalDataURL: "https://wompi.example/data.pdf",
		AcceptanceToken: "acc-fresh", PersonalAuthToken: "pda-fresh"}, nil
}

func (p *chargeProvider) SavePaymentSource(_ context.Context, req dto.PaymentSourceRequest) (dto.PaymentSource, error) {
	p.sources = append(p.sources, req)
	if p.sourceErr != nil {
		return dto.PaymentSource{}, p.sourceErr
	}
	return dto.PaymentSource{ID: fmt.Sprintf("%d", 5000+len(p.sources)), Brand: "MASTERCARD", LastFour: "5100"}, nil
}

func (p *chargeProvider) Charge(_ context.Context, req dto.BillingChargeRequest) (dto.BillingTransaction, error) {
	p.requests = append(p.requests, req)
	if p.chargeErr != nil {
		return dto.BillingTransaction{}, p.chargeErr
	}
	tx := dto.BillingTransaction{ID: fmt.Sprintf("tx_%d", len(p.requests)), Reference: req.Reference, Status: p.answer,
		AmountInCents: req.AmountInCents, Currency: req.Currency}
	p.taken[tx.ID] = tx
	return tx, nil
}

func (p *chargeProvider) Transaction(_ context.Context, id string) (dto.BillingTransaction, error) {
	if p.lookupErr != nil {
		return dto.BillingTransaction{}, p.lookupErr
	}
	tx, ok := p.taken[id]
	if !ok {
		return dto.BillingTransaction{}, domainError.ErrBillingTransactionNotFound
	}
	return tx, nil
}

func (p *chargeProvider) TransactionByReference(_ context.Context, reference string) (dto.BillingTransaction, error) {
	if p.lookupErr != nil {
		return dto.BillingTransaction{}, p.lookupErr
	}
	for _, tx := range p.taken {
		if tx.Reference == reference {
			return tx, nil
		}
	}
	return dto.BillingTransaction{}, domainError.ErrBillingTransactionNotFound
}

type recordingInvalidator struct{ dropped []string }

func (r *recordingInvalidator) InvalidateAccess(_ context.Context, orgID string) {
	r.dropped = append(r.dropped, orgID)
}

type chargeFixture struct {
	store    *chargeStore
	provider *chargeProvider
	access   *recordingInvalidator
	logs     *bytes.Buffer
	service  *ChargeService
	now      time.Time
}

// chargeDue is the trial's end in "Monthly trial ends": 2026-12-15.
var chargeDue = time.Date(2026, 12, 15, 14, 0, 0, 0, time.UTC)

func newChargeFixture(t *testing.T) *chargeFixture {
	t.Helper()
	f := &chargeFixture{store: newChargeStore(), provider: &chargeProvider{answer: dto.BillingChargePending, taken: map[string]dto.BillingTransaction{}},
		access: &recordingInvalidator{}, logs: &bytes.Buffer{}, now: chargeDue}
	f.service = NewChargeService(chargeBilling{f.store}, f.store, f.provider, passthroughUnitOfWork{}, f.access,
		slog.New(slog.NewTextHandler(f.logs, nil)))
	f.service.now = func() time.Time { return f.now }
	suffixes := 0
	f.service.newSuffix = func() string {
		suffixes++
		if suffixes == 1 {
			return "r4nd0m"
		}
		return fmt.Sprintf("r4nd0m%d", suffixes)
	}
	return f
}

func (f *chargeFixture) trialWithCard(orgID, plan, period string) {
	end := chargeDue
	source, provider, brand, last := "3891", "wompi", "VISA", "4242"
	f.store.orgs[orgID] = &dto.Organization{ID: orgID, Status: dto.OrganizationStatusTrial, Plan: plan, TrialEndsAt: &end}
	f.store.billing[orgID] = &dto.OrganizationBilling{OrganizationID: orgID, Period: period, Provider: &provider, PaymentSourceID: &source,
		CardBrand: &brand, CardLastFour: &last}
	f.store.emails[orgID] = "rosa@example.com"
}

func (f *chargeFixture) paidUntil(orgID, plan, period string, paidUntil time.Time) {
	f.trialWithCard(orgID, plan, period)
	f.store.orgs[orgID].Status = dto.OrganizationStatusActive
	f.store.billing[orgID].PaidUntil = &paidUntil
}

func (f *chargeFixture) at(d time.Duration) { f.now = chargeDue.Add(d) }

func (f *chargeFixture) chargeDue(t *testing.T) {
	t.Helper()
	require.NoError(t, f.service.ChargeDue(context.Background()))
}

func (f *chargeFixture) report(t *testing.T, charge *dto.BillingCharge, status dto.BillingChargeStatus) {
	t.Helper()
	require.NoError(t, f.service.Settle(context.Background(), dto.BillingTransaction{ID: *charge.TransactionID, Reference: charge.Reference,
		Status: status, AmountInCents: int64(charge.Amount) * 100, Currency: charge.Currency}))
}

func (f *chargeFixture) lastCharge(t *testing.T) *dto.BillingCharge {
	t.Helper()
	require.NotEmpty(t, f.store.charges)
	return f.store.charges[len(f.store.charges)-1]
}

const day = 24 * time.Hour

// "Monthly trial ends": einvoicing, monthly, trial ends 2026-12-15 with a saved card.
func TestChargeService_FirstCharge_MonthlyTrialEnds(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")

	f.at(-time.Minute)
	f.chargeDue(t)
	require.Empty(t, f.provider.requests, "nothing is charged before the trial ends")

	f.at(0)
	f.chargeDue(t)

	require.Len(t, f.provider.requests, 1)
	req := f.provider.requests[0]
	assert.Equal(t, dto.BillingChargeRequest{PaymentSourceID: "3891", AmountInCents: 4000000, Currency: "COP",
		Reference: "kb-" + testOrgA + "-20261215-1", CustomerEmail: "rosa@example.com"}, req)
	charge := f.lastCharge(t)
	assert.Equal(t, dto.BillingChargePending, charge.Status)
	assert.Equal(t, 40000, charge.Amount)
	assert.Equal(t, 1, charge.Attempt)
	assert.Equal(t, dto.OrganizationStatusTrial, f.store.orgs[testOrgA].Status, "only an approval changes the organization")

	f.report(t, charge, dto.BillingChargeApproved)

	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
	paidUntil := f.store.billing[testOrgA].PaidUntil
	require.NotNil(t, paidUntil)
	assert.True(t, paidUntil.Equal(time.Date(2027, 1, 15, 14, 0, 0, 0, time.UTC)), "paid until %s", paidUntil)
	settled := f.lastCharge(t)
	assert.Equal(t, dto.BillingChargeApproved, settled.Status)
	assert.True(t, settled.CoversFrom.Equal(chargeDue), "the paid period runs from the trial's end")
	assert.True(t, settled.CoversUntil.Equal(*paidUntil))
	assert.Equal(t, []string{testOrgA}, f.access.dropped, "the cached access is dropped so the payment applies at once")
}

func TestChargeService_FirstCharge_AnnualTrialEnds(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "full", "annual")

	f.chargeDue(t)
	require.Len(t, f.provider.requests, 1)
	assert.Equal(t, int64(100000000), f.provider.requests[0].AmountInCents)
	f.report(t, f.lastCharge(t), dto.BillingChargeApproved)

	assert.Equal(t, 1000000, f.lastCharge(t).Amount)
	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(time.Date(2027, 12, 15, 14, 0, 0, 0, time.UTC)))
}

func TestChargeService_FirstCharge_ApprovedInTheAnswerSettlesAtOnce(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "starter", "monthly")
	f.provider.answer = dto.BillingChargeApproved

	f.chargeDue(t)

	assert.Equal(t, dto.BillingChargeApproved, f.lastCharge(t).Status)
	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
}

// "Renewal paid two days late": paid until 2027-01-15, approved on 2027-01-17, paid until 2027-02-15.
func TestChargeService_Renewal_ApprovedLateExtendsFromTheDueDate(t *testing.T) {
	f := newChargeFixture(t)
	due := time.Date(2027, 1, 15, 14, 0, 0, 0, time.UTC)
	f.paidUntil(testOrgA, "einvoicing", "monthly", due)

	f.now = due
	f.chargeDue(t)
	charge := f.lastCharge(t)
	assert.Equal(t, "kb-"+testOrgA+"-20270115-1", charge.Reference)

	f.now = due.Add(2 * day)
	f.report(t, charge, dto.BillingChargeApproved)

	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(time.Date(2027, 2, 15, 14, 0, 0, 0, time.UTC)))
	assert.True(t, f.lastCharge(t).CoversFrom.Equal(due), "from the previous due date, not the day of the payment")
	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
}

// "Plan changed during the period": the renewal is the price of the plan at that moment.
func TestChargeService_Renewal_ChargesThePlanOfThatMoment(t *testing.T) {
	f := newChargeFixture(t)
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.store.orgs[testOrgA].Plan = "full"

	f.chargeDue(t)

	assert.Equal(t, int64(10000000), f.provider.requests[0].AmountInCents)
	assert.Equal(t, 100000, f.lastCharge(t).Amount)
	assert.Equal(t, "full", f.lastCharge(t).Plan)
}

func TestChargeService_Settle_ApprovedWithTheRightAmount(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)
	charge := f.lastCharge(t)

	err := f.service.Settle(context.Background(), dto.BillingTransaction{ID: *charge.TransactionID, Reference: charge.Reference,
		Status: dto.BillingChargeApproved, AmountInCents: 4000000, Currency: "COP"})

	require.NoError(t, err)
	assert.Equal(t, dto.BillingChargeApproved, f.lastCharge(t).Status)
	assert.NotNil(t, f.store.billing[testOrgA].PaidUntil)
}

func TestChargeService_Settle_ApprovedWithAWrongAmountOrCurrencyIsNotApplied(t *testing.T) {
	cases := []struct {
		name     string
		cents    int64
		currency string
	}{
		{"wrong amount", 100000, "COP"},
		{"wrong currency", 4000000, "USD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newChargeFixture(t)
			f.trialWithCard(testOrgA, "einvoicing", "monthly")
			f.chargeDue(t)
			charge := f.lastCharge(t)

			err := f.service.Settle(context.Background(), dto.BillingTransaction{ID: *charge.TransactionID, Reference: charge.Reference,
				Status: dto.BillingChargeApproved, AmountInCents: tc.cents, Currency: tc.currency})

			require.NoError(t, err)
			assert.Equal(t, dto.BillingChargePending, f.lastCharge(t).Status)
			assert.Equal(t, dto.OrganizationStatusTrial, f.store.orgs[testOrgA].Status)
			assert.Nil(t, f.store.billing[testOrgA].PaidUntil)
			assert.Empty(t, f.access.dropped)
			logged := f.logs.String()
			assert.Contains(t, logged, "level=ERROR")
			assert.Contains(t, logged, testOrgA)
			assert.Contains(t, logged, "charge_amount_in_cents=4000000")
			assert.Contains(t, logged, fmt.Sprintf("reported_amount_in_cents=%d", tc.cents))
		})
	}
}

func TestChargeService_Settle_ReportedTwiceExtendsOnce(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)
	charge := f.lastCharge(t)

	f.report(t, charge, dto.BillingChargeApproved)
	first := *f.store.billing[testOrgA].PaidUntil
	f.at(5 * day)
	f.report(t, charge, dto.BillingChargeApproved)

	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(first))
	assert.Len(t, f.access.dropped, 1)
}

func TestChargeService_Settle_OnlyAnApprovalChangesTheOrganization(t *testing.T) {
	for _, status := range []dto.BillingChargeStatus{dto.BillingChargeDeclined, dto.BillingChargeVoided, dto.BillingChargeError} {
		t.Run(string(status), func(t *testing.T) {
			f := newChargeFixture(t)
			f.trialWithCard(testOrgA, "einvoicing", "monthly")
			f.chargeDue(t)

			f.report(t, f.lastCharge(t), status)

			assert.Equal(t, status, f.lastCharge(t).Status)
			assert.Equal(t, dto.OrganizationStatusTrial, f.store.orgs[testOrgA].Status)
			assert.Nil(t, f.store.billing[testOrgA].PaidUntil)
			assert.Contains(t, f.logs.String(), "subscription charge failed")
		})
	}
}

func TestChargeService_Settle_AFailedChargeIsNotApprovedLater(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)
	charge := f.lastCharge(t)
	f.report(t, charge, dto.BillingChargeDeclined)

	f.report(t, charge, dto.BillingChargeApproved)

	assert.Equal(t, dto.BillingChargeDeclined, f.lastCharge(t).Status)
	assert.Nil(t, f.store.billing[testOrgA].PaidUntil)
	assert.Contains(t, f.logs.String(), "refund it by hand")
}

func TestChargeService_Settle_UnknownReference(t *testing.T) {
	f := newChargeFixture(t)

	err := f.service.Settle(context.Background(), dto.BillingTransaction{ID: "tx_9", Reference: "someone-elses", Status: dto.BillingChargeApproved})

	assert.ErrorIs(t, err, domainError.ErrChargeNotFound)
}

func TestChargeService_Settle_AnotherTransactionForTheReferenceIsNotApplied(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)
	charge := f.lastCharge(t)

	err := f.service.Settle(context.Background(), dto.BillingTransaction{ID: "tx_forged", Reference: charge.Reference,
		Status: dto.BillingChargeApproved, AmountInCents: 4000000, Currency: "COP"})

	require.NoError(t, err)
	assert.Equal(t, dto.BillingChargePending, f.lastCharge(t).Status)
	assert.Nil(t, f.store.billing[testOrgA].PaidUntil)
}

func TestChargeService_Settle_ASuspendedOrganizationStaysSuspended(t *testing.T) {
	f := newChargeFixture(t)
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.chargeDue(t)
	f.store.orgs[testOrgA].Status = dto.OrganizationStatusSuspended

	f.report(t, f.lastCharge(t), dto.BillingChargeApproved)

	assert.Equal(t, dto.OrganizationStatusSuspended, f.store.orgs[testOrgA].Status)
	assert.NotNil(t, f.store.billing[testOrgA].PaidUntil)
}

// "Declined on the due day, approved the next": paid from the due date.
func TestChargeService_ChargeDue_RetriesOnTheDueDateOneDayAndThreeDaysLater(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.answer = dto.BillingChargeDeclined

	f.at(0)
	f.chargeDue(t)
	require.Len(t, f.provider.requests, 1)

	f.at(10 * time.Minute)
	f.chargeDue(t)
	f.at(day - time.Minute)
	f.chargeDue(t)
	require.Len(t, f.provider.requests, 1, "no retry before one day has passed")

	f.at(day)
	f.chargeDue(t)
	require.Len(t, f.provider.requests, 2)

	f.at(2 * day)
	f.chargeDue(t)
	f.at(3*day - time.Minute)
	f.chargeDue(t)
	require.Len(t, f.provider.requests, 2, "no retry before three days have passed")

	f.at(3 * day)
	f.chargeDue(t)
	require.Len(t, f.provider.requests, 3)

	f.at(4 * day)
	f.chargeDue(t)
	f.at(30 * day)
	f.chargeDue(t)
	require.Len(t, f.provider.requests, 3, "no fourth charge")

	references := []string{f.provider.requests[0].Reference, f.provider.requests[1].Reference, f.provider.requests[2].Reference}
	assert.Equal(t, []string{"kb-" + testOrgA + "-20261215-1", "kb-" + testOrgA + "-20261215-2", "kb-" + testOrgA + "-20261215-3"}, references)
	assert.Equal(t, 3, f.store.billing[testOrgA].Attempts)
	assert.Equal(t, dto.OrganizationStatusTrial, f.store.orgs[testOrgA].Status)
}

func TestChargeService_ChargeDue_DeclinedThenApprovedTheNextDay(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.answer = dto.BillingChargeDeclined
	f.chargeDue(t)

	f.at(day)
	f.provider.answer = dto.BillingChargeApproved
	f.chargeDue(t)

	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
	assert.True(t, f.lastCharge(t).CoversFrom.Equal(chargeDue), "paid from the due date")
	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(time.Date(2027, 1, 15, 14, 0, 0, 0, time.UTC)))
	assert.Zero(t, f.store.billing[testOrgA].Attempts, "the next due date starts with three attempts again")
}

func TestChargeService_ChargeDue_NoneWhileOneIsOpen(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)

	f.at(day)
	f.chargeDue(t)
	f.at(3 * day)
	f.chargeDue(t)

	assert.Len(t, f.provider.requests, 1)
	assert.Equal(t, 1, f.store.billing[testOrgA].Attempts)
}

func TestChargeService_ChargeDue_WhoIsNeverCharged(t *testing.T) {
	cancelledAt := chargeDue.Add(-4 * day)
	cases := []struct {
		name   string
		mutate func(f *chargeFixture)
	}{
		{"cancelled before the trial ends", func(f *chargeFixture) { f.store.billing[testOrgA].CancelledAt = &cancelledAt }},
		{"suspended", func(f *chargeFixture) { f.store.orgs[testOrgA].Status = dto.OrganizationStatusSuspended }},
		{"no saved card", func(f *chargeFixture) {
			f.store.billing[testOrgA].PaymentSourceID, f.store.billing[testOrgA].Provider = nil, nil
		}},
		{"no billing at all", func(f *chargeFixture) { delete(f.store.billing, testOrgA) }},
		{"activated by hand, with a card and no paid period", func(f *chargeFixture) { f.store.orgs[testOrgA].Status = dto.OrganizationStatusActive }},
		{"pending", func(f *chargeFixture) {
			f.store.orgs[testOrgA].Status, f.store.orgs[testOrgA].TrialEndsAt = dto.OrganizationStatusPending, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newChargeFixture(t)
			f.trialWithCard(testOrgA, "einvoicing", "monthly")
			tc.mutate(f)

			for _, after := range []time.Duration{0, day, 3 * day, 40 * day} {
				f.at(after)
				f.chargeDue(t)
			}

			assert.Empty(t, f.provider.requests)
			assert.Empty(t, f.store.charges)
		})
	}
}

// chargeable is what the repository's query leaves to the service, so each rule is checked on its own.
func TestChargeable(t *testing.T) {
	cancelledAt := chargeDue
	base := dto.DueSubscription{OrganizationID: testOrgA, OrganizationStatus: "trial", PaymentSourceID: "3891", DueOn: chargeDue}
	cases := []struct {
		name   string
		mutate func(*dto.DueSubscription)
		after  time.Duration
		want   bool
	}{
		{"first attempt on the due date", func(*dto.DueSubscription) {}, 0, true},
		{"second attempt before a day", func(s *dto.DueSubscription) { s.Attempts = 1 }, day - time.Second, false},
		{"second attempt after a day", func(s *dto.DueSubscription) { s.Attempts = 1 }, day, true},
		{"third attempt before three days", func(s *dto.DueSubscription) { s.Attempts = 2 }, 3*day - time.Second, false},
		{"third attempt after three days", func(s *dto.DueSubscription) { s.Attempts = 2 }, 3 * day, true},
		{"after three attempts", func(s *dto.DueSubscription) { s.Attempts = 3 }, 30 * day, false},
		{"cancelled", func(s *dto.DueSubscription) { s.CancelledAt = &cancelledAt }, 0, false},
		{"suspended", func(s *dto.DueSubscription) { s.OrganizationStatus = "suspended" }, 0, false},
		{"without a card", func(s *dto.DueSubscription) { s.PaymentSourceID = "" }, 0, false},
		{"with an open charge", func(s *dto.DueSubscription) { s.HasOpenCharge = true }, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := base
			tc.mutate(&sub)
			assert.Equal(t, tc.want, chargeable(sub, chargeDue.Add(tc.after)))
		})
	}
}

func TestChargeService_ChargeDue_OneOrganizationsProblemDoesNotStopTheOthers(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.trialWithCard(testOrgB, "starter", "monthly")
	f.store.emails[testOrgA] = ""

	f.chargeDue(t)

	require.Len(t, f.provider.requests, 1)
	assert.Equal(t, int64(2500000), f.provider.requests[0].AmountInCents)
	assert.Zero(t, f.store.billing[testOrgA].Attempts, "an attempt that was never made is not counted")
	assert.Contains(t, f.logs.String(), "no admin with an email")
}

func TestChargeService_ChargeDue_ARefusedRequestIsAFailedAttempt(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.chargeErr = fmt.Errorf("%w: status 422", domainError.ErrCardRefused)

	f.chargeDue(t)

	assert.Equal(t, dto.BillingChargeError, f.lastCharge(t).Status)
	assert.Equal(t, 1, f.store.billing[testOrgA].Attempts)

	f.at(day)
	f.chargeDue(t)
	assert.Len(t, f.provider.requests, 2, "the next attempt is free")
}

// "Crash after Wompi accepted": the row stays created, and no second transaction is made for it.
func TestChargeService_ChargeDue_AnUnansweredCallLeavesTheChargeCreated(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.chargeErr = fmt.Errorf("%w: timeout", domainError.ErrPaymentProviderUnavailable)

	f.chargeDue(t)
	f.at(5 * time.Minute)
	f.chargeDue(t)

	assert.Equal(t, dto.BillingChargeCreated, f.lastCharge(t).Status)
	assert.Len(t, f.provider.requests, 1)
}

func TestChargeService_Reconcile_APendingChargeIsSettledFromTheLookup(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)
	charge := f.lastCharge(t)
	approved := f.provider.taken[*charge.TransactionID]
	approved.Status = dto.BillingChargeApproved
	f.provider.taken[approved.ID] = approved

	f.at(9 * time.Minute)
	require.NoError(t, f.service.Reconcile(context.Background()))
	assert.Equal(t, dto.BillingChargePending, f.lastCharge(t).Status, "a charge younger than ten minutes waits for its event")

	f.at(11 * time.Minute)
	require.NoError(t, f.service.Reconcile(context.Background()))

	assert.Equal(t, dto.BillingChargeApproved, f.lastCharge(t).Status)
	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
}

func TestChargeService_Reconcile_ACreatedRowWithATransactionAtTheProviderIsAdopted(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.chargeErr = fmt.Errorf("%w: timeout", domainError.ErrPaymentProviderUnavailable)
	f.chargeDue(t)
	charge := f.lastCharge(t)
	require.Equal(t, dto.BillingChargeCreated, charge.Status)
	f.provider.taken["tx_lost"] = dto.BillingTransaction{ID: "tx_lost", Reference: charge.Reference, Status: dto.BillingChargePending,
		AmountInCents: 4000000, Currency: "COP"}

	f.at(11 * time.Minute)
	require.NoError(t, f.service.Reconcile(context.Background()))

	adopted := f.lastCharge(t)
	assert.Equal(t, dto.BillingChargePending, adopted.Status)
	require.NotNil(t, adopted.TransactionID)
	assert.Equal(t, "tx_lost", *adopted.TransactionID)
	assert.Len(t, f.provider.requests, 1, "no second transaction for the attempt")

	tx := f.provider.taken["tx_lost"]
	tx.Status = dto.BillingChargeApproved
	f.provider.taken["tx_lost"] = tx
	f.at(16 * time.Minute)
	require.NoError(t, f.service.Reconcile(context.Background()))
	assert.Equal(t, dto.BillingChargeApproved, f.lastCharge(t).Status, "and it is settled by its reference")
}

func TestChargeService_Reconcile_ACreatedRowWithoutATransactionIsMarkedError(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.chargeErr = fmt.Errorf("%w: timeout", domainError.ErrPaymentProviderUnavailable)
	f.chargeDue(t)

	f.at(11 * time.Minute)
	require.NoError(t, f.service.Reconcile(context.Background()))

	assert.Equal(t, dto.BillingChargeError, f.lastCharge(t).Status)

	f.provider.chargeErr = nil
	f.at(day)
	f.chargeDue(t)
	assert.Len(t, f.provider.requests, 2, "the next attempt is free")
	assert.Equal(t, 2, f.lastCharge(t).Attempt)
}

func TestChargeService_Reconcile_AnOutageNeverFreesACharge(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.provider.chargeErr = fmt.Errorf("%w: timeout", domainError.ErrPaymentProviderUnavailable)
	f.chargeDue(t)
	f.provider.lookupErr = fmt.Errorf("%w: 502", domainError.ErrPaymentProviderUnavailable)

	f.at(2 * time.Hour)
	require.NoError(t, f.service.Reconcile(context.Background()))

	assert.Equal(t, dto.BillingChargeCreated, f.lastCharge(t).Status)
}

func TestChargeService_Reconcile_OpenFor24HoursIsLoggedAndCountedAsFailed(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "einvoicing", "monthly")
	f.chargeDue(t)

	f.at(23 * time.Hour)
	require.NoError(t, f.service.Reconcile(context.Background()))
	assert.Equal(t, dto.BillingChargePending, f.lastCharge(t).Status, "the lookup repeats until the charge is settled")
	assert.NotContains(t, f.logs.String(), "after 24 hours")

	f.at(24 * time.Hour)
	require.NoError(t, f.service.Reconcile(context.Background()))

	assert.Equal(t, dto.BillingChargeError, f.lastCharge(t).Status)
	logged := f.logs.String()
	assert.Contains(t, logged, "no final status after 24 hours")
	assert.Equal(t, 1, strings.Count(logged, "subscription charge failed"))

	f.at(25 * time.Hour)
	f.chargeDue(t)
	assert.Len(t, f.provider.requests, 2, "it counted as the first failure, so the second attempt follows")
}

func TestChargeService_ChargeNow_ANewCardInGraceChargesTheUnpaidPeriod(t *testing.T) {
	f := newChargeFixture(t)
	f.paidUntil(testOrgA, "einvoicing", "monthly", chargeDue)
	f.provider.answer = dto.BillingChargeDeclined
	f.chargeDue(t)
	f.provider.answer = dto.BillingChargeApproved

	f.at(day - time.Hour)
	charge, err := f.service.ChargeNow(context.Background(), testOrgA, "rosa@example.com")

	require.NoError(t, err)
	assert.Equal(t, 0, charge.Attempt)
	assert.Equal(t, "kb-"+testOrgA+"-20261215-r4nd0m", charge.Reference)
	assert.Equal(t, dto.BillingChargeApproved, charge.Status)
	assert.True(t, charge.CoversFrom.Equal(chargeDue), "an organization that never stopped pays from its due date")
	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(time.Date(2027, 1, 15, 14, 0, 0, 0, time.UTC)))
}

func TestChargeService_ChargeNow_RecoveryPaysFromTheApprovalAndResumes(t *testing.T) {
	f := newChargeFixture(t)
	f.paidUntil(testOrgA, "starter", "monthly", chargeDue)
	cancelledAt := chargeDue.Add(-10 * day)
	f.store.billing[testOrgA].CancelledAt = &cancelledAt
	f.store.billing[testOrgA].Attempts = 3
	f.provider.answer = dto.BillingChargeApproved

	f.at(20 * day)
	charge, err := f.service.ChargeNow(context.Background(), testOrgA, "rosa@example.com")

	require.NoError(t, err)
	approvedAt := chargeDue.Add(20 * day)
	assert.True(t, charge.CoversFrom.Equal(approvedAt), "a stopped organization pays from the approval")
	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(AddBillingPeriod(approvedAt, "monthly")))
	assert.Nil(t, f.store.billing[testOrgA].CancelledAt, "the cancellation is removed")
	assert.Zero(t, f.store.billing[testOrgA].Attempts)
	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
}

func TestChargeService_ChargeNow_RecoveryOfAnExpiredTrialMakesItActive(t *testing.T) {
	f := newChargeFixture(t)
	f.trialWithCard(testOrgA, "starter", "monthly")
	f.store.billing[testOrgA].Attempts = 3
	f.provider.answer = dto.BillingChargeApproved

	f.at(7 * day)
	charge, err := f.service.ChargeNow(context.Background(), testOrgA, "rosa@example.com")

	require.NoError(t, err)
	assert.Equal(t, 25000, charge.Amount)
	assert.Equal(t, dto.OrganizationStatusActive, f.store.orgs[testOrgA].Status)
	assert.True(t, f.store.billing[testOrgA].PaidUntil.Equal(AddBillingPeriod(chargeDue.Add(7*day), "monthly")))
}

func TestChargeService_ChargeNow_Refusals(t *testing.T) {
	t.Run("while a charge is pending", func(t *testing.T) {
		f := newChargeFixture(t)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.chargeDue(t)

		_, err := f.service.ChargeNow(context.Background(), testOrgA, "rosa@example.com")

		assert.ErrorIs(t, err, domainError.ErrChargePending)
		assert.Len(t, f.provider.requests, 1)
	})
	t.Run("without a saved card", func(t *testing.T) {
		f := newChargeFixture(t)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.store.billing[testOrgA].PaymentSourceID = nil

		_, err := f.service.ChargeNow(context.Background(), testOrgA, "rosa@example.com")

		assert.ErrorIs(t, err, domainError.ErrNoSubscription)
	})
	t.Run("declined", func(t *testing.T) {
		f := newChargeFixture(t)
		f.trialWithCard(testOrgA, "starter", "monthly")
		f.provider.answer = dto.BillingChargeDeclined
		f.at(7 * day)

		charge, err := f.service.ChargeNow(context.Background(), testOrgA, "rosa@example.com")

		require.NoError(t, err)
		assert.Equal(t, dto.BillingChargeDeclined, charge.Status)
		assert.Equal(t, dto.OrganizationStatusTrial, f.store.orgs[testOrgA].Status)
	})
}

func TestAddBillingPeriod(t *testing.T) {
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 9, 30, 0, 0, time.UTC) }
	cases := []struct {
		name   string
		from   time.Time
		period string
		want   time.Time
	}{
		{"a month", at(2026, 12, 15), "monthly", at(2027, 1, 15)},
		{"a year", at(2026, 12, 15), "annual", at(2027, 12, 15)},
		{"the 31st into a 30-day month", at(2027, 3, 31), "monthly", at(2027, 4, 30)},
		{"the 31st of January", at(2027, 1, 31), "monthly", at(2027, 2, 28)},
		{"the 29th of February a year on", at(2028, 2, 29), "annual", at(2029, 2, 28)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, tc.want.Equal(AddBillingPeriod(tc.from, tc.period)), "got %s", AddBillingPeriod(tc.from, tc.period))
		})
	}
}
