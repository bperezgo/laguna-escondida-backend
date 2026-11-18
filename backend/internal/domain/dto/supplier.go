package dto

import "time"

type Supplier struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	IdentificationType   *string   `json:"identification_type,omitempty"`
	IdentificationNumber *string   `json:"identification_number,omitempty"`
	ContactName          *string   `json:"contact_name,omitempty"`
	Phone                *string   `json:"phone,omitempty"`
	Email                *string   `json:"email,omitempty"`
	Notes                *string   `json:"notes,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type CreateSupplierRequest struct {
	Name                 string  `json:"name" validate:"required,min=1,max=255"`
	IdentificationType   *string `json:"identification_type,omitempty" validate:"omitempty,max=50"`
	IdentificationNumber *string `json:"identification_number,omitempty" validate:"omitempty,max=50"`
	ContactName          *string `json:"contact_name,omitempty" validate:"omitempty,max=255"`
	Phone                *string `json:"phone,omitempty" validate:"omitempty,max=50"`
	Email                *string `json:"email,omitempty" validate:"omitempty,email,max=255"`
	Notes                *string `json:"notes,omitempty" validate:"omitempty,max=1000"`
}

type UpdateSupplierRequest struct {
	Name                 string  `json:"name" validate:"required,min=1,max=255"`
	IdentificationType   *string `json:"identification_type,omitempty" validate:"omitempty,max=50"`
	IdentificationNumber *string `json:"identification_number,omitempty" validate:"omitempty,max=50"`
	ContactName          *string `json:"contact_name,omitempty" validate:"omitempty,max=255"`
	Phone                *string `json:"phone,omitempty" validate:"omitempty,max=50"`
	Email                *string `json:"email,omitempty" validate:"omitempty,email,max=255"`
	Notes                *string `json:"notes,omitempty" validate:"omitempty,max=1000"`
}

type SupplierListResponse struct {
	Suppliers []*Supplier `json:"suppliers"`
	Total     *int        `json:"total,omitempty"`
}
