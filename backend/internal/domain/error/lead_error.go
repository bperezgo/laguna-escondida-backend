package error

import "errors"

var (
	ErrLeadNotFound = errors.New("lead not found")
	// ErrLeadRateLimited: too many leads stored lately, whoever sends them.
	ErrLeadRateLimited = errors.New("too many leads submitted; try again later")
)
