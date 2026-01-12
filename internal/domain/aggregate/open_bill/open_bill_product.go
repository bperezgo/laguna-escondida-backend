package open_bill

import (
	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

type OpenBillProduct struct {
	id        string
	productID string
	quantity  int
	notes     *string
	status    *shared.CommandStatus
	area      *string
	priority  int
}

func NewOpenBillProduct(id, productID string, quantity int, notes *string, area *string, priority int) (*OpenBillProduct, error) {
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
		id:        id,
		productID: productID,
		quantity:  quantity,
		notes:     notes,
		status:    status,
		area:      area,
		priority:  priority,
	}, nil
}

func NewOpenBillProductFromRepository(id, productID string, quantity int, notes *string, status dto.CommandStatus, area *string, priority int) (*OpenBillProduct, error) {
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
		id:        id,
		productID: productID,
		quantity:  quantity,
		notes:     notes,
		status:    statusVO,
		area:      area,
		priority:  priority,
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
