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
	CodeInvalidSKU            ProductErrorCode = "PRODUCT_INVALID_SKU"
	CodeInvalidPrice          ProductErrorCode = "PRODUCT_INVALID_PRICE"
	CodeInvalidVAT            ProductErrorCode = "PRODUCT_INVALID_VAT"
	CodeInvalidICO            ProductErrorCode = "PRODUCT_INVALID_ICO"
	CodeInvalidTaxCalculation ProductErrorCode = "PRODUCT_INVALID_TAX_CALCULATION"
	CodeMissingProductType    ProductErrorCode = "PRODUCT_MISSING_PRODUCT_TYPE"
	CodeInvalidProductType    ProductErrorCode = "PRODUCT_INVALID_PRODUCT_TYPE"
	CodeMissingUnitOfMeasure  ProductErrorCode = "PRODUCT_MISSING_UNIT_OF_MEASURE"
	CodeInvalidUnitOfMeasure  ProductErrorCode = "PRODUCT_INVALID_UNIT_OF_MEASURE"
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

// NewInvalidSKUError creates an error for invalid SKU format
func NewInvalidSKUError(sku string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidSKU), "sku must contain only alphanumeric characters (a-z, A-Z, 0-9)", sku)
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

// NewMissingProductTypeError creates an error for missing product type
func NewMissingProductTypeError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingProductType), "product_type is required")
}

// NewInvalidProductTypeError creates an error for invalid product type
func NewInvalidProductTypeError(value string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidProductType), "product_type must be one of: SELLABLE, INGREDIENT, COMPOSITE, BOTH", value)
}

// NewMissingUnitOfMeasureError creates an error for missing unit of measure
func NewMissingUnitOfMeasureError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingUnitOfMeasure), "unit_of_measure is required")
}

// NewInvalidUnitOfMeasureError creates an error for invalid unit of measure
func NewInvalidUnitOfMeasureError(value string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(baseError.ErrorCode(CodeInvalidUnitOfMeasure), "unit_of_measure must be one of: unit, kg, g, l, ml", value)
}

// Wrap wraps an existing error with a product error
func Wrap(err error, code ProductErrorCode, message string) *baseError.BaseError {
	return baseError.Wrap(err, baseError.ErrorCode(code), message)
}

// WrapWithField wraps an existing error with a product error and field value context
func WrapWithField(err error, code ProductErrorCode, message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.WrapWithField(err, baseError.ErrorCode(code), message, fieldValue)
}
