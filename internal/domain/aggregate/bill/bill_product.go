package bill

import (
	"laguna-escondida/backend/internal/domain/dto"
	"strconv"
	"time"
)

type BillProduct struct {
	id          string
	quantity    int
	unitPrice   float64
	description *string
	brand       *string
	model       *string
	code        string
	allowance   []dto.InvoiceAllowance
	taxes       []dto.InvoiceTax
	createdAt   time.Time
	updatedAt   time.Time
}

func NewBillProduct(productID string, quantity int, unitPrice float64, description *string, brand *string, model *string, code string, allowance []dto.InvoiceAllowance, vat float64, ico float64) *BillProduct {
	baseAmount := unitPrice * float64(quantity)
	taxes := []dto.InvoiceTax{}

	if vat > 0 {
		vatAmount := baseAmount * vat
		taxes = append(taxes, dto.InvoiceTax{
			TaxCode:   dto.TaxCodeVAT,
			TaxAmount: strconv.FormatFloat(vatAmount, 'f', 2, 64),
			Percent:   strconv.FormatFloat(vat*100, 'f', 2, 64),
		})
	}

	if ico > 0 {
		icoAmount := baseAmount * ico
		taxes = append(taxes, dto.InvoiceTax{
			TaxCode:   dto.TaxCodeICO,
			TaxAmount: strconv.FormatFloat(icoAmount, 'f', 2, 64),
			Percent:   strconv.FormatFloat(ico*100, 'f', 2, 64),
		})
	}

	return &BillProduct{
		id:          productID,
		quantity:    quantity,
		unitPrice:   unitPrice,
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

func (bp *BillProduct) UnitPrice() float64 {
	return bp.unitPrice
}

func (bp *BillProduct) Allowance() []dto.InvoiceAllowance {
	return bp.allowance
}

func (bp *BillProduct) Taxes() []dto.InvoiceTax {
	return bp.taxes
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
