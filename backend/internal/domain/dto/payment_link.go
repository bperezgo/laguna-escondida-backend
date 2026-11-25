package dto

import (
	"encoding/json"
	"time"
)

type PaymentLinkStatus string

const (
	PaymentLinkActive    PaymentLinkStatus = "active"
	PaymentLinkReplaced  PaymentLinkStatus = "replaced"
	PaymentLinkExpired   PaymentLinkStatus = "expired"
	PaymentLinkCancelled PaymentLinkStatus = "cancelled"
)

// VoidReasonPaymentExpired marks an agent order voided because nobody paid its link in time.
const VoidReasonPaymentExpired = "payment_expired"

// PaymentLink is one Wompi link an agent order had.
type PaymentLink struct {
	ID            string
	OrderID       string
	WompiLinkID   string
	AmountInCents int64
	Currency      string
	ExpiresAt     time.Time
	Status        PaymentLinkStatus
	CreatedAt     time.Time
	// OrderOpen is read with the link: its order is neither paid nor voided.
	OrderOpen bool
}

// PaymentLinkRequest is what Wompi needs to create a single-use link.
type PaymentLinkRequest struct {
	Name          string
	Description   string
	AmountInCents int64
	Currency      string
	ExpiresAt     time.Time
	SKU           string
}

type PaymentTransactionStatus string

const (
	PaymentTransactionPending  PaymentTransactionStatus = "PENDING"
	PaymentTransactionApproved PaymentTransactionStatus = "APPROVED"
	PaymentTransactionDeclined PaymentTransactionStatus = "DECLINED"
	PaymentTransactionVoided   PaymentTransactionStatus = "VOIDED"
	PaymentTransactionError    PaymentTransactionStatus = "ERROR"
)

// PaymentTransaction is a Wompi transaction as its latest event reported it. PaymentLinkID is
// ours, set when its Wompi link is one of our links; Matched means it paid that link's order.
type PaymentTransaction struct {
	ID                 string
	WompiTransactionID string
	PaymentLinkID      *string
	WompiLinkID        *string
	// Reference is Wompi's, set on a QR payment: it has no link to be matched by.
	Reference         *string
	Status            PaymentTransactionStatus
	AmountInCents     int64
	Currency          string
	PaymentMethodType string
	Matched           bool
	RawEvent          json.RawMessage
}

// WompiEventTransactionUpdated is the only Wompi event that can pay an order.
const WompiEventTransactionUpdated = "transaction.updated"

// WompiEvent is the body Wompi POSTs to the webhook. Data stays raw so the checksum can read the
// properties its signature names, whatever they are.
type WompiEvent struct {
	Event     string                     `json:"event"`
	Data      map[string]json.RawMessage `json:"data"`
	Signature WompiEventSignature        `json:"signature"`
	Timestamp int64                      `json:"timestamp"`
}

type WompiEventSignature struct {
	Properties []string `json:"properties"`
	Checksum   string   `json:"checksum"`
}

// WompiTransaction is the transaction a transaction.updated event carries.
type WompiTransaction struct {
	ID                string                   `json:"id"`
	Status            PaymentTransactionStatus `json:"status"`
	AmountInCents     int64                    `json:"amount_in_cents"`
	Currency          string                   `json:"currency"`
	PaymentMethodType string                   `json:"payment_method_type"`
	PaymentLinkID     *string                  `json:"payment_link_id"`
	Reference         string                   `json:"reference"`
}

// PaymentWebhookOutcome is how the webhook answers Wompi.
type PaymentWebhookOutcome string

const (
	PaymentWebhookAccepted PaymentWebhookOutcome = "accepted"
	PaymentWebhookRefused  PaymentWebhookOutcome = "refused"
)
