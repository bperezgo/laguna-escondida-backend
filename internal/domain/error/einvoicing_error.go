package error

import "errors"

var (
	ErrEInvoicingNotConfigured    = errors.New("electronic invoicing is not configured for this organization")
	ErrConsecutiveAlreadyAssigned = errors.New("pending invoice already has a consecutive")
)
