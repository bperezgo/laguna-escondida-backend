package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type ExpenseErrorCode string

const (
	CodeInvalidRequest   ExpenseErrorCode = "EXPENSE_INVALID_REQUEST"
	CodeInvalidAmount    ExpenseErrorCode = "EXPENSE_INVALID_AMOUNT"
	CodeMissingCategory  ExpenseErrorCode = "EXPENSE_MISSING_CATEGORY"
	CodeEmptyDescription ExpenseErrorCode = "EXPENSE_EMPTY_DESCRIPTION"
)

func NewInvalidRequestError(message string) *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeInvalidRequest),
		message,
	)
}

func NewInvalidAmountError(value string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidAmount),
		"amount must be a positive number",
		value,
	)
}

func NewMissingCategoryError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingCategory),
		"category ID is required",
	)
}

func NewEmptyDescriptionError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeEmptyDescription),
		"description cannot be empty",
	)
}
