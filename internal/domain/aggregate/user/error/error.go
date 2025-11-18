package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

// UserErrorCode defines error codes for user aggregate
type UserErrorCode string

const (
	CodeInvalidRequest        UserErrorCode = "USER_INVALID_REQUEST"
	CodeMissingUsername       UserErrorCode = "USER_MISSING_USERNAME"
	CodeMissingPassword       UserErrorCode = "USER_MISSING_PASSWORD"
	CodeInvalidPassword       UserErrorCode = "USER_INVALID_PASSWORD"
	CodePasswordHashingFailed UserErrorCode = "USER_PASSWORD_HASHING_FAILED"
)

// NewInvalidRequestError creates an error for invalid request
func NewInvalidRequestError(message string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidRequest), message)
}

// NewMissingUsernameError creates an error for missing username
func NewMissingUsernameError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingUsername), "username is required")
}

// NewMissingPasswordError creates an error for missing password
func NewMissingPasswordError() *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeMissingPassword), "password is required")
}

// NewInvalidPasswordError creates an error for invalid password
func NewInvalidPasswordError(message string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidPassword), message)
}

// NewPasswordHashingFailedError creates an error for password hashing failure
func NewPasswordHashingFailedError(err error) *baseError.BaseError {
	return baseError.Wrap(err, baseError.ErrorCode(CodePasswordHashingFailed), "failed to hash password")
}

// Wrap wraps an existing error with a user error
func Wrap(err error, code UserErrorCode, message string) *baseError.BaseError {
	return baseError.Wrap(err, baseError.ErrorCode(code), message)
}

// WrapWithField wraps an existing error with a user error and field value context
func WrapWithField(err error, code UserErrorCode, message string, fieldValue interface{}) *baseError.BaseError {
	return baseError.WrapWithField(err, baseError.ErrorCode(code), message, fieldValue)
}
