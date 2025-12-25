package open_bill

import (
	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"

	"github.com/google/uuid"
)

type OpenBillProduct struct {
	id        string
	productID string
	quantity  int
	notes     *string
}

func NewOpenBillProduct(id, productID string, quantity int, notes *string) (*OpenBillProduct, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, openBillError.ErrInvalidOpenBillProductID
	}

	if _, err := uuid.Parse(productID); err != nil {
		return nil, openBillError.ErrInvalidProductID
	}

	if quantity <= 0 {
		return nil, openBillError.ErrInvalidQuantity
	}

	return &OpenBillProduct{
		id:        id,
		productID: productID,
		quantity:  quantity,
		notes:     notes,
	}, nil
}

func (p *OpenBillProduct) ID() string {
	return p.id
}

func (p *OpenBillProduct) ProductID() string {
	return p.productID
}

func (p *OpenBillProduct) Quantity() int {
	return p.quantity
}

func (p *OpenBillProduct) Notes() *string {
	return p.notes
}
