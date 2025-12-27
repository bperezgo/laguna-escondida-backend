package dto

import "time"

type CommandItemSSE struct {
	OpenBillProductID  string    `json:"open_bill_product_id"`
	OpenBillID         string    `json:"open_bill_id"`
	ProductName        string    `json:"product_name"`
	Notes              *string   `json:"notes,omitempty"`
	TemporalIdentifier string    `json:"temporal_identifier"`
	Priority           int       `json:"priority"`
	CreatedAt          time.Time `json:"created_at"`
	Name               string    `json:"name"`
}
