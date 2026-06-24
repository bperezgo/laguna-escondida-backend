package dto

// PrintTicketRequest is the body of POST /api/device/print. The edge node loads
// the authoritative open bill by ID and renders the ticket; the client never
// sends layout or computed totals.
type PrintTicketRequest struct {
	OpenBillID string `json:"open_bill_id" validate:"required,uuid"`
	Copies     int    `json:"copies" validate:"omitempty,min=1,max=10"`
}
