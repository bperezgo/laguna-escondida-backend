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
}
