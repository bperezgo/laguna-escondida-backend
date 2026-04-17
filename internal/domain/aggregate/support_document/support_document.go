package support_document

import (
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/shared"
	sdError "laguna-escondida/backend/internal/domain/aggregate/support_document/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type Aggregate struct {
	id             string
	totalAmount    decimal.Decimal
	discountAmount decimal.Decimal
	taxAmount      decimal.Decimal
	payAmount      decimal.Decimal
	vat            decimal.Decimal
	ico            decimal.Decimal
	tip            decimal.Decimal
	documentURL    *string
	provider       dto.Provider
	paymentCode    *shared.PaymentCode
	products       []*SupportDocumentProduct
	createdAt      time.Time
	updatedAt      time.Time
}

func NewSupportDocumentFromRequest(doc *dto.SupportDocument, products []*SupportDocumentProduct) (*Aggregate, error) {
	if len(products) == 0 {
		return nil, sdError.NewProductsCannotBeEmptyError()
	}

	if doc.Provider.DocumentNumber == "" || doc.Provider.Name == "" {
		return nil, sdError.NewProviderRequiredError()
	}

	paymentCode, err := shared.NewPaymentCode(doc.PaymentCode)
	if err != nil {
		return nil, err
	}

	totalAmount := decimal.Zero

	for _, product := range products {
		totalAmount = totalAmount.Add(product.unitPrice.Mul(decimal.NewFromInt(int64(product.quantity))))
	}

	payAmount := totalAmount

	return &Aggregate{
		id:             uuid.Must(uuid.NewV7()).String(),
		totalAmount:    totalAmount,
		discountAmount: decimal.Zero,
		taxAmount:      decimal.Zero,
		payAmount:      payAmount,
		vat:            decimal.Zero,
		ico:            decimal.Zero,
		tip:            decimal.Zero,
		documentURL:    nil,
		provider:       doc.Provider,
		paymentCode:    paymentCode,
		products:       products,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
	}, nil
}

func (a *Aggregate) ToDTO() *dto.SupportDocumentBill {
	return &dto.SupportDocumentBill{
		ID:             a.id,
		TotalAmount:    a.totalAmount,
		DiscountAmount: a.discountAmount,
		TaxAmount:      a.taxAmount,
		PayAmount:      a.payAmount,
		CreatedAt:      a.createdAt,
		UpdatedAt:      a.updatedAt,
		VAT:            a.vat,
		ICO:            a.ico,
		Tip:            a.tip,
		DocumentURL:    a.documentURL,
		Provider:       a.provider,
		Products: lo.Map(a.products, func(product *SupportDocumentProduct, _ int) dto.BillProduct {
			return dto.BillProduct{
				Name:      product.description,
				Quantity:  product.quantity,
				UnitPrice: product.unitPrice,
			}
		}),
	}
}

func (a *Aggregate) Products() []*SupportDocumentProduct {
	return a.products
}

func (a *Aggregate) PaymentCode() dto.ElectronicInvoicePaymentCode {
	return a.paymentCode.Value()
}
