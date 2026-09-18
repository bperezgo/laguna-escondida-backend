package dto

import "time"

// SideDishSSE is one resolved side dish on a kitchen-feed line: the ingredient's name (resolved
// by the SSE handler, not carried on the event) and the quantity actually served on that line.
type SideDishSSE struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type OpenBillProductSSE struct {
	OpenBillProductID  string        `json:"open_bill_product_id"`
	OpenBillID         string        `json:"open_bill_id"`
	ProductName        string        `json:"product_name"`
	Quantity           int           `json:"quantity"`
	Notes              *string       `json:"notes,omitempty"`
	Area               string        `json:"area"`
	Status             string        `json:"status"`
	TemporalIdentifier string        `json:"temporal_identifier"`
	Priority           int           `json:"priority"`
	SideDishes         []SideDishSSE `json:"side_dishes,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	CreatedByName      string        `json:"created_by_name"`
	// CompletedAt is only populated by the completed/ready feed (from updated_at);
	// nil on the live pending feed.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
