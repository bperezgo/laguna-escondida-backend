package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// PaymentLinkProvider is Wompi's payment links API, called with an organization's private key.
type PaymentLinkProvider interface {
	// CreateLink returns the provider's id of the new link.
	CreateLink(ctx context.Context, privateKey string, req dto.PaymentLinkRequest) (string, error)
	DeactivateLink(ctx context.Context, privateKey, linkID string) error
	CheckoutURL(linkID string) string
}

// PaymentLinkConfig is each organization's Wompi keys, read on every use so a change applies at once.
type PaymentLinkConfig interface {
	// For reports configured=false when the organization has no usable Wompi integration.
	For(ctx context.Context, organizationID string) (creds dto.WompiCredentials, configured bool, err error)
}

// PaymentLinkRepository stores an order's links, for the context's organization.
type PaymentLinkRepository interface {
	Create(ctx context.Context, link *dto.PaymentLink) error
	// FindActiveByOrder reports found=false when the order has no active link.
	FindActiveByOrder(ctx context.Context, orderID string) (link *dto.PaymentLink, found bool, err error)
	// FindByWompiLinkID reads the link with whether its order is still open (OrderOpen).
	FindByWompiLinkID(ctx context.Context, wompiLinkID string) (link *dto.PaymentLink, found bool, err error)
	UpdateStatus(ctx context.Context, id string, status dto.PaymentLinkStatus) error
	// ListDue is the active links past their expiry whose order is still open (unpaid, not voided).
	ListDue(ctx context.Context, now time.Time) ([]*dto.PaymentLink, error)
}

// PaymentTransactionRepository stores every Wompi transaction received, for the context's organization.
type PaymentTransactionRepository interface {
	FindByWompiID(ctx context.Context, wompiTransactionID string) (tx *dto.PaymentTransaction, found bool, err error)
	// Save inserts the transaction, or updates the stored one with the same Wompi id.
	Save(ctx context.Context, tx *dto.PaymentTransaction) error
	// HasPendingForOrder is true when a transaction of any of the order's links is still PENDING.
	HasPendingForOrder(ctx context.Context, orderID string) (bool, error)
}
