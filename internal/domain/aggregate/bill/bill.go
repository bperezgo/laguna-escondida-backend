package bill

import (
	billError "laguna-escondida/backend/internal/domain/aggregate/bill/error"
	"laguna-escondida/backend/internal/domain/dto"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type Aggregate struct {
	id             string
	totalAmount    float64
	discountAmount float64
	taxAmount      float64
	payAmount      float64
	vat            float64
	ico            float64
	tip            float64
	documentURL    *string
	customer       *dto.Customer
	paymentCode    dto.ElectronicInvoicePaymentCode
	products       []*BillProduct
	createdAt      time.Time
	updatedAt      time.Time
}

func NewBillFromCreateElectronicInvoiceRequest(invoice *dto.ElectronicInvoice, products []*BillProduct) (*Aggregate, error) {
	if len(products) == 0 {
		return nil, billError.NewProductsCannotBeEmptyError()
	}

	totalAmount := 0.0
	discountAmount := 0.0
	taxAmount := 0.0
	payAmount := 0.0
	totalVat := 0.0
	totalIco := 0.0
	totalTip := 0.0

	for _, product := range products {
		totalAmount += product.unitPrice * float64(product.quantity)

		for _, allowance := range product.allowance {
			allowanceAmount, err := strconv.ParseFloat(allowance.Amount, 64)
			if err != nil {
				return nil, billError.NewInvalidAllowanceAmountError(allowance.Amount)
			}
			discountAmount += allowanceAmount
		}

		for _, tax := range product.taxes {
			taxAmount, err := strconv.ParseFloat(tax.TaxAmount, 64)
			if err != nil {
				return nil, billError.NewInvalidTaxAmountError(tax.TaxAmount)
			}

			if tax.TaxCode == dto.TaxCodeVAT {
				totalVat += taxAmount
			} else if tax.TaxCode == dto.TaxCodeICO {
				totalIco += taxAmount
			}
			taxAmount += taxAmount
		}
	}

	payAmount = totalAmount + taxAmount - discountAmount

	return &Aggregate{
		id:             uuid.New().String(),
		totalAmount:    totalAmount,
		discountAmount: discountAmount,
		taxAmount:      taxAmount,
		payAmount:      payAmount,
		vat:            totalVat,
		ico:            totalIco,
		tip:            totalTip,
		documentURL:    nil,
		customer:       invoice.Customer,
		paymentCode:    invoice.PaymentCode,
		products:       products,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
	}, nil
}

func (a *Aggregate) ToDTO() *dto.Bill {
	return &dto.Bill{
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
		Customer:       a.customer,
		Products: lo.Map(a.products, func(product *BillProduct, _ int) dto.BillProduct {
			return dto.BillProduct{
				ProductID:   product.id,
				Quantity:    product.quantity,
				UnitPrice:   product.unitPrice,
				Description: product.description,
				Brand:       product.brand,
				Model:       product.model,
				Code:        product.code,
				Allowance:   product.allowance,
				Taxes:       product.taxes,
			}
		}),
	}
}

func (a *Aggregate) Products() []*BillProduct {
	return a.products
}

func (a *Aggregate) PaymentCode() dto.ElectronicInvoicePaymentCode {
	return a.paymentCode
}
