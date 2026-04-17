package support_document

import (
	"github.com/shopspring/decimal"
)

type SupportDocumentProduct struct {
	description string
	quantity    int
	unitPrice   decimal.Decimal
}

func NewSupportDocumentProduct(
	description string,
	quantity int,
	unitPrice decimal.Decimal,
) *SupportDocumentProduct {
	return &SupportDocumentProduct{
		description: description,
		quantity:    quantity,
		unitPrice:   unitPrice,
	}
}

func (p *SupportDocumentProduct) Description() string {
	return p.description
}

func (p *SupportDocumentProduct) Quantity() int {
	return p.quantity
}

func (p *SupportDocumentProduct) UnitPrice() decimal.Decimal {
	return p.unitPrice
}
