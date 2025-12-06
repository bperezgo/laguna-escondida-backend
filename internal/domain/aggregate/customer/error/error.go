package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type CustomerErrorCode string

const (
	CodeInvalidDocumentType   CustomerErrorCode = "CUSTOMER_INVALID_DOCUMENT_TYPE"
	CodeInvalidEmail          CustomerErrorCode = "CUSTOMER_INVALID_EMAIL"
	CodeInvalidDocumentNumber CustomerErrorCode = "CUSTOMER_INVALID_DOCUMENT_NUMBER"
	CodeMissingName           CustomerErrorCode = "CUSTOMER_MISSING_NAME"
	CodeMissingEmail          CustomerErrorCode = "CUSTOMER_MISSING_EMAIL"
	CodeMissingDocumentNumber CustomerErrorCode = "CUSTOMER_MISSING_DOCUMENT_NUMBER"
)

func NewInvalidDocumentTypeError(documentType string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidDocumentType),
		"document type must be either CC or NIT",
		documentType,
	)
}

func NewInvalidEmailError(email string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidEmail),
		"email format is invalid",
		email,
	)
}

func NewInvalidDocumentNumberError(documentNumber string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidDocumentNumber),
		"document number can only contain numbers and hyphens",
		documentNumber,
	)
}

func NewMissingNameError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingName),
		"name is required",
	)
}

func NewMissingEmailError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingEmail),
		"email is required",
	)
}

func NewMissingDocumentNumberError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingDocumentNumber),
		"document number is required",
	)
}

