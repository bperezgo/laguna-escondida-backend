package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type SupplierErrorCode string

const (
	CodeMissingName    SupplierErrorCode = "SUPPLIER_MISSING_NAME"
	CodeInvalidEmail   SupplierErrorCode = "SUPPLIER_INVALID_EMAIL"
	CodeInvalidPhone   SupplierErrorCode = "SUPPLIER_INVALID_PHONE"
	CodeInvalidRequest SupplierErrorCode = "SUPPLIER_INVALID_REQUEST"
)

func NewMissingNameError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingName),
		"supplier name is required",
	)
}

func NewInvalidEmailError(email string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidEmail),
		"supplier email format is invalid",
		email,
	)
}

func NewInvalidPhoneError(phone string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidPhone),
		"supplier phone format is invalid",
		phone,
	)
}

func NewInvalidRequestError(message string) *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeInvalidRequest),
		message,
	)
}
