package dto

import "time"

type InvoiceListItem struct {
	ID             string    `json:"id"`
	TotalAmount    float64   `json:"total_amount"`
	DiscountAmount float64   `json:"discount_amount"`
	VAT            float64   `json:"vat"`
	ICO            float64   `json:"ico"`
	Tip            float64   `json:"tip"`
	DocumentURL    *string   `json:"document_url,omitempty"`
	CUFE           string    `json:"cufe"`
	Tascode        string    `json:"tascode"`
	CustomerID     *string   `json:"customer_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ListInvoicesRequest struct {
	Page                   int        `json:"page" validate:"omitempty,min=1"`
	PageSize               int        `json:"page_size" validate:"omitempty,min=1,max=100"`
	CreatedAtStart         *time.Time `json:"created_at_start" validate:"omitempty"`
	CreatedAtEnd           *time.Time `json:"created_at_end" validate:"omitempty"`
	NationalIdentification *string    `json:"national_identification" validate:"omitempty"`
}

type ListInvoicesResponse struct {
	Invoices   []InvoiceListItem `json:"invoices"`
	TotalCount int64             `json:"total_count"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}
