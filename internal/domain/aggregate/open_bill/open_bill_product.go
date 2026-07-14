package open_bill

import (
	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

type OpenBillProduct struct {
	id          string
	productID   string
	quantity    int
	notes       *string
	status      *shared.CommandStatus
	area        *string
	priority    int
	createdByID string
}

func NewOpenBillProduct(id, productID string, quantity int, notes *string, area *string, priority int, createdByID string) (*OpenBillProduct, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, openBillError.ErrInvalidOpenBillProductID
	}

	if _, err := uuid.Parse(productID); err != nil {
		return nil, openBillError.ErrInvalidProductID
	}

	if quantity <= 0 {
		return nil, openBillError.ErrInvalidQuantity
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCreated)
	if err != nil {
		return nil, err
	}

	return &OpenBillProduct{
		id:          id,
		productID:   productID,
		quantity:    quantity,
		notes:       notes,
		status:      status,
		area:        area,
		priority:    priority,
		createdByID: createdByID,
	}, nil
}

func NewOpenBillProductFromRepository(id, productID string, quantity int, notes *string, status dto.CommandStatus, area *string, priority int, createdByID string) (*OpenBillProduct, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, openBillError.ErrInvalidOpenBillProductID
	}

	if _, err := uuid.Parse(productID); err != nil {
		return nil, openBillError.ErrInvalidProductID
	}

	if quantity <= 0 {
		return nil, openBillError.ErrInvalidQuantity
	}

	statusVO, err := shared.NewCommandStatus(status)
	if err != nil {
		return nil, err
	}

	return &OpenBillProduct{
		id:          id,
		productID:   productID,
		quantity:    quantity,
		notes:       notes,
		status:      statusVO,
		area:        area,
		priority:    priority,
		createdByID: createdByID,
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

func (p *OpenBillProduct) Status() dto.CommandStatus {
	return p.status.Value()
}

func (p *OpenBillProduct) Area() *string {
	return p.area
}

func (p *OpenBillProduct) Priority() int {
	return p.priority
}

func (p *OpenBillProduct) CreatedByID() string {
	return p.createdByID
}

func (p *OpenBillProduct) IsCreated() bool {
	return p.status.IsCreated()
}

func (p *OpenBillProduct) IsCompleted() bool {
	return p.status.IsCompleted()
}

func (p *OpenBillProduct) IsCancelled() bool {
	return p.status.IsCancelled()
}

func (p *OpenBillProduct) IsInProgress() bool {
	return p.status.IsInProgress()
}

func (p *OpenBillProduct) IsFinalized() bool {
	return p.status.IsCompleted() || p.status.IsCancelled()
}

func (p *OpenBillProduct) Complete() error {
	if p.status.IsCompleted() {
		return openBillError.ErrProductAlreadyCompleted
	}

	if p.status.IsCancelled() {
		return openBillError.ErrCannotCompleteProduct
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCompleted)
	if err != nil {
		return err
	}

	p.status = status
	return nil
}

// Uncomplete reverts a completed product back to "created" (undo a strike-through
// in the kitchen). Only a completed product can be uncompleted.
func (p *OpenBillProduct) Uncomplete() error {
	if !p.status.IsCompleted() {
		return openBillError.ErrProductNotCompleted
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCreated)
	if err != nil {
		return err
	}

	p.status = status
	return nil
}

func (p *OpenBillProduct) Cancel() error {
	if p.status.IsCancelled() {
		return openBillError.ErrProductAlreadyCancelled
	}

	if p.status.IsCompleted() {
		return openBillError.ErrCannotCancelProduct
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCancelled)
	if err != nil {
		return err
	}

	p.status = status
	return nil
}

func (p *OpenBillProduct) SetInProgress() error {
	if p.status.IsCancelled() {
		return openBillError.ErrCannotSetInProgressFromCancelled
	}

	if p.status.IsInProgress() {
		return nil
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusInProgress)
	if err != nil {
		return err
	}

	p.status = status
	return nil
}
