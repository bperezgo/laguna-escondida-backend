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
	category    string
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
	category string,
	code string,
	allowance []dto.InvoiceAllowance,
	vatPercentage decimal.Decimal,
	vatAmount decimal.Decimal,
	icoPercentage decimal.Decimal,
	icoAmount decimal.Decimal,
) *BillProduct {
	quantityDecimal := decimal.NewFromInt(int64(quantity))
	taxes := []dto.InvoiceTax{}

	// Use pre-calculated absolute tax amounts, multiply by quantity
	if vatAmount.GreaterThan(decimal.Zero) {
		totalVatAmount := vatAmount.Mul(quantityDecimal)
		taxes = append(taxes, dto.InvoiceTax{
			TaxCode:   dto.TaxCodeVAT,
			TaxAmount: totalVatAmount.StringFixed(2),
			Percent:   vatPercentage.Mul(decimal.NewFromInt(100)).StringFixed(2),
		})
	}

	if icoAmount.GreaterThan(decimal.Zero) {
		totalIcoAmount := icoAmount.Mul(quantityDecimal)
		taxes = append(taxes, dto.InvoiceTax{
			TaxCode:   dto.TaxCodeICO,
			TaxAmount: totalIcoAmount.StringFixed(2),
			Percent:   icoPercentage.Mul(decimal.NewFromInt(100)).StringFixed(2),
		})
	}

	return &BillProduct{
		id:          productID,
		quantity:    quantity,
		unitPrice:   unitPrice,
		name:        name,
		description: description,
		category:    category,
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

func (bp *BillProduct) Category() string {
	return bp.category
}

func (bp *BillProduct) Code() string {
	return bp.code
}
