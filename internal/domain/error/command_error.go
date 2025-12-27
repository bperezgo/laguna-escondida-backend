package error

import "errors"

var (
	ErrCommandNotFound     = errors.New("command not found")
	ErrCommandItemNotFound = errors.New("command item not found")
)
