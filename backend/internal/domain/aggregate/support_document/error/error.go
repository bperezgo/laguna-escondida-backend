package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type SupportDocumentErrorCode string

const (
	CodeProductsCannotBeEmpty SupportDocumentErrorCode = "PRODUCTS_CANNOT_BE_EMPTY"
	CodeProviderRequired      SupportDocumentErrorCode = "PROVIDER_REQUIRED"
)

func NewProductsCannotBeEmptyError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeProductsCannotBeEmpty), "products cannot be empty")
}

func NewProviderRequiredError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeProviderRequired), "provider is required for support documents")
}
