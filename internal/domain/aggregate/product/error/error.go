package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

// ProductErrorCode defines error codes for product aggregate
type ProductErrorCode string

const (
	CodeInvalidRequest        ProductErrorCode = "PRODUCT_INVALID_REQUEST"
	CodeMissingName           ProductErrorCode = "PRODUCT_MISSING_NAME"
	CodeMissingCategory       ProductErrorCode = "PRODUCT_MISSING_CATEGORY"
	CodeMissingSKU            ProductErrorCode = "PRODUCT_MISSING_SKU"
	CodeInvalidPrice          ProductErrorCode = "PRODUCT_INVALID_PRICE"
	CodeInvalidVAT            ProductErrorCode = "PRODUCT_INVALID_VAT"
	CodeInvalidICO            ProductErrorCode = "PRODUCT_INVALID_ICO"
	CodeInvalidTaxCalculation ProductErrorCode = "PRODUCT_INVALID_TAX_CALCULATION"
)

// NewInvalidRequestError creates an error for invalid request
func NewInvalidRequestError(message string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidRequest), message)
}

// NewInvalidRequestErrorWithField creates an error for invalid request with field value context
func NewInvalidRequestErrorWithField(message string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidRequest), message)
}

// NewMissingNameError creates an error for missing name
func NewMissingNameError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingName), "name is required")
}

// NewMissingCategoryError creates an error for missing category
func NewMissingCategoryError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingCategory), "category is required")
}

// NewMissingSKUError creates an error for missing SKU
func NewMissingSKUError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingSKU), "sku is required")
}

// NewInvalidPriceErrorWithField creates an error for invalid price with field value context
func NewInvalidPriceErrorWithField(message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidPrice), message, fieldValue)
}

// NewInvalidVATError creates an error for invalid VAT
func NewInvalidVATError(message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidVAT), message, fieldValue)
}

// NewInvalidICOError creates an error for invalid ICO
func NewInvalidICOError(message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidICO), message, fieldValue)
}

// NewInvalidTaxCalculationError creates an error for invalid tax calculation
func NewInvalidTaxCalculationError(message string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidTaxCalculation), message)
}

// NewInvalidTaxCalculationErrorWithField creates an error for invalid tax calculation with field value context
func NewInvalidTaxCalculationErrorWithField(message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidTaxCalculation), message, fieldValue)
}

// Wrap wraps an existing error with a product error
func Wrap(err error, code ProductErrorCode, message string) *baseError.BaseError {
	return baseError.Wrap(err, baseError.ErrorCode(code), message)
}

// WrapWithField wraps an existing error with a product error and field value context
func WrapWithField(err error, code ProductErrorCode, message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.WrapWithField(err, baseError.ErrorCode(code), message, fieldValue)
}
