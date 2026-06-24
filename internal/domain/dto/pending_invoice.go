package dto

import (
	"encoding/json"
	"time"
)

// PendingInvoiceStatus is the lifecycle of a queued electronic-invoice submission.
// pending → submitted on a successful provider call; failed is reserved for an explicit
// permanent rejection (a connectivity outage keeps a row pending and retries via backoff,
// it is never moved to failed).
type PendingInvoiceStatus string

const (
	PendingInvoiceStatusPending   PendingInvoiceStatus = "pending"
	PendingInvoiceStatusSubmitted PendingInvoiceStatus = "submitted"
	PendingInvoiceStatusFailed    PendingInvoiceStatus = "failed"
)

// PendingInvoice is one queued electronic-invoice submission. It is enqueued in the same
// transaction as the bill (so closing an order never depends on the fiscal provider) and
// drained by the background submitter when online. RequestPayload is the full
// CreateElectronicInvoiceRequest captured at pay time — a retry resubmits the exact same
// invoice (sale-time prices, same reserved prefix+consecutive), never re-priced or re-numbered.
type PendingInvoice struct {
	ID             string
	BillID         string
	Prefix         string
	Consecutive    int
	RequestPayload json.RawMessage
	Status         PendingInvoiceStatus
	Attempts       int
	LastAttemptAt  *time.Time
	NextAttemptAt  *time.Time
	LastError      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
