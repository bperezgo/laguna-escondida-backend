package expense

import (
	"time"

	expenseError "laguna-escondida/backend/internal/domain/aggregate/expense/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Aggregate struct {
	id          string
	categoryID  string
	supplierID  *string
	amount      decimal.Decimal
	description string
	expenseDate time.Time
	reference   *string
	notes       *string
	createdAt   time.Time
}

func NewAggregateFromCreateRequest(req *dto.CreateExpenseRequest) (*Aggregate, error) {
	if req == nil {
		return nil, expenseError.NewInvalidRequestError("request cannot be nil")
	}

	if req.CategoryID == "" {
		return nil, expenseError.NewMissingCategoryError()
	}

	if req.Description == "" {
		return nil, expenseError.NewEmptyDescriptionError()
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, expenseError.NewInvalidAmountError(req.Amount)
	}

	expenseDate := time.Now()
	if req.ExpenseDate != nil {
		expenseDate = *req.ExpenseDate
	}

	now := time.Now()
	return &Aggregate{
		id:          uuid.Must(uuid.NewV7()).String(),
		categoryID:  req.CategoryID,
		supplierID:  req.SupplierID,
		amount:      amount,
		description: req.Description,
		expenseDate: expenseDate,
		reference:   req.Reference,
		notes:       req.Notes,
		createdAt:   now,
	}, nil
}

func NewAggregateFromRepository(
	id string,
	categoryID string,
	supplierID *string,
	amount decimal.Decimal,
	description string,
	expenseDate time.Time,
	reference *string,
	notes *string,
	createdAt time.Time,
) *Aggregate {
	return &Aggregate{
		id:          id,
		categoryID:  categoryID,
		supplierID:  supplierID,
		amount:      amount,
		description: description,
		expenseDate: expenseDate,
		reference:   reference,
		notes:       notes,
		createdAt:   createdAt,
	}
}

func (a *Aggregate) Update(req *dto.UpdateExpenseRequest) error {
	if req == nil {
		return expenseError.NewInvalidRequestError("request cannot be nil")
	}

	if req.CategoryID == "" {
		return expenseError.NewMissingCategoryError()
	}

	if req.Description == "" {
		return expenseError.NewEmptyDescriptionError()
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return expenseError.NewInvalidAmountError(req.Amount)
	}

	a.categoryID = req.CategoryID
	a.supplierID = req.SupplierID
	a.amount = amount
	a.description = req.Description
	if req.ExpenseDate != nil {
		a.expenseDate = *req.ExpenseDate
	}
	a.reference = req.Reference
	a.notes = req.Notes

	return nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) CategoryID() string {
	return a.categoryID
}

func (a *Aggregate) SupplierID() *string {
	return a.supplierID
}

func (a *Aggregate) Amount() decimal.Decimal {
	return a.amount
}

func (a *Aggregate) Description() string {
	return a.description
}

func (a *Aggregate) ExpenseDate() time.Time {
	return a.expenseDate
}

func (a *Aggregate) Reference() *string {
	return a.reference
}

func (a *Aggregate) Notes() *string {
	return a.notes
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
}

func (a *Aggregate) ToDTO() *dto.Expense {
	return &dto.Expense{
		ID:          a.id,
		CategoryID:  a.categoryID,
		SupplierID:  a.supplierID,
		Amount:      a.amount,
		Description: a.description,
		ExpenseDate: a.expenseDate,
		Reference:   a.reference,
		Notes:       a.notes,
		CreatedAt:   a.createdAt,
	}
}
