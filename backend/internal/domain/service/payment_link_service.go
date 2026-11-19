package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PaymentLinkService keeps one active Wompi link per agent order. Links are made after the order
// commits, so a Wompi failure never fails the order: it is left without a link and logged.
type PaymentLinkService struct {
	links    ports.PaymentLinkRepository
	provider ports.PaymentLinkProvider
	config   ports.PaymentLinkConfig
	orgs     ports.OrganizationRepository
	ttl      time.Duration
	logger   *slog.Logger
	now      func() time.Time
}

func NewPaymentLinkService(
	links ports.PaymentLinkRepository,
	provider ports.PaymentLinkProvider,
	config ports.PaymentLinkConfig,
	orgs ports.OrganizationRepository,
	ttl time.Duration,
	logger *slog.Logger,
	now func() time.Time,
) *PaymentLinkService {
	if now == nil {
		now = time.Now
	}
	return &PaymentLinkService{links: links, provider: provider, config: config, orgs: orgs, ttl: ttl, logger: logger, now: now}
}

// Configured reports whether the context's organization can take agent orders.
func (s *PaymentLinkService) Configured(ctx context.Context) (bool, error) {
	org, err := s.organization(ctx)
	if err != nil {
		return false, err
	}
	return s.config.ConfiguredFor(org.Slug), nil
}

// CreateForOrder makes the order's first link, expiring the TTL after the order was created.
func (s *PaymentLinkService) CreateForOrder(ctx context.Context, order *dto.OpenBill) *dto.PaymentLink {
	return s.create(ctx, order, order.CreatedAt.Add(s.ttl))
}

// ReplaceForOrder swaps the order's active link for one of its new total, with the same expiry:
// a change never buys more time. An order without an active link stays without one.
func (s *PaymentLinkService) ReplaceForOrder(ctx context.Context, order *dto.OpenBill) *dto.PaymentLink {
	current, found, err := s.links.FindActiveByOrder(ctx, order.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment link not replaced: read the active link", slog.String("order_id", order.ID), slog.Any("error", err))
		return nil
	}
	if !found {
		return nil
	}
	// Past its expiry the order is the expiry job's to void; a new link would outlive it.
	if !s.now().Before(current.ExpiresAt) {
		return nil
	}
	s.deactivateAtProvider(ctx, current)
	if err := s.links.UpdateStatus(ctx, current.ID, dto.PaymentLinkReplaced); err != nil {
		s.logger.ErrorContext(ctx, "payment link not replaced: mark the old one replaced", slog.String("order_id", order.ID), slog.Any("error", err))
		return nil
	}
	return s.create(ctx, order, current.ExpiresAt)
}

// Cancel deactivates a voided order's link. A failure is logged: the order is already voided,
// and a payment on the link would be stored as unmatched.
func (s *PaymentLinkService) Cancel(ctx context.Context, orderID string) {
	current, found, err := s.links.FindActiveByOrder(ctx, orderID)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment link not cancelled: read the active link", slog.String("order_id", orderID), slog.Any("error", err))
		return
	}
	if !found {
		return
	}
	s.deactivateAtProvider(ctx, current)
	if err := s.links.UpdateStatus(ctx, current.ID, dto.PaymentLinkCancelled); err != nil {
		s.logger.ErrorContext(ctx, "payment link not cancelled", slog.String("order_id", orderID), slog.Any("error", err))
	}
}

// Expire deactivates an expired link at Wompi, best effort: Wompi already refuses a payment past
// the link's expires_at.
func (s *PaymentLinkService) Expire(ctx context.Context, link *dto.PaymentLink) {
	s.deactivateAtProvider(ctx, link)
}

// MarkExpired records the link expired; ExpireOrder calls it in the void's transaction.
func (s *PaymentLinkService) MarkExpired(ctx context.Context, linkID string) error {
	return s.links.UpdateStatus(ctx, linkID, dto.PaymentLinkExpired)
}

// ActiveURL is the checkout URL of the order's active link, or nil when it has none.
func (s *PaymentLinkService) ActiveURL(ctx context.Context, orderID string) (*string, error) {
	link, found, err := s.links.FindActiveByOrder(ctx, orderID)
	if err != nil || !found {
		return nil, err
	}
	return s.URL(link), nil
}

func (s *PaymentLinkService) URL(link *dto.PaymentLink) *string {
	if link == nil {
		return nil
	}
	url := s.provider.CheckoutURL(link.WompiLinkID)
	return &url
}

func (s *PaymentLinkService) create(ctx context.Context, order *dto.OpenBill, expiresAt time.Time) *dto.PaymentLink {
	log := s.logger.With(slog.String("order_id", order.ID))
	org, err := s.organization(ctx)
	if err != nil {
		log.ErrorContext(ctx, "payment link not created: read the organization", slog.Any("error", err))
		return nil
	}
	privateKey, ok := s.config.PrivateKeyFor(org.Slug)
	if !ok {
		log.ErrorContext(ctx, "payment link not created: the organization has no Wompi private key", slog.String("organization", org.Slug))
		return nil
	}
	amount := amountInCents(order.TotalAmount)
	req := dto.PaymentLinkRequest{
		Name:          restaurantName(org),
		Description:   "Pedido " + shortOrderID(order.ID),
		AmountInCents: amount,
		Currency:      org.Currency,
		ExpiresAt:     expiresAt.UTC(),
		SKU:           order.ID,
	}
	wompiLinkID, err := s.provider.CreateLink(ctx, privateKey, req)
	if err != nil {
		log.ErrorContext(ctx, "payment link not created: Wompi refused it", slog.Any("error", err))
		return nil
	}
	link := &dto.PaymentLink{
		ID:            uuid.Must(uuid.NewV7()).String(),
		OrderID:       order.ID,
		WompiLinkID:   wompiLinkID,
		AmountInCents: amount,
		Currency:      org.Currency,
		ExpiresAt:     req.ExpiresAt,
		Status:        dto.PaymentLinkActive,
		CreatedAt:     s.now(),
	}
	if err := s.links.Create(ctx, link); err != nil {
		log.ErrorContext(ctx, "payment link created at Wompi but not stored", slog.String("wompi_link_id", wompiLinkID), slog.Any("error", err))
		s.deactivateWith(ctx, privateKey, wompiLinkID)
		return nil
	}
	return link
}

func (s *PaymentLinkService) deactivateAtProvider(ctx context.Context, link *dto.PaymentLink) {
	org, err := s.organization(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment link not deactivated at Wompi: read the organization", slog.String("wompi_link_id", link.WompiLinkID), slog.Any("error", err))
		return
	}
	privateKey, ok := s.config.PrivateKeyFor(org.Slug)
	if !ok {
		s.logger.ErrorContext(ctx, "payment link not deactivated at Wompi: no private key", slog.String("wompi_link_id", link.WompiLinkID))
		return
	}
	s.deactivateWith(ctx, privateKey, link.WompiLinkID)
}

func (s *PaymentLinkService) deactivateWith(ctx context.Context, privateKey, wompiLinkID string) {
	if err := s.provider.DeactivateLink(ctx, privateKey, wompiLinkID); err != nil {
		s.logger.ErrorContext(ctx, "payment link not deactivated at Wompi", slog.String("wompi_link_id", wompiLinkID), slog.Any("error", err))
	}
}

func (s *PaymentLinkService) organization(ctx context.Context) (*dto.Organization, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil, err
	}
	org, err := s.orgs.Get(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("read organization: %w", err)
	}
	return org, nil
}

func amountInCents(total decimal.Decimal) int64 {
	return total.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func restaurantName(org *dto.Organization) string {
	if org.DisplayName != nil && strings.TrimSpace(*org.DisplayName) != "" {
		return *org.DisplayName
	}
	return org.Name
}

// shortOrderID is what staff and the customer can read out: the id's last six characters.
func shortOrderID(orderID string) string {
	compact := strings.ReplaceAll(orderID, "-", "")
	if len(compact) > 6 {
		compact = compact[len(compact)-6:]
	}
	return strings.ToUpper(compact)
}
