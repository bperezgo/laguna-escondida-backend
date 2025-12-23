package dto

import "time"

type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusCancelled CommandStatus = "cancelled"
)

type CommandItem struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Notes       *string `json:"notes,omitempty"`
}

type Command struct {
	ID                 string           `json:"id"`
	OpenBillID         string           `json:"open_bill_id"`
	TemporalIdentifier string           `json:"temporal_identifier"`
	CreatedBy          *OpenBillCreator `json:"created_by,omitempty"`
	Area               string           `json:"area"`
	Status             CommandStatus    `json:"status"`
	Items              []CommandItem    `json:"items"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}
