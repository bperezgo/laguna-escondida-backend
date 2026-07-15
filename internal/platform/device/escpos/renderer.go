// Package escpos renders a domain dto.Ticket into ESC/POS bytes for a thermal
// printer. All ESC/POS control sequences are intentionally contained in this
// file so the encoding can be swapped wholesale (e.g. for a maintained library
// when QR/logo support is needed) without touching the domain or service layers.
package escpos

import (
	"bytes"
	"fmt"
	"strings"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// ESC/POS control bytes.
const (
	escByte byte = 0x1B
	gsByte  byte = 0x1D
	fsByte  byte = 0x1C
	lfByte  byte = 0x0A
)

// intlUSA selects the USA international character set (ESC R 0). Many thermal
// printers power up in the China set, which remaps byte 0x24 from '$' to the
// yuan sign '¥'; selecting USA keeps 0x24 as '$' for Colombian pesos / dollars.
const intlUSA byte = 0x00

// Codepage selects the device character table and the matching transcoding used
// for accented characters (á, é, í, ó, ú, ñ, ¡, ¿).
type Codepage string

const (
	CodepageCP850  Codepage = "CP850"
	CodepageCP858  Codepage = "CP858"
	CodepageCP1252 Codepage = "CP1252"
)

// CutMode controls the paper cut emitted at the end of the ticket.
type CutMode string

const (
	CutFull    CutMode = "full"
	CutPartial CutMode = "partial"
	CutNone    CutMode = "none"
)

// Options is the adapter-level configuration for a render. The transport/config
// layer (PRINTER_WIDTH_MM, PRINTER_CODEPAGE, PRINTER_CUT) maps onto these.
type Options struct {
	Width    int      // characters per line (48 ≈ 80mm Font A, 32 ≈ 58mm)
	Codepage Codepage // device codepage + transcoding target
	Cut      CutMode
}

func (o Options) width() int {
	if o.Width > 0 {
		return o.Width
	}
	return 48
}

// charmapFor maps a Codepage to the x/text charmap used to transcode text and
// the ESC t selector byte sent to the device. The selector numbers are Epson
// defaults and must be confirmed on the real device (Phase B).
func charmapFor(cp Codepage) (*charmap.Charmap, byte) {
	switch cp {
	case CodepageCP858:
		return charmap.CodePage858, 19
	case CodepageCP1252:
		return charmap.Windows1252, 16
	case CodepageCP850:
		fallthrough
	default:
		return charmap.CodePage850, 2
	}
}

// Render encodes a ticket into ESC/POS bytes.
func Render(ticket *dto.Ticket, opts Options) ([]byte, error) {
	if ticket == nil {
		return nil, fmt.Errorf("escpos: nil ticket")
	}

	cm, cpSelector := charmapFor(opts.Codepage)
	w := &writer{cm: cm, width: opts.width()}

	w.raw(escByte, '@')             // initialize
	w.raw(fsByte, '.')              // cancel Kanji mode: render high bytes as single-byte CP850 (accents, ñ)
	w.raw(escByte, 'R', intlUSA)    // select international character set (USA): keep 0x24 as '$'
	w.raw(escByte, 't', cpSelector) // select character table

	// Header — centered, business name doubled.
	w.raw(escByte, 'a', 1)
	w.raw(gsByte, '!', 0x11)
	w.line(ticket.BusinessName)
	w.raw(gsByte, '!', 0x00)
	if ticket.BusinessNIT != "" {
		w.line("NIT " + ticket.BusinessNIT)
	}
	if ticket.BusinessAddress != "" {
		w.wrapped(ticket.BusinessAddress)
	}
	w.raw(escByte, 'a', 0)

	// Bill metadata.
	//
	// ticket.Descriptor is deliberately NOT printed here. It is an internal field
	// used to recognize the customer and must never reach the physical ticket.
	// This omission is enforced by TestRender_DescriptorNotPrinted; reintroducing
	// a "Detalle:" line (or otherwise emitting the descriptor) will fail that test
	// on purpose, so the privacy implication is reviewed before it can ship.
	w.rule()
	w.line("Cuenta: " + ticket.TemporalIdentifier)
	if ticket.ServerName != "" {
		w.line("Atendio: " + ticket.ServerName)
	}
	w.line("Fecha: " + ticket.IssuedAt.Format("2006-01-02 15:04"))

	// Items.
	w.rule()
	for _, item := range ticket.Items {
		w.wrapped(item.Name)
		qty := fmt.Sprintf("%d x %s", item.Quantity, formatCOP(item.UnitPrice))
		w.line(padLR(qty, formatCOP(item.LineTotal), w.width))
		if item.Notes != "" {
			w.wrapped("* " + item.Notes)
		}
		w.raw(lfByte)
	}

	// Totals.
	w.rule()
	w.line(padLR("Subtotal", formatCOP(ticket.Subtotal), w.width))
	if ticket.VAT.IsPositive() {
		w.line(padLR("IVA", formatCOP(ticket.VAT), w.width))
	}
	if ticket.ICO.IsPositive() {
		w.line(padLR("ICO", formatCOP(ticket.ICO), w.width))
	}
	if ticket.Tip.IsPositive() {
		w.line(padLR("Propina", formatCOP(ticket.Tip), w.width))
	}
	w.raw(lfByte)
	w.raw(escByte, 'E', 1)
	w.line(padLR("TOTAL", formatCOP(ticket.Total), w.width))
	w.raw(escByte, 'E', 0)

	// Footer — centered.
	if ticket.Footer != "" || ticket.LegalNotice != "" {
		w.rule()
		w.raw(escByte, 'a', 1)
		if ticket.Footer != "" {
			w.wrapped(ticket.Footer)
		}
		if ticket.LegalNotice != "" {
			w.wrapped(ticket.LegalNotice)
		}
		w.raw(escByte, 'a', 0)
	}

	// Feed and cut.
	w.raw(escByte, 'd', 3)
	switch opts.Cut {
	case CutNone:
	case CutFull:
		w.raw(gsByte, 'V', 0)
	case CutPartial:
		fallthrough
	default:
		w.raw(gsByte, 'V', 1)
	}

	if w.err != nil {
		return nil, w.err
	}
	return w.buf.Bytes(), nil
}

// writer accumulates ESC/POS bytes, transcoding only text (never control bytes)
// to the selected codepage.
type writer struct {
	buf   bytes.Buffer
	cm    *charmap.Charmap
	width int
	err   error
}

func (w *writer) raw(b ...byte) {
	if w.err != nil {
		return
	}
	w.buf.Write(b)
}

func (w *writer) text(s string) {
	if w.err != nil {
		return
	}
	enc := encoding.ReplaceUnsupported(w.cm.NewEncoder())
	out, err := enc.Bytes([]byte(s))
	if err != nil {
		w.err = err
		return
	}
	w.buf.Write(out)
}

func (w *writer) line(s string) {
	w.text(s)
	w.raw(lfByte)
}

func (w *writer) wrapped(s string) {
	for _, l := range wrapText(s, w.width) {
		w.line(l)
	}
}

func (w *writer) rule() {
	w.line(strings.Repeat("-", w.width))
}

// padLR places left and right on one line of the given width, left-justified and
// right-justified respectively, truncating the left side if the two would
// overlap. Lengths are measured in runes so accented characters align correctly.
func padLR(left, right string, width int) string {
	l := []rune(left)
	r := []rune(right)
	if len(l)+len(r)+1 > width {
		maxLeft := width - len(r) - 1
		if maxLeft < 0 {
			maxLeft = 0
		}
		if len(l) > maxLeft {
			l = l[:maxLeft]
		}
	}
	gap := width - len(l) - len(r)
	if gap < 1 {
		gap = 1
	}
	return string(l) + strings.Repeat(" ", gap) + string(r)
}

// wrapText word-wraps to width, breaking any single word longer than the width.
func wrapText(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var lines []string
	var cur string
	flush := func() {
		for len([]rune(cur)) > width {
			runes := []rune(cur)
			lines = append(lines, string(runes[:width]))
			cur = string(runes[width:])
		}
	}
	for _, word := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = word
		case len([]rune(cur))+1+len([]rune(word)) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
		flush()
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// formatCOP renders an amount as Colombian pesos: no decimals, '.' as the
// thousands separator, prefixed with '$' (e.g. 51000 -> "$51.000").
func formatCOP(d decimal.Decimal) string {
	n := d.Round(0)
	neg := n.IsNegative()
	digits := n.Abs().String()

	var grouped []byte
	count := 0
	for i := len(digits) - 1; i >= 0; i-- {
		grouped = append(grouped, digits[i])
		count++
		if count%3 == 0 && i != 0 {
			grouped = append(grouped, '.')
		}
	}
	for i, j := 0, len(grouped)-1; i < j; i, j = i+1, j-1 {
		grouped[i], grouped[j] = grouped[j], grouped[i]
	}

	out := "$" + string(grouped)
	if neg {
		out = "-" + out
	}
	return out
}
