package command_item

import (
	commandItemError "laguna-escondida/backend/internal/domain/aggregate/command_item/error"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
	"time"
)

type Aggregate struct {
	id                string
	openBillProductID string
	productID         string
	productName       string
	quantity          int
	notes             *string
	status            *shared.CommandStatus
	priority          *Priority
	createdAt         time.Time
}

func NewCommandItem(
	id string,
	openBillProductID string,
	productID string,
	productName string,
	quantity int,
	notes *string,
	priorityValue int,
	createdAt time.Time,
) (*Aggregate, error) {
	if id == "" {
		return nil, commandItemError.NewMissingIDError()
	}

	if openBillProductID == "" {
		return nil, commandItemError.NewMissingOpenBillProductIDError()
	}

	if productID == "" {
		return nil, commandItemError.NewMissingProductIDError()
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCreated)
	if err != nil {
		return nil, err
	}

	priority, err := NewPriority(priorityValue)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                id,
		openBillProductID: openBillProductID,
		productID:         productID,
		productName:       productName,
		quantity:          quantity,
		notes:             notes,
		status:            status,
		priority:          priority,
		createdAt:         createdAt,
	}, nil
}

func NewCommandItemFromDTO(item *dto.CommandItem) (*Aggregate, error) {
	if item.ID == "" {
		return nil, commandItemError.NewMissingIDError()
	}

	if item.OpenBillProductID == "" {
		return nil, commandItemError.NewMissingOpenBillProductIDError()
	}

	if item.ProductID == "" {
		return nil, commandItemError.NewMissingProductIDError()
	}

	status, err := shared.NewCommandStatus(item.Status)
	if err != nil {
		return nil, err
	}

	priority, err := NewPriority(item.Priority)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                item.ID,
		openBillProductID: item.OpenBillProductID,
		productID:         item.ProductID,
		productName:       item.ProductName,
		quantity:          item.Quantity,
		notes:             item.Notes,
		status:            status,
		priority:          priority,
		createdAt:         item.CreatedAt,
	}, nil
}

func NewCommandItemFromRepository(
	id string,
	openBillProductID string,
	productID string,
	productName string,
	quantity int,
	notes *string,
	status dto.CommandStatus,
	priority int,
	createdAt time.Time,
) (*Aggregate, error) {
	if id == "" {
		return nil, commandItemError.NewMissingIDError()
	}

	if openBillProductID == "" {
		return nil, commandItemError.NewMissingOpenBillProductIDError()
	}

	if productID == "" {
		return nil, commandItemError.NewMissingProductIDError()
	}

	statusVO, err := shared.NewCommandStatus(status)
	if err != nil {
		return nil, err
	}

	priorityVO, err := NewPriority(priority)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                id,
		openBillProductID: openBillProductID,
		productID:         productID,
		productName:       productName,
		quantity:          quantity,
		notes:             notes,
		status:            statusVO,
		priority:          priorityVO,
		createdAt:         createdAt,
	}, nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) OpenBillProductID() string {
	return a.openBillProductID
}

func (a *Aggregate) ProductID() string {
	return a.productID
}

func (a *Aggregate) ProductName() string {
	return a.productName
}

func (a *Aggregate) Quantity() int {
	return a.quantity
}

func (a *Aggregate) Notes() *string {
	return a.notes
}

func (a *Aggregate) Status() dto.CommandStatus {
	return a.status.Value()
}

func (a *Aggregate) Priority() int {
	return a.priority.Value()
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
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

// Complete marks the command item as completed
func (a *Aggregate) Complete() error {
	if a.status.IsCompleted() {
		return commandItemError.NewAlreadyCompletedError()
	}

	if a.status.IsCancelled() {
		return commandItemError.NewCannotCompleteError()
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCompleted)
	if err != nil {
		return err
	}

	a.status = status
	return nil
}

// Cancel marks the command item as cancelled
func (a *Aggregate) Cancel() error {
	if a.status.IsCancelled() {
		return commandItemError.NewAlreadyCancelledError()
	}

	if a.status.IsCompleted() {
		return commandItemError.NewCannotCancelError()
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCancelled)
	if err != nil {
		return err
	}

	a.status = status
	return nil
}

// Update updates the quantity and notes of the command item
func (a *Aggregate) Update(quantity int, notes *string) error {
	if a.status.IsCancelled() {
		return commandItemError.NewCannotUpdateCancelledError()
	}

	if a.status.IsCompleted() {
		return commandItemError.NewCannotUpdateCompletedError()
	}

	a.quantity = quantity
	a.notes = notes
	return nil
}

// ToDTO converts the aggregate to a DTO
func (a *Aggregate) ToDTO() *dto.CommandItem {
	return &dto.CommandItem{
		ID:                a.id,
		OpenBillProductID: a.openBillProductID,
		ProductID:         a.productID,
		ProductName:       a.productName,
		Quantity:          a.quantity,
		Notes:             a.notes,
		Status:            a.status.Value(),
		Priority:          a.priority.Value(),
	}
}

// HasSameNotes compares the item's notes with another notes value
func (a *Aggregate) HasSameNotes(other *string) bool {
	if a.notes == nil && other == nil {
		return true
	}
	if a.notes == nil || other == nil {
		return false
	}
	return *a.notes == *other
}
