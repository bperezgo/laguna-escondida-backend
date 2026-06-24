package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// TicketBusinessInfo holds the fiscal identity printed in the ticket header plus
// the static footer/legal text. It is injected as configuration at wiring time so
// the domain stays independent of where these values come from (env/config/DB).
type TicketBusinessInfo struct {
	Name        string
	NIT         string
	Address     string
	Footer      string
	LegalNotice string
}

// TicketItem is a single printed line: a product with its quantity and money columns.
type TicketItem struct {
	Name      string
	Quantity  int
	UnitPrice decimal.Decimal
	LineTotal decimal.Decimal
	Notes     string
}

// Ticket is the structured, transport-agnostic representation of a printable
// receipt (the "cuenta"). The adapter layer renders it to ESC/POS bytes; the
// domain never deals with bytes or layout.
type Ticket struct {
	BusinessName    string
	BusinessNIT     string
	BusinessAddress string

	TemporalIdentifier string
	ServerName         string
	Descriptor         string
	IssuedAt           time.Time

	Items []TicketItem

	Subtotal decimal.Decimal
	VAT      decimal.Decimal
	ICO      decimal.Decimal
	Tip      decimal.Decimal
	Total    decimal.Decimal

	Footer      string
	LegalNotice string
}
