package dto

import "time"

type OpenBillProductSSE struct {
	OpenBillProductID  string    `json:"open_bill_product_id"`
	OpenBillID         string    `json:"open_bill_id"`
	ProductName        string    `json:"product_name"`
	Quantity           int       `json:"quantity"`
	Notes              *string   `json:"notes,omitempty"`
	Area               string    `json:"area"`
	Status             string    `json:"status"`
	TemporalIdentifier string    `json:"temporal_identifier"`
	Priority           int       `json:"priority"`
	CreatedAt          time.Time `json:"created_at"`
	CreatedByName      string    `json:"created_by_name"`
}
