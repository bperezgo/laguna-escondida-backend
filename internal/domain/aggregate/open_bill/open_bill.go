package open_bill

import (
	"time"

	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Aggregate struct {
	id                 string
	temporalIdentifier string
	totalAmount        decimal.Decimal
	status             *shared.CommandStatus
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

	status, err := shared.NewCommandStatus(dto.CommandStatusCreated)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Aggregate{
		id:                 openBillID,
		temporalIdentifier: temporalIdentifier,
		totalAmount:        totalAmount,
		status:             status,
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
		Status:             a.status.Value(),
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

	derivedStatus := deriveStatusFromProducts(products)
	status, err := shared.NewCommandStatus(derivedStatus)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                 id,
		temporalIdentifier: temporalIdentifier,
		totalAmount:        totalAmount,
		status:             status,
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

func (a *Aggregate) UpdateInfo(temporalIdentifier *string, descriptor *string) {
	if temporalIdentifier != nil {
		a.temporalIdentifier = *temporalIdentifier
	}
	if descriptor != nil {
		a.descriptor = descriptor
	}
	a.updatedAt = time.Now()
}

func (a *Aggregate) Status() dto.CommandStatus {
	return a.status.Value()
}

func (a *Aggregate) IsCreated() bool {
	return a.status.IsCreated()
}

func (a *Aggregate) IsCompleted() bool {
	return a.status.IsCompleted()
}

func (a *Aggregate) IsCancelled() bool {
	return a.status.IsCancelled()
}

func (a *Aggregate) GetProduct(openBillProductID string) *OpenBillProduct {
	for _, product := range a.products {
		if product.ID() == openBillProductID {
			return product
		}
	}
	return nil
}

func (a *Aggregate) CompleteProduct(openBillProductID string) error {
	product := a.GetProduct(openBillProductID)
	if product == nil {
		return openBillError.ErrOpenBillProductNotFound
	}

	if err := product.Complete(); err != nil {
		return err
	}

	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) UncompleteProduct(openBillProductID string) error {
	product := a.GetProduct(openBillProductID)
	if product == nil {
		return openBillError.ErrOpenBillProductNotFound
	}

	if err := product.Uncomplete(); err != nil {
		return err
	}

	// A product back in "created" means the bill can no longer be completed or
	// cancelled — reopen the header if it had been auto-finalized (e.g. undo
	// during the "Lista" flash, after the last line had just completed it).
	if !a.status.IsCreated() {
		status, err := shared.NewCommandStatus(dto.CommandStatusCreated)
		if err != nil {
			return err
		}
		a.status = status
	}

	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) CancelProduct(openBillProductID string) error {
	product := a.GetProduct(openBillProductID)
	if product == nil {
		return openBillError.ErrOpenBillProductNotFound
	}

	if err := product.Cancel(); err != nil {
		return err
	}

	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) SetProductInProgress(openBillProductID string) error {
	product := a.GetProduct(openBillProductID)
	if product == nil {
		return openBillError.ErrOpenBillProductNotFound
	}

	if err := product.SetInProgress(); err != nil {
		return err
	}

	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) CanComplete() bool {
	if !a.status.IsCreated() {
		return false
	}

	hasCompleted := false
	for _, product := range a.products {
		if product.IsCompleted() {
			hasCompleted = true
		} else if !product.IsCancelled() {
			return false
		}
	}

	return hasCompleted
}

func (a *Aggregate) CanCancel() bool {
	if !a.status.IsCreated() {
		return false
	}

	for _, product := range a.products {
		if !product.IsCancelled() {
			return false
		}
	}

	return true
}

func (a *Aggregate) TryComplete() (bool, error) {
	if !a.CanComplete() {
		return false, nil
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCompleted)
	if err != nil {
		return false, err
	}

	a.status = status
	a.updatedAt = time.Now()
	return true, nil
}

func (a *Aggregate) TryCancel() (bool, error) {
	if !a.CanCancel() {
		return false, nil
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCancelled)
	if err != nil {
		return false, err
	}

	a.status = status
	a.updatedAt = time.Now()
	return true, nil
}

func deriveStatusFromProducts(products []*OpenBillProduct) dto.CommandStatus {
	if len(products) == 0 {
		return dto.CommandStatusCreated
	}

	hasCompleted := false
	allCancelled := true
	allFinalized := true

	for _, product := range products {
		if product.IsCompleted() {
			hasCompleted = true
			allCancelled = false
		} else if product.IsCancelled() {
			// Product is cancelled, continue
		} else {
			// Product is still in created or in_progress status
			allFinalized = false
			allCancelled = false
		}
	}

	if allFinalized && hasCompleted {
		return dto.CommandStatusCompleted
	}

	if allCancelled {
		return dto.CommandStatusCancelled
	}

	return dto.CommandStatusCreated
}
