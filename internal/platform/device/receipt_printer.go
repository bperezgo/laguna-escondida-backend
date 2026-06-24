package device

import (
	"context"
	"strings"
	"sync"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/device/escpos"
)

// Ensure the adapter satisfies the domain port.
var _ ports.ReceiptPrinter = (*ReceiptPrinter)(nil)

// ReceiptPrinter implements ports.ReceiptPrinter: it renders a ticket to ESC/POS
// bytes and ships them over a Transport. A mutex serializes writes so concurrent
// prints (multiple tablets) never interleave bytes on a single device.
type ReceiptPrinter struct {
	transport Transport
	opts      escpos.Options
	mu        sync.Mutex
}

func NewReceiptPrinter(transport Transport, opts escpos.Options) *ReceiptPrinter {
	return &ReceiptPrinter{transport: transport, opts: opts}
}

// NewReceiptPrinterFromConfig builds the transport and render options from cfg and
// returns a ready ReceiptPrinter. Used by the edge wiring in cmd/main.go.
func NewReceiptPrinterFromConfig(cfg Config) (*ReceiptPrinter, error) {
	transport, err := NewTransport(cfg)
	if err != nil {
		return nil, err
	}
	return NewReceiptPrinter(transport, OptionsFromConfig(cfg)), nil
}

func (p *ReceiptPrinter) Print(ctx context.Context, ticket *dto.Ticket) error {
	data, err := escpos.Render(ticket, p.opts)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.transport.Write(ctx, data)
}

// OptionsFromConfig maps the adapter Config onto escpos render options: paper
// width in mm to characters per line, and codepage/cut string values to their
// typed equivalents. Unknown values fall back to safe defaults (48 chars, CP850,
// partial cut).
func OptionsFromConfig(cfg Config) escpos.Options {
	width := 48
	if cfg.WidthMM == 58 {
		width = 32
	}

	codepage := escpos.CodepageCP850
	switch strings.ToUpper(cfg.Codepage) {
	case "CP858":
		codepage = escpos.CodepageCP858
	case "CP1252", "WPC1252", "WINDOWS1252":
		codepage = escpos.CodepageCP1252
	}

	cut := escpos.CutPartial
	switch strings.ToLower(cfg.Cut) {
	case "full":
		cut = escpos.CutFull
	case "none":
		cut = escpos.CutNone
	}

	return escpos.Options{Width: width, Codepage: codepage, Cut: cut}
}
