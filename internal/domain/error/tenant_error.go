package error

import "errors"

// ErrNoOrganization means tenant data was reached without an organization in the context.
var ErrNoOrganization = errors.New("no organization in context")
