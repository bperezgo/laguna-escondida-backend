package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

// ProductErrorCode defines error codes for product aggregate
type ProductErrorCode string

const (
	CodeProductsCannotBeEmpty  ProductErrorCode = "PRODUCTS_CANNOT_BE_EMPTY"
	CodeInvalidAllowanceAmount ProductErrorCode = "INVALID_ALLOWANCE_AMOUNT"
	CodeInvalidTaxAmount       ProductErrorCode = "INVALID_TAX_AMOUNT"
	CodeInvalidPaymentCode     ProductErrorCode = "INVALID_PAYMENT_CODE"
)

// NewProductsCannotBeEmptyError creates an error for products cannot be empty
func NewProductsCannotBeEmptyError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeProductsCannotBeEmpty), "products cannot be empty")
}

// NewInvalidAllowanceAmountError creates an error for invalid allowance amount
func NewInvalidAllowanceAmountError(amount string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidAllowanceAmount), "invalid allowance amount: "+amount)
}

// NewInvalidTaxAmountError creates an error for invalid tax amount
func NewInvalidTaxAmountError(amount string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidTaxAmount), "invalid tax amount: "+amount)
}

// NewInvalidPaymentCodeError creates an error for invalid payment code
func NewInvalidPaymentCodeError(code string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidPaymentCode), "invalid payment code: "+code)
}
