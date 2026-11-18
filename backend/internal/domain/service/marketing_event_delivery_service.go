package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

const (
	marketingDeliveryBatch   = 50
	marketingDeliveryLease   = 2 * time.Minute
	marketingFirstRetryDelay = 10 * time.Second
	marketingMaxRetryDelay   = 30 * time.Minute
	// A 401 is a secret configured differently on each side; retrying faster won't fix it.
	marketingUnauthorizedRetryDelay = time.Hour
	marketingEventLifetime          = 72 * time.Hour
)

// MarketingEventDeliveryService sends one organization's due events, signed with its secret, and
// stores what the receiver answered. Delivery is at least once and in no particular order.
type MarketingEventDeliveryService struct {
	store         ports.MarketingEventStore
	transport     ports.MarketingEventTransport
	organizations ports.OrganizationRepository
	secrets       ports.MarketingEventSecrets
	logger        *slog.Logger
	now           func() time.Time
}

func NewMarketingEventDeliveryService(
	store ports.MarketingEventStore,
	transport ports.MarketingEventTransport,
	organizations ports.OrganizationRepository,
	secrets ports.MarketingEventSecrets,
	logger *slog.Logger,
	now func() time.Time,
) *MarketingEventDeliveryService {
	if now == nil {
		now = time.Now
	}
	return &MarketingEventDeliveryService{
		store: store, transport: transport, organizations: organizations, secrets: secrets, logger: logger, now: now,
	}
}

// DeliverDue sends the context's organization's due events and returns how many were delivered.
func (s *MarketingEventDeliveryService) DeliverDue(ctx context.Context) (int, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return 0, err
	}
	org, err := s.organizations.Get(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("read organization: %w", err)
	}
	secret, ok := s.secrets.SecretFor(org.Slug)
	if !ok {
		return 0, nil
	}
	now := s.now()
	events, err := s.store.ClaimDue(ctx, now, now.Add(marketingDeliveryLease), marketingDeliveryBatch)
	if err != nil {
		return 0, fmt.Errorf("claim due marketing events: %w", err)
	}
	delivered := 0
	for _, event := range events {
		ok, err := s.deliver(ctx, event, secret)
		if err != nil {
			return delivered, err
		}
		if ok {
			delivered++
		}
	}
	return delivered, nil
}

func (s *MarketingEventDeliveryService) deliver(ctx context.Context, event *dto.MarketingEvent, secret string) (bool, error) {
	log := s.logger.With(slog.String("event_id", event.EventID), slog.String("type", string(event.Type)), slog.Int("attempts", event.Attempts))
	now := s.now()
	if now.Sub(event.CreatedAt) > marketingEventLifetime {
		log.ErrorContext(ctx, "marketing event expired undelivered")
		return false, s.store.MarkFailed(ctx, event.EventID, dto.MarketingEventExpired, now, "not delivered within 72h")
	}

	status, sendErr := s.transport.Send(ctx, event.Path, event.Body, SignMarketingEvent(secret, event.Body))
	now = s.now()
	switch {
	case sendErr == nil && status >= 200 && status < 300:
		return true, s.store.MarkSent(ctx, event.EventID, now)
	case sendErr == nil && status == http.StatusBadRequest:
		log.ErrorContext(ctx, "marketing-app rejected an event as malformed; it will not be retried")
		return false, s.store.MarkFailed(ctx, event.EventID, dto.MarketingEventRejected, now, "400 Bad Request")
	case sendErr == nil && status == http.StatusUnauthorized:
		log.ErrorContext(ctx, "marketing-app refused an event's signature; check the organization's signing secret on both sides")
		return false, s.store.MarkFailed(ctx, event.EventID, dto.MarketingEventPending, now.Add(marketingUnauthorizedRetryDelay), "401 Unauthorized")
	}

	reason := fmt.Sprintf("HTTP %d", status)
	if sendErr != nil {
		reason = sendErr.Error()
	}
	log.WarnContext(ctx, "marketing event delivery failed; retrying", slog.String("reason", reason))
	return false, s.store.MarkFailed(ctx, event.EventID, dto.MarketingEventPending, now.Add(marketingRetryDelay(event.Attempts)), reason)
}

// marketingRetryDelay doubles from 10s per attempt made, capped at 30 minutes.
func marketingRetryDelay(attempts int) time.Duration {
	delay := marketingFirstRetryDelay
	for i := 1; i < attempts && delay < marketingMaxRetryDelay; i++ {
		delay *= 2
	}
	if delay > marketingMaxRetryDelay {
		return marketingMaxRetryDelay
	}
	return delay
}

// SignMarketingEvent is the X-Signature-256 value: sha256=<hex HMAC-SHA256 of the exact body>.
func SignMarketingEvent(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
