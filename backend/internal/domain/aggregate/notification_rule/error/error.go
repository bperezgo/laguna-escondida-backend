package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

const CodeInvalidField baseError.ErrorCode = "NOTIFICATION_RULE_INVALID_FIELD"

// NewInvalidFieldError keeps the offending field path as the error's field value so handlers can name it.
func NewInvalidFieldError(field, reason string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(CodeInvalidField, field+": "+reason, field)
}
