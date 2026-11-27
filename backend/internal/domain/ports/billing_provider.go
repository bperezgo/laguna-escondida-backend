package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// BillingProvider is the payment gateway the platform itself charges restaurants through,
// with the platform's own keys.
type BillingProvider interface {
	// Terms returns the provider's two documents and a fresh acceptance token for each.
	Terms(ctx context.Context) (dto.WompiMerchantTerms, error)
	// SavePaymentSource turns a card token into a chargeable source and charges nothing. It returns
	// error.ErrCardRefused for a refused token or an unusable source, and
	// error.ErrPaymentProviderUnavailable when the provider does not answer.
	SavePaymentSource(ctx context.Context, req dto.PaymentSourceRequest) (dto.PaymentSource, error)
	// Charge asks for one charge of a saved source and returns the transaction as the provider
	// answers it, usually still pending. It returns error.ErrCardRefused when the provider refuses
	// the request itself, and error.ErrPaymentProviderUnavailable when it does not answer: the
	// transaction may then exist, and only TransactionByReference can tell.
	Charge(ctx context.Context, req dto.BillingChargeRequest) (dto.BillingTransaction, error)
	// Transaction returns error.ErrBillingTransactionNotFound for an id the provider does not know.
	Transaction(ctx context.Context, id string) (dto.BillingTransaction, error)
	// TransactionByReference returns error.ErrBillingTransactionNotFound when no transaction has the reference.
	TransactionByReference(ctx context.Context, reference string) (dto.BillingTransaction, error)
}
