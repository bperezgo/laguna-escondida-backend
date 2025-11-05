package dto

import "time"

type Product struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Version   string    `json:"version"`
	Price     float64   `json:"price"`
	VAT       float64   `json:"vat"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
