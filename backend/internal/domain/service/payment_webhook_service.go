package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

// orderPayer is OrderService.PayOrder: a link payment is exactly a payment at the till.
type orderPayer interface {
	PayOrder(ctx context.Context, cmd command.PayOrderCommand) error
}

// PaymentWebhookService turns Wompi's signed transaction events into payments. An approved
// transaction pays only the order whose active link it is for, for that link's exact amount, once.
type PaymentWebhookService struct {
	orgs         ports.OrganizationRepository
	runner       *OrganizationRunner
	config       ports.PaymentLinkConfig
	links        ports.PaymentLinkRepository
	transactions ports.PaymentTransactionRepository
	payer        orderPayer
	unitOfWork   ports.UnitOfWork
	logger       *slog.Logger
}

func NewPaymentWebhookService(
	orgs ports.OrganizationRepository,
	runner *OrganizationRunner,
	config ports.PaymentLinkConfig,
	links ports.PaymentLinkRepository,
	transactions ports.PaymentTransactionRepository,
	payer orderPayer,
	unitOfWork ports.UnitOfWork,
	logger *slog.Logger,
) *PaymentWebhookService {
	return &PaymentWebhookService{
		orgs: orgs, runner: runner, config: config, links: links, transactions: transactions,
		payer: payer, unitOfWork: unitOfWork, logger: logger,
	}
}

// Handle answers refused for anything it can't verify as the organization's own Wompi event; an
// error means a verified event could not be applied, and Wompi should send it again.
func (s *PaymentWebhookService) Handle(ctx context.Context, orgID string, body []byte) (dto.PaymentWebhookOutcome, error) {
	log := s.logger.With(slog.String("organization_id", orgID))
	org, err := s.orgs.Get(ctx, orgID)
	if errors.Is(err, domainError.ErrOrganizationNotFound) {
		log.WarnContext(ctx, "Wompi event for an unknown organization")
		return dto.PaymentWebhookRefused, nil
	}
	if err != nil {
		return "", fmt.Errorf("read organization: %w", err)
	}
	creds, configured, err := s.config.For(ctx, org.ID)
	if err != nil {
		return "", fmt.Errorf("read wompi integration: %w", err)
	}
	if !configured {
		log.WarnContext(ctx, "Wompi event for an organization without payment links", slog.String("organization", org.Slug))
		return dto.PaymentWebhookRefused, nil
	}
	secret := creds.EventsSecret

	var event dto.WompiEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WarnContext(ctx, "Wompi event is not JSON", slog.Any("error", err))
		return dto.PaymentWebhookRefused, nil
	}
	if !validChecksum(event, secret) {
		log.WarnContext(ctx, "Wompi event refused: checksum mismatch", slog.String("event", event.Event))
		return dto.PaymentWebhookRefused, nil
	}
	if event.Event != dto.WompiEventTransactionUpdated {
		return dto.PaymentWebhookAccepted, nil
	}
	var tx dto.WompiTransaction
	if err := json.Unmarshal(event.Data["transaction"], &tx); err != nil || tx.ID == "" {
		log.ErrorContext(ctx, "signed Wompi transaction event without a transaction", slog.Any("error", err))
		return dto.PaymentWebhookAccepted, nil
	}

	if err := s.runner.Run(ctx, org.ID, func(orgCtx context.Context) error {
		return s.apply(orgCtx, log, tx, body)
	}); err != nil {
		return "", err
	}
	return dto.PaymentWebhookAccepted, nil
}

func (s *PaymentWebhookService) apply(ctx context.Context, log *slog.Logger, tx dto.WompiTransaction, body []byte) error {
	log = log.With(slog.String("wompi_transaction_id", tx.ID), slog.String("status", string(tx.Status)))
	existing, seen, err := s.transactions.FindByWompiID(ctx, tx.ID)
	if err != nil {
		return fmt.Errorf("read transaction: %w", err)
	}
	if seen && existing.Status == tx.Status {
		return nil
	}

	var link *dto.PaymentLink
	if tx.PaymentLinkID != nil && *tx.PaymentLinkID != "" {
		found := false
		if link, found, err = s.links.FindByWompiLinkID(ctx, *tx.PaymentLinkID); err != nil {
			return fmt.Errorf("read payment link: %w", err)
		}
		if !found {
			link = nil
		}
	}

	record := &dto.PaymentTransaction{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		WompiTransactionID: tx.ID,
		WompiLinkID:        tx.PaymentLinkID,
		Status:             tx.Status,
		AmountInCents:      tx.AmountInCents,
		Currency:           tx.Currency,
		PaymentMethodType:  tx.PaymentMethodType,
		RawEvent:           body,
	}
	if seen {
		record.ID, record.Matched = existing.ID, existing.Matched
	}
	if link != nil {
		record.PaymentLinkID = &link.ID
	}

	if tx.Status != dto.PaymentTransactionApproved {
		return s.transactions.Save(ctx, record)
	}

	return s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		matched, reason := matches(link, tx)
		record.Matched = matched
		// Saved before paying: a second delivery waits here on the row, then finds the order paid.
		if err := s.transactions.Save(txCtx, record); err != nil {
			return fmt.Errorf("store transaction: %w", err)
		}
		if !matched {
			// Money was taken that no order received; staff refund it in Wompi.
			log.ErrorContext(txCtx, "approved Wompi transaction paid no order", slog.String("reason", reason),
				slog.Int64("amount_in_cents", tx.AmountInCents), slog.String("currency", tx.Currency))
			return nil
		}
		// Last, so nothing after it can roll back a payment the kitchen was already told about.
		return s.payer.PayOrder(txCtx, command.PayOrderCommand{OpenBillID: link.OrderID, PaymentCode: paymentCodeFor(tx.PaymentMethodType)})
	})
}

// matches reports whether an approved transaction pays its link's order, and why not otherwise.
func matches(link *dto.PaymentLink, tx dto.WompiTransaction) (bool, string) {
	switch {
	case link == nil:
		return false, "not one of this organization's links"
	case link.Status != dto.PaymentLinkActive:
		return false, "link is " + string(link.Status)
	case !link.OrderOpen:
		return false, "order is paid or voided"
	case tx.AmountInCents != link.AmountInCents || !strings.EqualFold(tx.Currency, link.Currency):
		return false, "amount or currency differs from the link's"
	}
	return true, ""
}

// paymentCodeFor is the bill's and the e-invoice's payment method for Wompi's payment_method_type.
func paymentCodeFor(methodType string) dto.ElectronicInvoicePaymentCode {
	switch strings.ToUpper(methodType) {
	case "CARD":
		return dto.ElectronicInvoicePaymentCodeCreditCard
	case "PSE":
		return dto.ElectronicInvoicePaymentCodeTransferDebitBank
	default:
		return dto.ElectronicInvoicePaymentCodeTransferDebitInterbank
	}
}

// validChecksum is Wompi's: SHA-256 of the named properties' values, the timestamp and the secret.
func validChecksum(event dto.WompiEvent, secret string) bool {
	if event.Signature.Checksum == "" || len(event.Signature.Properties) == 0 {
		return false
	}
	var b strings.Builder
	for _, property := range event.Signature.Properties {
		value, ok := propertyValue(event.Data, property)
		if !ok {
			return false
		}
		b.WriteString(value)
	}
	b.WriteString(strconv.FormatInt(event.Timestamp, 10))
	b.WriteString(secret)
	sum := sha256.Sum256([]byte(b.String()))
	want := strings.ToLower(event.Signature.Checksum)
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(want)) == 1
}

// propertyValue reads a dotted path such as transaction.amount_in_cents from the event's data, as
// the text Wompi concatenates: a string unquoted, anything else as written.
func propertyValue(data map[string]json.RawMessage, path string) (string, bool) {
	parts := strings.Split(path, ".")
	current := data
	for i, part := range parts {
		raw, ok := current[part]
		if !ok {
			return "", false
		}
		if i == len(parts)-1 {
			var text string
			if err := json.Unmarshal(raw, &text); err == nil {
				return text, true
			}
			return strings.TrimSpace(string(raw)), true
		}
		current = nil
		if err := json.Unmarshal(raw, &current); err != nil {
			return "", false
		}
	}
	return "", false
}
