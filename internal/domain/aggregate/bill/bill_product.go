package bill

import (
	"laguna-escondida/backend/internal/domain/dto"
	"time"
)

type BillProduct struct {
	id        string
	quantity  int
	unitPrice float64
	allowance []dto.InvoiceAllowance
	taxes     []dto.InvoiceTax
	createdAt time.Time
	updatedAt time.Time
}

func NewBillProduct(productID string, quantity int, unitPrice float64, allowance []dto.InvoiceAllowance, taxes []dto.InvoiceTax) *BillProduct {
	return &BillProduct{
		id:        productID,
		quantity:  quantity,
		unitPrice: unitPrice,
		allowance: allowance,
		taxes:     taxes,
		createdAt: time.Now(),
		updatedAt: time.Now(),
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
