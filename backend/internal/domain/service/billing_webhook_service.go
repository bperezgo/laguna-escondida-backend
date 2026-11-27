package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

type chargeSettler interface {
	Settle(ctx context.Context, tx dto.BillingTransaction) error
}

// BillingWebhookService takes the events of the platform's own Wompi account, the one restaurants
// pay their plan to. The checksum under the platform's events secret is the only proof an event is
// Wompi's; what an approval may change is then decided by Settle.
type BillingWebhookService struct {
	charges      chargeSettler
	eventsSecret string
	logger       *slog.Logger
}

func NewBillingWebhookService(charges chargeSettler, eventsSecret string, logger *slog.Logger) *BillingWebhookService {
	return &BillingWebhookService{charges: charges, eventsSecret: eventsSecret, logger: logger}
}

// Handle answers refused for anything it can't verify; an error means a verified event could not
// be applied, and Wompi should send it again.
func (s *BillingWebhookService) Handle(ctx context.Context, body []byte) (dto.PaymentWebhookOutcome, error) {
	var event dto.WompiEvent
	if err := json.Unmarshal(body, &event); err != nil {
		s.logger.WarnContext(ctx, "Wompi billing event is not JSON", slog.Any("error", err))
		return dto.PaymentWebhookRefused, nil
	}
	if s.eventsSecret == "" || !validChecksum(event, s.eventsSecret) {
		s.logger.WarnContext(ctx, "Wompi billing event refused: checksum mismatch", slog.String("event", event.Event))
		return dto.PaymentWebhookRefused, nil
	}
	if event.Event != dto.WompiEventTransactionUpdated {
		return dto.PaymentWebhookAccepted, nil
	}
	var tx dto.WompiTransaction
	if err := json.Unmarshal(event.Data["transaction"], &tx); err != nil || tx.ID == "" {
		s.logger.ErrorContext(ctx, "signed Wompi billing event without a transaction", slog.Any("error", err))
		return dto.PaymentWebhookAccepted, nil
	}
	err := s.charges.Settle(ctx, dto.BillingTransaction{ID: tx.ID, Reference: tx.Reference,
		Status: billingStatusOf(tx.Status), AmountInCents: tx.AmountInCents, Currency: tx.Currency})
	if errors.Is(err, domainError.ErrChargeNotFound) {
		// The platform's account also takes payments that are not subscription charges.
		return dto.PaymentWebhookAccepted, nil
	}
	if err != nil {
		return "", fmt.Errorf("settle billing event: %w", err)
	}
	return dto.PaymentWebhookAccepted, nil
}

// billingStatusOf keeps an unknown status pending, so the lookup asks again instead of failing a charge.
func billingStatusOf(status dto.PaymentTransactionStatus) dto.BillingChargeStatus {
	switch status {
	case dto.PaymentTransactionApproved:
		return dto.BillingChargeApproved
	case dto.PaymentTransactionDeclined:
		return dto.BillingChargeDeclined
	case dto.PaymentTransactionVoided:
		return dto.BillingChargeVoided
	case dto.PaymentTransactionError:
		return dto.BillingChargeError
	default:
		return dto.BillingChargePending
	}
}
