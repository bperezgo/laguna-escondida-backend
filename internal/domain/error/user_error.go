package error

import "errors"

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserAlreadyExists     = errors.New("user already exists")
	ErrUserCreationFailed    = errors.New("failed to create user")
	ErrRoleNotFound          = errors.New("role not found")
	ErrInvalidRoleIDs        = errors.New("invalid role IDs provided")
	ErrInvalidCredentials    = errors.New("invalid username or password")
	ErrUserInactive          = errors.New("user is inactive")
	ErrCannotDeleteSelf      = errors.New("cannot delete your own user")
	ErrCannotDeactivateSelf  = errors.New("cannot deactivate your own user")
	ErrCannotRemoveLastAdmin = errors.New("cannot remove the last active admin")
)
