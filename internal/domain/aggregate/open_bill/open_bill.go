package open_bill

import (
	"time"

	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Aggregate struct {
	id                 string
	temporalIdentifier string
	totalAmount        decimal.Decimal
	descriptor         *string
	products           []*OpenBillProduct
	createdByID        string
	createdAt          time.Time
	updatedAt          time.Time
}

func NewAggregate(req *dto.CreateOrderRequest, totalAmount decimal.Decimal, createdByID string) (*Aggregate, error) {
	if _, err := uuid.Parse(req.OpenBillID); err != nil {
		return nil, openBillError.ErrInvalidOpenBillID
	}

	products := make([]*OpenBillProduct, 0, len(req.Products))
	for _, item := range req.Products {
		product, err := NewOpenBillProduct(item.OpenBillProductID, item.ProductID, item.Quantity, item.Notes)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	now := time.Now()
	return &Aggregate{
		id:                 req.OpenBillID,
		temporalIdentifier: req.TemporalIdentifier,
		totalAmount:        totalAmount,
		descriptor:         req.Descriptor,
		products:           products,
		createdByID:        createdByID,
		createdAt:          now,
		updatedAt:          now,
	}, nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) TemporalIdentifier() string {
	return a.temporalIdentifier
}

func (a *Aggregate) TotalAmount() decimal.Decimal {
	return a.totalAmount
}

func (a *Aggregate) Descriptor() *string {
	return a.descriptor
}

func (a *Aggregate) Products() []*OpenBillProduct {
	return a.products
}

func (a *Aggregate) CreatedByID() string {
	return a.createdByID
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
}

func (a *Aggregate) UpdatedAt() time.Time {
	return a.updatedAt
}

func (a *Aggregate) ToDTO() *dto.OpenBill {
	return &dto.OpenBill{
		ID:                 a.id,
		TemporalIdentifier: a.temporalIdentifier,
		TotalAmount:        a.totalAmount,
		Descriptor:         a.descriptor,
		CreatedAt:          a.createdAt,
		UpdatedAt:          a.updatedAt,
	}
}
