package bill

import (
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/shared"
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

// ToDTO maps a single line item (with its computed taxes) to the transport DTO. It is the
// single source of truth for the line-item shape the fiscal provider request is built from,
// reused by Aggregate.ToDTO and by the cloud submission path that rebuilds the request.
func (bp *BillProduct) ToDTO() dto.BillProduct {
	return dto.BillProduct{
		ProductID:   bp.id,
		Quantity:    bp.quantity,
		UnitPrice:   bp.unitPrice,
		Name:        bp.name,
		Description: bp.description,
		Category:    bp.category,
		Code:        bp.code,
		Allowance:   bp.allowance,
		Taxes:       bp.taxes,
	}
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

const (
	deliveryFeeItemName = "Domicilio"
	deliveryFeeItemCode = "DOMICILIO"
)

// DeliveryFeeItem is the invoice line for a bill's delivery fee: no product behind it, the
// stored base as unit price and the stored ICO, so nothing is rounded again.
func DeliveryFeeItem(b *dto.Bill) (dto.BillProduct, bool) {
	if b.DeliveryFee == nil || !b.DeliveryFee.IsPositive() || b.DeliveryFeeBase == nil || b.DeliveryFeeICO == nil {
		return dto.BillProduct{}, false
	}
	name := deliveryFeeItemName
	return NewBillProduct("", 1, *b.DeliveryFeeBase, name, &name, name, deliveryFeeItemCode, []dto.InvoiceAllowance{},
		decimal.Zero, decimal.Zero, shared.DeliveryFeeICORate, *b.DeliveryFeeICO).ToDTO(), true
}
