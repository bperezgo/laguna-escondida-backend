package error

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserCreationFailed = errors.New("failed to create user")
	ErrRoleNotFound       = errors.New("role not found")
	ErrInvalidRoleIDs     = errors.New("invalid role IDs provided")
)
