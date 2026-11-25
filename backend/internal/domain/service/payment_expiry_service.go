package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

// orderExpirer is OrderService.ExpireOrder.
type orderExpirer interface {
	ExpireOrder(ctx context.Context, link *dto.PaymentLink) error
}

// manualOrderExpirer is what the expiry job needs to void manual orders: OpenBillRepository.ListManualDue
// and OrderService.ExpireManualOrder.
type manualOrderExpirer interface {
	ListManualDue(ctx context.Context, now time.Time) ([]string, error)
}

type manualOrderVoider interface {
	ExpireManualOrder(ctx context.Context, orderID string) error
}

// PaymentExpiryOption adds to the job what only the cloud has.
type PaymentExpiryOption func(*PaymentExpiryService)

// WithManualExpiry also voids manual orders past their own expiry (not under review).
func WithManualExpiry(due manualOrderExpirer, orders manualOrderVoider) PaymentExpiryOption {
	return func(s *PaymentExpiryService) { s.manualDue, s.manualOrders = due, orders }
}

// PaymentExpiryService voids agent orders whose link expired unpaid. A payment still pending at
// Wompi holds the order until it resolves; an order that never got a link never expires.
type PaymentExpiryService struct {
	runner       *OrganizationRunner
	links        ports.PaymentLinkRepository
	transactions ports.PaymentTransactionRepository
	linkService  *PaymentLinkService
	orders       orderExpirer
	manualDue    manualOrderExpirer
	manualOrders manualOrderVoider
	logger       *slog.Logger
	now          func() time.Time
}

func NewPaymentExpiryService(
	runner *OrganizationRunner,
	links ports.PaymentLinkRepository,
	transactions ports.PaymentTransactionRepository,
	linkService *PaymentLinkService,
	orders orderExpirer,
	logger *slog.Logger,
	now func() time.Time,
	opts ...PaymentExpiryOption,
) *PaymentExpiryService {
	if now == nil {
		now = time.Now
	}
	s := &PaymentExpiryService{runner: runner, links: links, transactions: transactions, linkService: linkService, orders: orders, logger: logger, now: now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ExpireDue runs once per active organization; one organization's failure is logged and the rest run.
func (s *PaymentExpiryService) ExpireDue(ctx context.Context) error {
	failures, err := s.runner.ForEach(ctx, AllActiveOrganizations, s.ExpireDueForOrganization)
	if err != nil {
		return err
	}
	for _, failure := range failures {
		s.logger.ErrorContext(ctx, "unpaid orders not expired", slog.String("organization_id", failure.OrganizationID), slog.Any("error", failure.Err))
	}
	return nil
}

// ExpireDueForOrganization expires the context organization's due orders, one at a time.
func (s *PaymentExpiryService) ExpireDueForOrganization(ctx context.Context) error {
	due, err := s.links.ListDue(ctx, s.now())
	if err != nil {
		return fmt.Errorf("list expired payment links: %w", err)
	}
	var errs []error
	for _, link := range due {
		if err := s.expire(ctx, link); err != nil {
			errs = append(errs, fmt.Errorf("order %s: %w", link.OrderID, err))
		}
	}
	errs = append(errs, s.expireManual(ctx)...)
	return errors.Join(errs...)
}

// expireManual voids manual orders past their window; an order under review is not listed.
func (s *PaymentExpiryService) expireManual(ctx context.Context) []error {
	if s.manualDue == nil || s.manualOrders == nil {
		return nil
	}
	ids, err := s.manualDue.ListManualDue(ctx, s.now())
	if err != nil {
		return []error{fmt.Errorf("list expired manual orders: %w", err)}
	}
	var errs []error
	for _, id := range ids {
		err := s.manualOrders.ExpireManualOrder(ctx, id)
		// Paid or voided since it was listed: nothing left to expire.
		if err != nil && !errors.Is(err, orderError.ErrOrderNotFound) {
			errs = append(errs, fmt.Errorf("manual order %s: %w", id, err))
		}
	}
	return errs
}

func (s *PaymentExpiryService) expire(ctx context.Context, link *dto.PaymentLink) error {
	pending, err := s.transactions.HasPendingForOrder(ctx, link.OrderID)
	if err != nil {
		return fmt.Errorf("read pending transactions: %w", err)
	}
	if pending {
		return nil
	}
	s.linkService.Expire(ctx, link)
	err = s.orders.ExpireOrder(ctx, link)
	// Paid or voided since it was listed: nothing left to expire.
	if errors.Is(err, orderError.ErrOrderNotFound) {
		return nil
	}
	return err
}
