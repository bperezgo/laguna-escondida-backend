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

func NewAggregate(
	openBillID string,
	temporalIdentifier string,
	descriptor *string,
	totalAmount decimal.Decimal,
	products []*OpenBillProduct,
	createdByID string,
) (*Aggregate, error) {
	if _, err := uuid.Parse(openBillID); err != nil {
		return nil, openBillError.ErrInvalidOpenBillID
	}

	now := time.Now()
	return &Aggregate{
		id:                 openBillID,
		temporalIdentifier: temporalIdentifier,
		totalAmount:        totalAmount,
		descriptor:         descriptor,
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

func NewAggregateFromRepository(
	id string,
	temporalIdentifier string,
	totalAmount decimal.Decimal,
	descriptor *string,
	products []*OpenBillProduct,
	createdByID string,
	createdAt time.Time,
	updatedAt time.Time,
) (*Aggregate, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, openBillError.ErrInvalidOpenBillID
	}

	return &Aggregate{
		id:                 id,
		temporalIdentifier: temporalIdentifier,
		totalAmount:        totalAmount,
		descriptor:         descriptor,
		products:           products,
		createdByID:        createdByID,
		createdAt:          createdAt,
		updatedAt:          updatedAt,
	}, nil
}

func (a *Aggregate) UpdateProducts(products []*OpenBillProduct, totalAmount decimal.Decimal) {
	a.products = products
	a.totalAmount = totalAmount
	a.updatedAt = time.Now()
}
