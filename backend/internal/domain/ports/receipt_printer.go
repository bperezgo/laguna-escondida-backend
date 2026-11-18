package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// ReceiptPrinter is the port for sending a structured ticket to a physical
// printer. Implementations live in the platform/device adapter and own the
// ESC/POS encoding and transport; the port deliberately takes a *dto.Ticket,
// never bytes.
type ReceiptPrinter interface {
	Print(ctx context.Context, ticket *dto.Ticket) error
}
