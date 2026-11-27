package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

const (
	// MaxChargeAttempts is how many scheduled charges one due date gets.
	MaxChargeAttempts = 3
	// chargeLookupAfter is how long a charge waits for the provider's event before it is looked up.
	chargeLookupAfter = 10 * time.Minute
	// chargeGiveUpAfter is when a charge still without a final status counts as failed.
	chargeGiveUpAfter = 24 * time.Hour
	centsPerPeso      = 100
)

// chargeRetryAfter is when each scheduled attempt may be made, counted from the due date.
var chargeRetryAfter = [MaxChargeAttempts]time.Duration{0, 24 * time.Hour, BillingGrace}

// ChargeService charges subscriptions and settles what the provider reports. Access never depends
// on it running: it only moves paid_until, and access reads that.
type ChargeService struct {
	billing  ports.BillingRepository
	orgs     ports.OrganizationRepository
	provider ports.BillingProvider
	uow      ports.UnitOfWork
	// access is nil where nothing caches access.
	access    ports.OrganizationAccessInvalidator
	logger    *slog.Logger
	now       func() time.Time
	newSuffix func() string
}

func NewChargeService(billing ports.BillingRepository, orgs ports.OrganizationRepository, provider ports.BillingProvider,
	uow ports.UnitOfWork, access ports.OrganizationAccessInvalidator, logger *slog.Logger) *ChargeService {
	return &ChargeService{billing: billing, orgs: orgs, provider: provider, uow: uow, access: access, logger: logger,
		now: time.Now, newSuffix: randomReferenceSuffix}
}

func randomReferenceSuffix() string {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return uuid.NewString()[:8]
	}
	return hex.EncodeToString(raw)
}

type plannedCharge struct {
	charge          *dto.BillingCharge
	paymentSourceID string
	customerEmail   string
}

// ChargeDue creates one charge for every subscription whose due date, or retry time, has come.
// Each charge is stored before the provider is called, under the billing row's lock, so neither
// another replica nor a crash can make a second one for the same attempt.
func (s *ChargeService) ChargeDue(ctx context.Context) error {
	now := s.now().UTC()
	var planned []plannedCharge
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		planned = nil
		due, err := s.billing.LockDue(ctx, now)
		if err != nil {
			return err
		}
		for _, sub := range due {
			if !chargeable(sub, now) {
				continue
			}
			if sub.CustomerEmail == "" {
				s.logger.ErrorContext(ctx, "subscription not charged: the organization has no admin with an email",
					slog.String("organization_id", sub.OrganizationID))
				continue
			}
			charge, ok := s.newCharge(sub.OrganizationID, sub.Plan, sub.Period, sub.DueOn, sub.Attempts+1, now)
			if !ok {
				s.logger.ErrorContext(ctx, "subscription not charged: no price for its plan and period",
					slog.String("organization_id", sub.OrganizationID), slog.String("plan", sub.Plan), slog.String("period", sub.Period))
				continue
			}
			if err := s.billing.InsertCharge(ctx, charge); err != nil {
				return err
			}
			if err := s.billing.IncrementAttempts(ctx, sub.OrganizationID); err != nil {
				return err
			}
			planned = append(planned, plannedCharge{charge: charge, paymentSourceID: sub.PaymentSourceID, customerEmail: sub.CustomerEmail})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("charge due subscriptions: %w", err)
	}
	for _, p := range planned {
		s.send(ctx, p)
	}
	return nil
}

// chargeable is the whole rule for a scheduled charge: a live subscription of a working
// organization, with no charge waiting, attempts left, and its retry time reached.
func chargeable(sub dto.DueSubscription, now time.Time) bool {
	if sub.PaymentSourceID == "" || sub.CancelledAt != nil || sub.HasOpenCharge {
		return false
	}
	if sub.OrganizationStatus != dto.OrganizationStatusTrial && sub.OrganizationStatus != dto.OrganizationStatusActive {
		return false
	}
	if sub.Attempts < 0 || sub.Attempts >= MaxChargeAttempts {
		return false
	}
	return !now.Before(sub.DueOn.Add(chargeRetryAfter[sub.Attempts]))
}

// newCharge reads the price now and stores it, so settlement compares against what was asked.
func (s *ChargeService) newCharge(orgID, plan, period string, dueOn time.Time, attempt int, now time.Time) (*dto.BillingCharge, bool) {
	amount := plans.Price(plans.Plan(plan), plans.Period(period))
	if amount <= 0 {
		return nil, false
	}
	suffix := fmt.Sprintf("%d", attempt)
	if attempt == 0 {
		suffix = s.newSuffix()
	}
	return &dto.BillingCharge{
		ID: uuid.Must(uuid.NewV7()).String(), OrganizationID: orgID, DueOn: dueOn, Attempt: attempt,
		Reference: fmt.Sprintf("kb-%s-%s-%s", orgID, dueOn.UTC().Format("20060102"), suffix),
		Plan:      plan, Period: period, Amount: amount, Currency: plans.Currency,
		Status: dto.BillingChargeCreated, CreatedAt: now, UpdatedAt: now,
	}, true
}

// ChargeNow makes one charge at once, outside the schedule and its count of attempts: after a new
// card, or to pay a stopped organization's way back. It returns error.ErrChargePending while
// another charge waits for its answer, and error.ErrNoSubscription without a saved card.
func (s *ChargeService) ChargeNow(ctx context.Context, orgID, customerEmail string) (*dto.BillingCharge, error) {
	now := s.now().UTC()
	var planned plannedCharge
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		billing, found, err := s.billing.LockSubscription(ctx, orgID)
		if err != nil {
			return err
		}
		if !found || !billing.HasCard() {
			return domainError.ErrNoSubscription
		}
		org, err := s.orgs.Get(ctx, orgID)
		if err != nil {
			return err
		}
		dueOn, ok := DueDate(org, billing)
		if !ok {
			return domainError.ErrNoSubscription
		}
		charge, ok := s.newCharge(orgID, org.Plan, billing.Period, dueOn, 0, now)
		if !ok {
			return fmt.Errorf("no price for plan %s and period %s", org.Plan, billing.Period)
		}
		if err := s.billing.InsertCharge(ctx, charge); err != nil {
			return err
		}
		planned = plannedCharge{charge: charge, paymentSourceID: *billing.PaymentSourceID, customerEmail: customerEmail}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("charge now: %w", err)
	}
	s.send(ctx, planned)
	charge, _, err := s.chargeByReference(ctx, planned.charge.Reference)
	if err != nil {
		return nil, fmt.Errorf("charge now: %w", err)
	}
	return charge, nil
}

// DueDate is the end of the paid period once the organization has paid, and the trial's end
// before. An organization with neither has nothing to be charged for.
func DueDate(org *dto.Organization, billing *dto.OrganizationBilling) (time.Time, bool) {
	if billing != nil && billing.PaidUntil != nil {
		return *billing.PaidUntil, true
	}
	if org.TrialEndsAt != nil {
		return *org.TrialEndsAt, true
	}
	return time.Time{}, false
}

func (s *ChargeService) chargeByReference(ctx context.Context, reference string) (*dto.BillingCharge, bool, error) {
	var charge *dto.BillingCharge
	var found bool
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		charge, found, err = s.billing.LockChargeByReference(ctx, reference)
		return err
	})
	return charge, found, err
}

// send calls the provider for a stored charge. An unanswered call leaves the charge as created:
// the lookup finds whether the provider took it.
func (s *ChargeService) send(ctx context.Context, p plannedCharge) {
	log := s.logger.With(slog.String("organization_id", p.charge.OrganizationID), slog.String("reference", p.charge.Reference))
	tx, err := s.provider.Charge(ctx, dto.BillingChargeRequest{
		PaymentSourceID: p.paymentSourceID, AmountInCents: int64(p.charge.Amount) * centsPerPeso, Currency: p.charge.Currency,
		Reference: p.charge.Reference, CustomerEmail: p.customerEmail,
	})
	switch {
	case errors.Is(err, domainError.ErrCardRefused):
		log.WarnContext(ctx, "the provider refused the charge request", slog.Any("error", err))
		if failErr := s.fail(ctx, p.charge.Reference); failErr != nil {
			log.ErrorContext(ctx, "could not store the refused charge", slog.Any("error", failErr))
		}
	case err != nil:
		log.WarnContext(ctx, "charge sent without an answer; the lookup will settle it", slog.Any("error", err))
	default:
		tx.Reference = p.charge.Reference
		if settleErr := s.Settle(ctx, tx); settleErr != nil {
			log.ErrorContext(ctx, "could not store the provider's answer; the lookup will settle it", slog.Any("error", settleErr))
		}
	}
}

// Settle applies what the provider reports of a transaction, from an event or a lookup. It returns
// error.ErrChargeNotFound when the reference is no charge's. A transaction is applied at most
// once: a charge that already has a final status is left as it is.
func (s *ChargeService) Settle(ctx context.Context, tx dto.BillingTransaction) error {
	var approvedOrg, failedOrg string
	var charge *dto.BillingCharge
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		approvedOrg, failedOrg = "", ""
		found, ok, err := s.billing.LockChargeByReference(ctx, tx.Reference)
		if err != nil {
			return err
		}
		if !ok {
			return domainError.ErrChargeNotFound
		}
		charge = found
		log := s.logger.With(slog.String("organization_id", charge.OrganizationID), slog.String("reference", charge.Reference),
			slog.String("wompi_transaction_id", tx.ID))
		if !charge.Status.Open() {
			if tx.Status == dto.BillingChargeApproved && charge.Status != dto.BillingChargeApproved {
				log.ErrorContext(ctx, "approved transaction for a charge already settled as failed: not applied, refund it by hand",
					slog.String("charge_status", string(charge.Status)))
			}
			return nil
		}
		if charge.TransactionID != nil && tx.ID != "" && *charge.TransactionID != tx.ID {
			log.ErrorContext(ctx, "transaction is not the one this charge was sent as: not applied",
				slog.String("charge_transaction_id", *charge.TransactionID))
			return nil
		}
		if tx.ID != "" {
			charge.TransactionID = &tx.ID
		}

		switch tx.Status {
		case dto.BillingChargeApproved:
			if tx.AmountInCents != int64(charge.Amount)*centsPerPeso || tx.Currency != charge.Currency {
				log.ErrorContext(ctx, "approved transaction does not match its charge: not applied",
					slog.Int64("charge_amount_in_cents", int64(charge.Amount)*centsPerPeso), slog.String("charge_currency", charge.Currency),
					slog.Int64("reported_amount_in_cents", tx.AmountInCents), slog.String("reported_currency", tx.Currency))
				charge.Status = dto.BillingChargePending
				return s.billing.UpdateCharge(ctx, charge)
			}
			if err := s.approve(ctx, charge); err != nil {
				return err
			}
			approvedOrg = charge.OrganizationID
			return nil
		case dto.BillingChargeDeclined, dto.BillingChargeVoided, dto.BillingChargeError:
			charge.Status = tx.Status
			failedOrg = charge.OrganizationID
			return s.billing.UpdateCharge(ctx, charge)
		default:
			charge.Status = dto.BillingChargePending
			return s.billing.UpdateCharge(ctx, charge)
		}
	})
	if err != nil {
		if errors.Is(err, domainError.ErrChargeNotFound) {
			return err
		}
		return fmt.Errorf("settle charge %s: %w", tx.Reference, err)
	}
	if approvedOrg != "" {
		s.invalidate(ctx, approvedOrg)
		s.logger.InfoContext(ctx, "subscription charge approved", slog.String("organization_id", approvedOrg),
			slog.String("reference", charge.Reference), slog.Int("attempt", charge.Attempt), slog.Int("amount", charge.Amount))
	}
	if failedOrg != "" {
		s.logFailed(ctx, charge)
	}
	return nil
}

// approve gives the organization its paid period. A recovery charge pays from now, since the
// organization got nothing for the time it was stopped; every other charge pays from its due date,
// however late it is approved.
func (s *ChargeService) approve(ctx context.Context, charge *dto.BillingCharge) error {
	billing, found, err := s.billing.LockSubscription(ctx, charge.OrganizationID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("organization %s has a charge and no billing", charge.OrganizationID)
	}
	org, err := s.orgs.Get(ctx, charge.OrganizationID)
	if err != nil {
		return err
	}
	stoppedWhenCharged := StoppedReason(&dto.OrganizationAccess{Status: org.Status, TrialEndsAt: org.TrialEndsAt,
		PaidUntil: billing.PaidUntil, CancelledAt: billing.CancelledAt, HasCard: billing.HasCard()}, charge.CreatedAt) != nil
	recovery := charge.Attempt == 0 && stoppedWhenCharged

	coversFrom := charge.DueOn
	if recovery {
		coversFrom = s.now().UTC()
	}
	coversUntil := AddBillingPeriod(coversFrom, charge.Period)
	charge.Status, charge.CoversFrom, charge.CoversUntil = dto.BillingChargeApproved, &coversFrom, &coversUntil
	if err := s.billing.UpdateCharge(ctx, charge); err != nil {
		return err
	}
	// A suspension is the operator's and no payment lifts it.
	if org.Status == dto.OrganizationStatusTrial {
		active := dto.OrganizationStatusActive
		if _, err := s.orgs.Update(ctx, org.ID, &dto.UpdateOrganizationRequest{Status: &active}); err != nil {
			return err
		}
	}
	return s.billing.MarkPaid(ctx, org.ID, coversUntil, recovery)
}

// AddBillingPeriod adds one month, or one year for annual, keeping the day of the month and
// stopping at the month's last day when it has no such day.
func AddBillingPeriod(from time.Time, period string) time.Time {
	months := 1
	if plans.Period(period) == plans.Annual {
		months = 12
	}
	year, month, day := from.Date()
	hour, minute, second := from.Clock()
	firstOfTarget := time.Date(year, month+time.Month(months), 1, 0, 0, 0, 0, from.Location())
	if lastDay := firstOfTarget.AddDate(0, 1, -1).Day(); day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), day, hour, minute, second, from.Nanosecond(), from.Location())
}

// fail ends an open charge that the provider never took, or never answered for, as an error.
func (s *ChargeService) fail(ctx context.Context, reference string) error {
	var failed *dto.BillingCharge
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		failed = nil
		charge, found, err := s.billing.LockChargeByReference(ctx, reference)
		if err != nil || !found || !charge.Status.Open() {
			return err
		}
		charge.Status = dto.BillingChargeError
		failed = charge
		return s.billing.UpdateCharge(ctx, charge)
	})
	if err != nil {
		return fmt.Errorf("fail charge %s: %w", reference, err)
	}
	if failed != nil {
		s.logFailed(ctx, failed)
	}
	return nil
}

// logFailed writes the line the operator watches until owners are told about a failed charge some other way.
func (s *ChargeService) logFailed(ctx context.Context, charge *dto.BillingCharge) {
	s.logger.ErrorContext(ctx, "subscription charge failed", slog.String("organization_id", charge.OrganizationID),
		slog.String("reference", charge.Reference), slog.Int("attempt", charge.Attempt), slog.String("status", string(charge.Status)),
		slog.Int("amount", charge.Amount))
}

func (s *ChargeService) invalidate(ctx context.Context, orgID string) {
	if s.access != nil {
		s.access.InvalidateAccess(ctx, orgID)
	}
}

// Reconcile looks up at the provider every charge that has waited ten minutes for an answer: a
// stored transaction by its id, and a charge whose call may never have arrived by its reference.
// One charge's failure never stops the others.
func (s *ChargeService) Reconcile(ctx context.Context) error {
	now := s.now().UTC()
	open, err := s.billing.OpenChargesCreatedBefore(ctx, now.Add(-chargeLookupAfter))
	if err != nil {
		return fmt.Errorf("reconcile charges: %w", err)
	}
	for _, charge := range open {
		s.reconcile(ctx, charge, now)
	}
	return nil
}

func (s *ChargeService) reconcile(ctx context.Context, charge *dto.BillingCharge, now time.Time) {
	log := s.logger.With(slog.String("organization_id", charge.OrganizationID), slog.String("reference", charge.Reference))
	var tx dto.BillingTransaction
	var err error
	if charge.TransactionID != nil {
		tx, err = s.provider.Transaction(ctx, *charge.TransactionID)
	} else {
		tx, err = s.provider.TransactionByReference(ctx, charge.Reference)
	}
	switch {
	case err == nil:
		tx.Reference = charge.Reference
		if settleErr := s.Settle(ctx, tx); settleErr != nil {
			log.ErrorContext(ctx, "could not settle a looked-up charge", slog.Any("error", settleErr))
			return
		}
		if tx.Status.Open() {
			s.giveUpIfOld(ctx, log, charge, now)
		}
	case errors.Is(err, domainError.ErrBillingTransactionNotFound) && charge.TransactionID == nil:
		// The call never reached the provider: nothing was charged, and the attempt is spent.
		log.WarnContext(ctx, "charge was never taken by the provider")
		if failErr := s.fail(ctx, charge.Reference); failErr != nil {
			log.ErrorContext(ctx, "could not store the failed charge", slog.Any("error", failErr))
		}
	default:
		log.WarnContext(ctx, "charge lookup failed; it will be tried again", slog.Any("error", err))
		s.giveUpIfOld(ctx, log, charge, now)
	}
}

func (s *ChargeService) giveUpIfOld(ctx context.Context, log *slog.Logger, charge *dto.BillingCharge, now time.Time) {
	if now.Sub(charge.CreatedAt) < chargeGiveUpAfter {
		return
	}
	log.ErrorContext(ctx, "charge has no final status after 24 hours: counted as failed, check it at the provider")
	if err := s.fail(ctx, charge.Reference); err != nil {
		log.ErrorContext(ctx, "could not store the failed charge", slog.Any("error", err))
	}
}
