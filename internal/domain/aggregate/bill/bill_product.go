package bill

import (
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
)

type BillProduct struct {
	id          string
	name        string
	quantity    int
	unitPrice   decimal.Decimal
	description *string
	brand       *string
	model       *string
	code        string
	allowance   []dto.InvoiceAllowance
	taxes       []dto.InvoiceTax
	createdAt   time.Time
	updatedAt   time.Time
}

func NewBillProduct(
	productID string,
	quantity int,
	unitPrice decimal.Decimal,
	name string,
	description *string,
	brand *string,
	model *string,
	code string,
	allowance []dto.InvoiceAllowance,
	vat decimal.Decimal,
	ico decimal.Decimal,
) *BillProduct {
	baseAmount := unitPrice.Mul(decimal.NewFromInt(int64(quantity)))
	taxes := []dto.InvoiceTax{}

	if vat.GreaterThan(decimal.Zero) {
		vatAmount := baseAmount.Mul(vat)
		taxes = append(taxes, dto.InvoiceTax{
			TaxCode:   dto.TaxCodeVAT,
			TaxAmount: vatAmount.StringFixed(2),
			Percent:   vat.Mul(decimal.NewFromInt(100)).StringFixed(2),
		})
	}

	if ico.GreaterThan(decimal.Zero) {
		icoAmount := baseAmount.Mul(ico)
		taxes = append(taxes, dto.InvoiceTax{
			TaxCode:   dto.TaxCodeICO,
			TaxAmount: icoAmount.StringFixed(2),
			Percent:   ico.Mul(decimal.NewFromInt(100)).StringFixed(2),
		})
	}

	return &BillProduct{
		id:          productID,
		quantity:    quantity,
		unitPrice:   unitPrice,
		name:        name,
		description: description,
		brand:       brand,
		model:       model,
		code:        code,
		allowance:   allowance,
		taxes:       taxes,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
}

func (bp *BillProduct) ID() string {
	return bp.id
}

func (bp *BillProduct) Quantity() int {
	return bp.quantity
}

func (bp *BillProduct) UnitPrice() decimal.Decimal {
	return bp.unitPrice
}

func (bp *BillProduct) Allowance() []dto.InvoiceAllowance {
	return bp.allowance
}

func (bp *BillProduct) Taxes() []dto.InvoiceTax {
	return bp.taxes
}

func (bp *BillProduct) Name() string {
	return bp.name
}

func (bp *BillProduct) Description() *string {
	return bp.description
}

func (bp *BillProduct) Brand() *string {
	return bp.brand
}

func (bp *BillProduct) Model() *string {
	return bp.model
}

func (bp *BillProduct) Code() string {
	return bp.code
}
