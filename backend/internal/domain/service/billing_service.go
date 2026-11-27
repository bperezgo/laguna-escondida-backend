package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

// recoverySignInWindow bounds how long after its approval a recovery payment still signs its admin in.
const recoverySignInWindow = time.Hour

// BillingService is what an admin does about the subscription from inside the app, and what the
// admin of a stopped organization does to pay its way back.
type BillingService struct {
	billing  ports.BillingRepository
	orgRepo  ports.OrganizationRepository
	userRole ports.UserRoleRepository
	users    *UserService
	setup    *BillingSetupService
	uow      ports.UnitOfWork
	// charges is nil while the cloud has no events or integrity secret: nothing is charged.
	charges *ChargeService
	// access is nil where nothing caches access.
	access ports.OrganizationAccessInvalidator
	now    func() time.Time
}

func NewBillingService(billing ports.BillingRepository, orgRepo ports.OrganizationRepository, userRole ports.UserRoleRepository,
	users *UserService, setup *BillingSetupService, uow ports.UnitOfWork, access ports.OrganizationAccessInvalidator) *BillingService {
	return &BillingService{billing: billing, orgRepo: orgRepo, userRole: userRole, users: users, setup: setup, uow: uow,
		access: access, now: time.Now}
}

// EnableCharging turns charging on. Without it cards are saved and changed, and nothing is charged.
func (s *BillingService) EnableCharging(charges *ChargeService) { s.charges = charges }

func (s *BillingService) Charging() bool { return s.charges != nil }

type subscriptionFacts struct {
	org     *dto.Organization
	billing *dto.OrganizationBilling
	charges []*dto.BillingCharge
}

func (s *BillingService) facts(ctx context.Context, orgID string) (*subscriptionFacts, error) {
	org, err := s.orgRepo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}
	billing, found, err := s.billing.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &subscriptionFacts{org: org, billing: &dto.OrganizationBilling{OrganizationID: orgID, Period: string(plans.Monthly)}}, nil
	}
	facts := &subscriptionFacts{org: org, billing: billing}
	if billing.HasCard() {
		if facts.charges, err = s.billing.ListCharges(ctx, orgID); err != nil {
			return nil, err
		}
	}
	return facts, nil
}

func (f *subscriptionFacts) access() *dto.OrganizationAccess {
	return &dto.OrganizationAccess{Status: f.org.Status, Plan: f.org.Plan, TrialEndsAt: f.org.TrialEndsAt,
		PaidUntil: f.billing.PaidUntil, CancelledAt: f.billing.CancelledAt, HasCard: f.billing.HasCard()}
}

// charged reports whether the organization is one the charge job charges: a live subscription of
// a trial, or of an organization that has paid before.
func (f *subscriptionFacts) charged() bool {
	if !f.billing.HasCard() || f.billing.Cancelled() {
		return false
	}
	switch f.org.Status {
	case dto.OrganizationStatusTrial:
		return f.org.TrialEndsAt != nil
	case dto.OrganizationStatusActive:
		return f.billing.PaidUntil != nil
	default:
		return false
	}
}

func (f *subscriptionFacts) latestCharge() *dto.BillingCharge {
	if len(f.charges) == 0 {
		return nil
	}
	return f.charges[0]
}

func (f *subscriptionFacts) summary(now time.Time, cardEntry bool) *dto.BillingSummary {
	summary := &dto.BillingSummary{Period: f.billing.Period, HasCard: f.billing.HasCard(), CardEntry: cardEntry,
		Cancelled: f.billing.HasCard() && f.billing.Cancelled()}
	access := f.access()
	working := f.org.Status == dto.OrganizationStatusTrial || f.org.Status == dto.OrganizationStatusActive
	dueOn, hasDue := DueDate(f.org, f.billing)
	// An organization activated by hand has a trial end and no due date: it is never charged and never stops.
	if f.org.Status == dto.OrganizationStatusActive && f.billing.PaidUntil == nil {
		hasDue = false
	}
	if !working || !hasDue {
		return summary
	}
	summary.DueDate = &dueOn
	ends := dueOn
	if access.HasCard && access.CancelledAt == nil {
		ends = dueOn.Add(BillingGrace)
	}
	summary.AccessEndsAt = &ends
	if f.charged() {
		if amount := plans.Price(plans.Plan(f.org.Plan), plans.Period(f.billing.Period)); amount > 0 {
			summary.NextChargeAmount = &amount
		}
		latest := f.latestCharge()
		failed := latest != nil && !latest.Status.Open() && latest.Status != dto.BillingChargeApproved && latest.DueOn.Equal(dueOn)
		summary.PaymentFailed = failed && StoppedReason(access, now) == nil
	}
	return summary
}

func chargeView(charge *dto.BillingCharge) dto.SubscriptionCharge {
	status := charge.Status
	// created is ours alone: to the admin the charge is waiting for its answer either way.
	if status == dto.BillingChargeCreated {
		status = dto.BillingChargePending
	}
	return dto.SubscriptionCharge{ID: charge.ID, Date: charge.CreatedAt, Amount: charge.Amount, Currency: charge.Currency,
		Status: string(status), CoversFrom: charge.CoversFrom, CoversUntil: charge.CoversUntil}
}

func savedCard(billing *dto.OrganizationBilling) *dto.SavedCard {
	if !billing.HasCard() {
		return nil
	}
	card := &dto.SavedCard{}
	if billing.CardBrand != nil {
		card.Brand = *billing.CardBrand
	}
	if billing.CardLastFour != nil {
		card.LastFour = *billing.CardLastFour
	}
	return card
}

// Read never carries the payment source's identifier.
func (s *BillingService) Read(ctx context.Context, orgID string) (*dto.Subscription, error) {
	facts, err := s.facts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("read subscription: %w", err)
	}
	summary := facts.summary(s.now(), s.setup.CardEntry())
	sub := &dto.Subscription{
		Plan: facts.org.Plan, Period: facts.billing.Period, Status: facts.org.Status,
		TrialEndsAt: facts.org.TrialEndsAt, PaidUntil: facts.billing.PaidUntil,
		DueDate: summary.DueDate, NextChargeAmount: summary.NextChargeAmount, Currency: plans.Currency,
		Cancelled: summary.Cancelled, PaymentFailed: summary.PaymentFailed, AccessEndsAt: summary.AccessEndsAt,
		Card: savedCard(facts.billing), Charges: []dto.SubscriptionCharge{}, CardEntry: summary.CardEntry,
	}
	for _, charge := range facts.charges {
		sub.Charges = append(sub.Charges, chargeView(charge))
	}
	if len(sub.Charges) > 0 {
		sub.LatestCharge = &sub.Charges[0]
	}
	return sub, nil
}

// Summary is what the app's notice shows an admin on every page.
func (s *BillingService) Summary(ctx context.Context, orgID string) (*dto.BillingSummary, error) {
	facts, err := s.facts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("read billing summary: %w", err)
	}
	return facts.summary(s.now(), s.setup.CardEntry()), nil
}

func (s *BillingService) CardSetup(ctx context.Context) (*dto.BillingCardSetup, error) {
	return s.setup.CardSetup(ctx)
}

// ChangeCard replaces the saved card under the rules of the first save. A refused card leaves the
// previous one in use. When the due date has passed with no approved charge, the new card is
// charged at once, outside the three scheduled attempts.
func (s *BillingService) ChangeCard(ctx context.Context, orgID, userID string, req *dto.SaveCardRequest) (*dto.Subscription, error) {
	ctx = tenant.WithOrg(ctx, orgID)
	card, holder, err := s.setup.newCard(ctx, orgID, userID, req)
	if err != nil {
		return nil, fmt.Errorf("change card: %w", err)
	}
	if err = s.billing.SaveCard(ctx, orgID, card); err != nil {
		return nil, fmt.Errorf("change card: %w", err)
	}
	s.invalidate(ctx, orgID)

	facts, err := s.facts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("change card: %w", err)
	}
	if dueOn, ok := DueDate(facts.org, facts.billing); s.Charging() && facts.charged() && ok && !s.now().Before(dueOn) {
		// A charge already waiting will be answered for the old card; the schedule tries the new one next.
		if _, chargeErr := s.charges.ChargeNow(ctx, orgID, *holder.Email); chargeErr != nil && !errors.Is(chargeErr, domainError.ErrChargePending) {
			return nil, fmt.Errorf("change card: %w", chargeErr)
		}
	}
	return s.Read(ctx, orgID)
}

// Cancel keeps the card and the access until the due date; nothing is charged after it.
func (s *BillingService) Cancel(ctx context.Context, orgID string) (*dto.Subscription, error) {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		billing, found, err := s.billing.LockSubscription(ctx, orgID)
		if err != nil {
			return err
		}
		if !found || !billing.HasCard() {
			return domainError.ErrNoSubscription
		}
		if billing.Cancelled() {
			return nil
		}
		now := s.now().UTC()
		return s.billing.SetCancelled(ctx, orgID, &now)
	})
	if err != nil {
		return nil, fmt.Errorf("cancel subscription: %w", err)
	}
	s.invalidate(ctx, orgID)
	return s.Read(ctx, orgID)
}

// Resume removes the cancellation while the due date has not passed.
func (s *BillingService) Resume(ctx context.Context, orgID string) (*dto.Subscription, error) {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		billing, found, err := s.billing.LockSubscription(ctx, orgID)
		if err != nil {
			return err
		}
		if !found || !billing.HasCard() {
			return domainError.ErrNoSubscription
		}
		if !billing.Cancelled() {
			return nil
		}
		org, err := s.orgRepo.Get(ctx, orgID)
		if err != nil {
			return err
		}
		if dueOn, ok := DueDate(org, billing); ok && !s.now().Before(dueOn) {
			return domainError.ErrSubscriptionLapsed
		}
		return s.billing.SetCancelled(ctx, orgID, nil)
	})
	if err != nil {
		return nil, fmt.Errorf("resume subscription: %w", err)
	}
	s.invalidate(ctx, orgID)
	return s.Read(ctx, orgID)
}

func (s *BillingService) invalidate(ctx context.Context, orgID string) {
	if s.access != nil {
		s.access.InvalidateAccess(ctx, orgID)
	}
}

func stoppedCode(reason error) string {
	if errors.Is(reason, domainError.ErrPaymentRequired) {
		return "payment_required"
	}
	return "trial_expired"
}

// RecoveryState answers what a stopped organization owes. Once its recovery payment is approved it
// answers, once, the sign-in body instead: the page's polling is what signs the person in.
func (s *BillingService) RecoveryState(ctx context.Context, orgID, userID string) (*dto.RecoveryState, *dto.SignInResponse, error) {
	ctx = tenant.WithOrg(ctx, orgID)
	facts, err := s.facts(ctx, orgID)
	if err != nil {
		return nil, nil, fmt.Errorf("recovery state: %w", err)
	}
	now := s.now()
	reason := StoppedReason(facts.access(), now)
	if reason == nil {
		signIn, signInErr := s.recoverySignIn(ctx, orgID, userID, now)
		return nil, signIn, signInErr
	}
	state := &dto.RecoveryState{
		OrganizationName: facts.org.Name, Reason: stoppedCode(reason), Plan: facts.org.Plan, Period: facts.billing.Period,
		AmountDue: plans.Price(plans.Plan(facts.org.Plan), plans.Period(facts.billing.Period)), Currency: plans.Currency,
		Card: savedCard(facts.billing),
	}
	if latest := facts.latestCharge(); latest != nil {
		view := chargeView(latest)
		state.LatestCharge = &view
	}
	// Paying needs charging, not only card entry: a card that can't be charged pays nothing.
	if !s.Charging() {
		return state, nil, nil
	}
	setup, err := s.setup.CardSetup(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("recovery state: %w", err)
	}
	state.CardEntry, state.Wompi = setup.CardEntry, setup.Wompi
	return state, nil, nil
}

func (s *BillingService) recoverySignIn(ctx context.Context, orgID, userID string, now time.Time) (*dto.SignInResponse, error) {
	org, err := s.orgRepo.Get(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("recovery sign-in: %w", err)
	}
	if org.Status != dto.OrganizationStatusActive && org.Status != dto.OrganizationStatusTrial {
		return nil, domainError.ErrOrganizationNotStopped
	}
	holder, err := s.setup.cardHolder(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("recovery sign-in: %w", err)
	}
	claimed, err := s.billing.ClaimRecoverySignIn(ctx, orgID, now.Add(-recoverySignInWindow), now.UTC())
	if err != nil {
		return nil, fmt.Errorf("recovery sign-in: %w", err)
	}
	if !claimed {
		return nil, domainError.ErrOrganizationNotStopped
	}
	roles, err := s.userRole.FindRolesByUserID(ctx, holder.ID)
	if err != nil {
		return nil, fmt.Errorf("recovery sign-in: read roles: %w", err)
	}
	return s.users.issueSignIn(holder, orgID, roles)
}

// RecoverySelect changes plan and period as signup does, for an organization that is stopped.
func (s *BillingService) RecoverySelect(ctx context.Context, orgID string, req *dto.BillingSelectionRequest) (*dto.BillingSelection, error) {
	return s.setup.selectWhen(ctx, orgID, req, func(ctx context.Context, org *dto.Organization) error {
		billing, found, err := s.billing.Get(ctx, orgID)
		if err != nil {
			return err
		}
		if !found {
			billing = &dto.OrganizationBilling{OrganizationID: orgID}
		}
		facts := subscriptionFacts{org: org, billing: billing}
		if StoppedReason(facts.access(), s.now()) == nil {
			return domainError.ErrOrganizationNotStopped
		}
		return nil
	})
}

// RecoveryPay saves the new card when one is given and charges the plan and period at once.
func (s *BillingService) RecoveryPay(ctx context.Context, orgID, userID string, req *dto.RecoveryPayRequest) (*dto.RecoveryPayResponse, error) {
	if !s.Charging() {
		return nil, domainError.ErrChargingOff
	}
	ctx = tenant.WithOrg(ctx, orgID)
	facts, err := s.facts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("recovery pay: %w", err)
	}
	if StoppedReason(facts.access(), s.now()) == nil {
		return nil, domainError.ErrOrganizationNotStopped
	}
	if latest := facts.latestCharge(); latest != nil && latest.Status.Open() {
		return nil, domainError.ErrChargePending
	}

	newCard := strings.TrimSpace(req.CardToken) != ""
	if !newCard && !facts.billing.HasCard() {
		return nil, &domainError.BillingValidationError{Field: "card_token", Reason: "is required: there is no saved card"}
	}
	var holder *dto.User
	if newCard {
		var card dto.BillingCard
		card, holder, err = s.setup.newCard(ctx, orgID, userID, &dto.SaveCardRequest{CardToken: req.CardToken,
			AcceptanceToken: req.AcceptanceToken, PersonalDataAcceptanceToken: req.PersonalDataAcceptanceToken})
		if err != nil {
			return nil, fmt.Errorf("recovery pay: %w", err)
		}
		if err = s.billing.SaveCard(ctx, orgID, card); err != nil {
			return nil, fmt.Errorf("recovery pay: %w", err)
		}
	} else if holder, err = s.setup.cardHolder(ctx, userID); err != nil {
		return nil, fmt.Errorf("recovery pay: %w", err)
	}

	charge, err := s.charges.ChargeNow(ctx, orgID, *holder.Email)
	if err != nil {
		return nil, fmt.Errorf("recovery pay: %w", err)
	}
	s.invalidate(ctx, orgID)
	return &dto.RecoveryPayResponse{Charge: chargeView(charge)}, nil
}
