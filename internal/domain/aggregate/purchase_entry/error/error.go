package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type PurchaseEntryErrorCode string

const (
	CodeInvalidRequest   PurchaseEntryErrorCode = "PURCHASE_ENTRY_INVALID_REQUEST"
	CodeInvalidQuantity  PurchaseEntryErrorCode = "PURCHASE_ENTRY_INVALID_QUANTITY"
	CodeInvalidUnitCost  PurchaseEntryErrorCode = "PURCHASE_ENTRY_INVALID_UNIT_COST"
	CodeEmptyItems       PurchaseEntryErrorCode = "PURCHASE_ENTRY_EMPTY_ITEMS"
	CodeMissingSupplier  PurchaseEntryErrorCode = "PURCHASE_ENTRY_MISSING_SUPPLIER"
)

func NewInvalidRequestError(message string) *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeInvalidRequest),
		message,
	)
}

func NewInvalidQuantityError(value string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidQuantity),
		"quantity must be a positive number",
		value,
	)
}

func NewInvalidUnitCostError(value string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidUnitCost),
		"unit cost must be a non-negative number",
		value,
	)
}

func NewEmptyItemsError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeEmptyItems),
		"purchase entry must have at least one item",
	)
}

func NewMissingSupplierError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingSupplier),
		"supplier ID is required",
	)
}
