package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// BillingRepository is cloud-only and takes the organization explicitly: the operator reads any
// organization's billing.
type BillingRepository interface {
	// Get reports false when the organization has chosen nothing yet.
	Get(ctx context.Context, organizationID string) (*dto.OrganizationBilling, bool, error)
	// GetMany returns the rows that exist, keyed by organization id.
	GetMany(ctx context.Context, organizationIDs []string) (map[string]*dto.OrganizationBilling, error)
	// SetPeriod creates the row or changes only its period.
	SetPeriod(ctx context.Context, organizationID, period string) error
	// SaveCard creates the row or replaces only its payment source, card and consent.
	SaveCard(ctx context.Context, organizationID string, card dto.BillingCard) error

	// LockDue returns the subscriptions with a saved card, fewer than three attempts and a due
	// date at or before now, whose organization is on trial or paid. It locks each billing row and
	// skips the ones another transaction holds, so it must run inside a unit of work.
	LockDue(ctx context.Context, now time.Time) ([]dto.DueSubscription, error)
	// LockSubscription locks the organization's billing row for the rest of the transaction.
	LockSubscription(ctx context.Context, organizationID string) (*dto.OrganizationBilling, bool, error)
	// InsertCharge returns error.ErrChargePending when the organization already has an open charge.
	InsertCharge(ctx context.Context, charge *dto.BillingCharge) error
	// UpdateCharge writes the status, the transaction id and the covered period.
	UpdateCharge(ctx context.Context, charge *dto.BillingCharge) error
	IncrementAttempts(ctx context.Context, organizationID string) error
	// LockChargeByReference locks the charge for the rest of the transaction.
	LockChargeByReference(ctx context.Context, reference string) (*dto.BillingCharge, bool, error)
	FindChargeByTransactionID(ctx context.Context, transactionID string) (*dto.BillingCharge, bool, error)
	// OpenChargesCreatedBefore returns the created and pending charges older than before, oldest first.
	OpenChargesCreatedBefore(ctx context.Context, before time.Time) ([]*dto.BillingCharge, error)
	// ListCharges returns the organization's charges, newest first.
	ListCharges(ctx context.Context, organizationID string) ([]*dto.BillingCharge, error)
	// MarkPaid sets the paid period's end and zeroes the attempts; resume also removes a cancellation.
	MarkPaid(ctx context.Context, organizationID string, paidUntil time.Time, resume bool) error
	// SetCancelled marks the subscription cancelled at the given time, or resumes it with nil.
	SetCancelled(ctx context.Context, organizationID string, cancelledAt *time.Time) error
	// ClaimRecoverySignIn marks the organization's newest approved recovery or new-card charge,
	// approved since the given time, as having signed its admin in. It reports true once per charge.
	ClaimRecoverySignIn(ctx context.Context, organizationID string, approvedSince, now time.Time) (bool, error)
	// EndSubscription clears the paid period and cancels, so the organization is never charged again.
	EndSubscription(ctx context.Context, organizationID string, cancelledAt time.Time) error
}
