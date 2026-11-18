package error

import "errors"

var (
	ErrOpenBillNotFound  = errors.New("open bill not found")
	ErrTicketPrintFailed = errors.New("failed to print ticket")
)
