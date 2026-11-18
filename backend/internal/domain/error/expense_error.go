package error

import "errors"

var (
	ErrExpenseCategoryNotFound       = errors.New("expense category not found")
	ErrExpenseCategoryCreationFailed = errors.New("failed to create expense category")
	ErrExpenseCategoryUpdateFailed   = errors.New("failed to update expense category")
	ErrExpenseCategoryCodeExists     = errors.New("expense category code already exists")

	ErrExpenseNotFound       = errors.New("expense not found")
	ErrExpenseCreationFailed = errors.New("failed to create expense")
	ErrExpenseUpdateFailed   = errors.New("failed to update expense")
	ErrExpenseDeleteFailed   = errors.New("failed to delete expense")
)
