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

// PaymentExpiryService voids agent orders whose link expired unpaid. A payment still pending at
// Wompi holds the order until it resolves; an order that never got a link never expires.
type PaymentExpiryService struct {
	runner       *OrganizationRunner
	links        ports.PaymentLinkRepository
	transactions ports.PaymentTransactionRepository
	linkService  *PaymentLinkService
	orders       orderExpirer
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
) *PaymentExpiryService {
	if now == nil {
		now = time.Now
	}
	return &PaymentExpiryService{runner: runner, links: links, transactions: transactions, linkService: linkService, orders: orders, logger: logger, now: now}
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
	return errors.Join(errs...)
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
