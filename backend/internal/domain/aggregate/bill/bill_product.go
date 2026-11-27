package bill

import (
	"time"

	billError "laguna-escondida/backend/internal/domain/aggregate/bill/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
)

type BillProduct struct {
	id          string
	name        string
	quantity    int
	unitPrice   decimal.Decimal
	total       decimal.Decimal
	description *string
	category    string
	code        string
	allowance   []dto.InvoiceAllowance
	taxes       []dto.InvoiceTax
	createdAt   time.Time
	updatedAt   time.Time
}

// LineAmounts is one invoice line split into its base and one amount per tax.
type LineAmounts struct {
	Base decimal.Decimal
	VAT  decimal.Decimal
	ICO  decimal.Decimal
}

// SplitLine splits the tax-inclusive line total (price × qty) into base and taxes, rounding
// each tax once per line. Rounding per unit and multiplying drifts from the provider's
// line base × rate and gets the invoice rejected.
func SplitLine(unitPriceWithTaxes decimal.Decimal, quantity int, vatRate, icoRate decimal.Decimal) LineAmounts {
	gross := unitPriceWithTaxes.Mul(decimal.NewFromInt(int64(quantity)))
	divisor := decimal.NewFromInt(1).Add(vatRate).Add(icoRate)
	vat := gross.Mul(vatRate).Div(divisor).Round(2)
	ico := gross.Mul(icoRate).Div(divisor).Round(2)
	return LineAmounts{Base: gross.Sub(vat).Sub(ico), VAT: vat, ICO: ico}
}

// NewBillProduct builds an invoice line from the product's tax-inclusive unit price and its
// VAT/ICO rates (fractions, 0.08 = 8%).
func NewBillProduct(
	productID string,
	quantity int,
	unitPriceWithTaxes decimal.Decimal,
	name string,
	description *string,
	category string,
	code string,
	allowance []dto.InvoiceAllowance,
	vatRate decimal.Decimal,
	icoRate decimal.Decimal,
) *BillProduct {
	line := SplitLine(unitPriceWithTaxes, quantity, vatRate, icoRate)
	taxes := []dto.InvoiceTax{}
	if vatRate.IsPositive() {
		taxes = append(taxes, newInvoiceTax(dto.TaxCodeVAT, line.VAT, vatRate))
	}
	if icoRate.IsPositive() {
		taxes = append(taxes, newInvoiceTax(dto.TaxCodeICO, line.ICO, icoRate))
	}
	return newBillProduct(productID, quantity, line.Base, name, description, category, code, allowance, taxes)
}

func newInvoiceTax(code dto.TaxCode, amount, rate decimal.Decimal) dto.InvoiceTax {
	return dto.InvoiceTax{
		TaxCode:   code,
		TaxAmount: amount.StringFixed(2),
		Percent:   rate.Mul(decimal.NewFromInt(100)).StringFixed(2),
	}
}

func newBillProduct(
	productID string,
	quantity int,
	base decimal.Decimal,
	name string,
	description *string,
	category string,
	code string,
	allowance []dto.InvoiceAllowance,
	taxes []dto.InvoiceTax,
) *BillProduct {
	unitPrice := decimal.Zero
	if quantity > 0 {
		unitPrice = base.Div(decimal.NewFromInt(int64(quantity))).Round(2)
	}
	now := time.Now()
	return &BillProduct{
		id:          productID,
		quantity:    quantity,
		unitPrice:   unitPrice,
		total:       base,
		name:        name,
		description: description,
		category:    category,
		code:        code,
		allowance:   allowance,
		taxes:       taxes,
		createdAt:   now,
		updatedAt:   now,
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

// Total is the line base: the tax-inclusive line total minus the line's taxes.
func (bp *BillProduct) Total() decimal.Decimal {
	return bp.total
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
		Total:       bp.total,
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
// stored base as line total and the stored ICO at the stored rate, so nothing is rounded again.
func DeliveryFeeItem(b *dto.Bill) (dto.BillProduct, bool) {
	if b.DeliveryFee == nil || !b.DeliveryFee.IsPositive() || b.DeliveryFeeBase == nil || b.DeliveryFeeICO == nil ||
		b.DeliveryFeeICORate == nil {
		return dto.BillProduct{}, false
	}
	name := deliveryFeeItemName
	taxes := []dto.InvoiceTax{newInvoiceTax(dto.TaxCodeICO, *b.DeliveryFeeICO, *b.DeliveryFeeICORate)}
	return newBillProduct("", 1, *b.DeliveryFeeBase, name, &name, name, deliveryFeeItemCode, []dto.InvoiceAllowance{},
		taxes).ToDTO(), true
}

// ApplyLineTotals sets the bill's net, VAT, ICO and tax totals to the sums of its lines, so
// the invoice header always agrees with the items. Pay and discount amounts are left as
// stored: they are what was charged.
func ApplyLineTotals(b *dto.Bill) error {
	total, vat, ico := decimal.Zero, decimal.Zero, decimal.Zero
	for _, p := range b.Products {
		total = total.Add(p.Total)
		for _, tax := range p.Taxes {
			amount, err := decimal.NewFromString(tax.TaxAmount)
			if err != nil {
				return billError.NewInvalidTaxAmountError(tax.TaxAmount)
			}
			switch tax.TaxCode {
			case dto.TaxCodeVAT:
				vat = vat.Add(amount)
			case dto.TaxCodeICO:
				ico = ico.Add(amount)
			}
		}
	}
	b.TotalAmount, b.VAT, b.ICO, b.TaxAmount = total, vat, ico, vat.Add(ico)
	return nil
}
