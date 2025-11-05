package dto

import "time"

type Product struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Version   int       `json:"version"`
	Price     float64   `json:"price"`
	VAT       float64   `json:"vat"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateProductRequest struct {
	Name     string  `json:"name" validate:"required,min=1,max=255"`
	Category string  `json:"category" validate:"required,min=1,max=100"`
	Price    float64 `json:"price" validate:"required,gt=0"`
	VAT      float64 `json:"vat" validate:"required,gte=0"`
}

type UpdateProductRequest struct {
	Name     string  `json:"name" validate:"required,min=1,max=255"`
	Category string  `json:"category" validate:"required,min=1,max=100"`
	Price    float64 `json:"price" validate:"required,gt=0"`
	VAT      float64 `json:"vat" validate:"required,gte=0"`
}
