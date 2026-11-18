package dto

import (
	"encoding/json"
	"time"
)

// PendingInvoiceStatus is the lifecycle of a queued electronic-invoice submission.
// pending → submitted on a successful provider call; failed is reserved for an explicit
// permanent rejection (a connectivity outage keeps a row pending and retries via backoff,
// it is never moved to failed). pending_cloud means the edge delegated the row to the
// cloud — it will not be picked up by the edge cron.
type PendingInvoiceStatus string

const (
	PendingInvoiceStatusPending      PendingInvoiceStatus = "pending"
	PendingInvoiceStatusPendingCloud PendingInvoiceStatus = "pending_cloud"
	PendingInvoiceStatusSubmitted    PendingInvoiceStatus = "submitted"
	PendingInvoiceStatusFailed       PendingInvoiceStatus = "failed"
)

// PendingInvoice is one queued electronic-invoice submission. It is enqueued in the same
// transaction as the bill (so closing an order never depends on the fiscal provider) and
// drained by the background submitter on the cloud. Consecutive and RequestPayload are nil
// until the cloud cron assigns the next consecutive from the centralized invoice_sequences
// table and builds the provider request — this guarantees a single source of truth for
// consecutive numbers regardless of where the bill originated.
type PendingInvoice struct {
	ID             string
	BillID         string
	PaymentCode    ElectronicInvoicePaymentCode
	Prefix         string
	Consecutive    *int            // nil until assigned by the cloud cron
	RequestPayload json.RawMessage // nil until built by the cloud cron
	Status         PendingInvoiceStatus
	Attempts       int
	LastAttemptAt  *time.Time
	NextAttemptAt  *time.Time
	LastError      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
