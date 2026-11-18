// Package device is the adapter that turns a domain dto.Ticket into bytes (via
// the escpos renderer) and ships them to a physical printer over a configurable
// transport. The transport is the only platform-specific piece: on macOS dev use
// `file` (or `tcp` against an emulator); on the restaurant's Windows box use
// `windows` (RAW print spooler, finished in Phase B).
package device

import (
	"context"
	"fmt"
	"strings"
)

// Transport ships a fully-rendered ESC/POS byte stream to a device. Each Write
// is self-contained (one ticket copy).
type Transport interface {
	Write(ctx context.Context, p []byte) error
}

// Config is the adapter configuration, mapped from PRINTER_* env at wiring time.
type Config struct {
	Transport string // file | tcp | windows | serial
	Target    string // file dir | host:port | windows printer name | COM port
	WidthMM   int    // 80 or 58
	Codepage  string // CP850 | CP858 | CP1252
	Cut       string // full | partial | none
}

// NewTransport builds the transport selected by cfg.Transport.
func NewTransport(cfg Config) (Transport, error) {
	switch strings.ToLower(cfg.Transport) {
	case "file":
		return newFileTransport(cfg.Target)
	case "tcp":
		return newTCPTransport(cfg.Target)
	case "windows":
		return newWindowsTransport(cfg.Target)
	case "serial":
		return nil, fmt.Errorf("device: serial transport not implemented (Phase B); use PRINTER_TRANSPORT=file|tcp|windows")
	case "":
		return nil, fmt.Errorf("device: no transport configured (set PRINTER_TRANSPORT)")
	default:
		return nil, fmt.Errorf("device: unknown transport %q (want file|tcp|windows|serial)", cfg.Transport)
	}
}
