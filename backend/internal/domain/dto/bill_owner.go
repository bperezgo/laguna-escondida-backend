package dto

import "time"

type BillOwner struct {
	ID                 string    `json:"id"`
	Celphone           *string   `json:"celphone"`
	Email              string    `json:"email"`
	Name               string    `json:"name"`
	IdentificationType *string   `json:"identification_type"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateBillOwnerRequest struct {
	ID                 string  `json:"id" validate:"required,min=1"`
	Celphone           *string `json:"celphone"`
	Email              string  `json:"email" validate:"required,email"`
	Name               string  `json:"name" validate:"required,min=1,max=255"`
	IdentificationType *string `json:"identification_type"`
}

type UpdateBillOwnerRequest struct {
	Celphone           *string `json:"celphone"`
	Email              string  `json:"email" validate:"required,email"`
	Name               string  `json:"name" validate:"required,min=1,max=255"`
	IdentificationType *string `json:"identification_type"`
}
