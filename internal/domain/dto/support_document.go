package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type Provider struct {
	DocumentNumber string       `json:"id"`
	DocumentType   DocumentType `json:"document_type"`
	Name           string       `json:"name"`
	Email          string       `json:"email"`
}

type SupportDocumentItem struct {
	Quantity    int             `json:"quantity"`
	Description string          `json:"description"`
	Price       decimal.Decimal `json:"price"`
}

type SupportDocument struct {
	PaymentCode ElectronicInvoicePaymentCode `json:"payment_code"`
	Provider    Provider                     `json:"provider"`
	Items       []SupportDocumentItem        `json:"items"`
}

type SupportDocumentBill struct {
	ID             string          `json:"id"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	TaxAmount      decimal.Decimal `json:"tax_amount"`
	PayAmount      decimal.Decimal `json:"pay_amount"`
	VAT            decimal.Decimal `json:"vat"`
	ICO            decimal.Decimal `json:"ico"`
	Tip            decimal.Decimal `json:"tip"`
	DocumentURL    *string         `json:"document_url,omitempty"`
	PDFStoragePath *string         `json:"pdf_storage_path,omitempty"`
	XMLStoragePath *string         `json:"xml_storage_path,omitempty"`
	Provider       Provider        `json:"provider"`
	Products       []BillProduct   `json:"products,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type CreateSupportDocumentRequest struct {
	Prefix      string
	Consecutive int
	PaymentCode ElectronicInvoicePaymentCode
	Bill        *SupportDocumentBill
}

type SupportDocumentListItem struct {
	ID                     string          `json:"id"`
	TotalAmount            decimal.Decimal `json:"total_amount"`
	DiscountAmount         decimal.Decimal `json:"discount_amount"`
	VAT                    decimal.Decimal `json:"vat"`
	ICO                    decimal.Decimal `json:"ico"`
	Tip                    decimal.Decimal `json:"tip"`
	DocumentURL            *string         `json:"document_url,omitempty"`
	CUFE                   string          `json:"cufe"`
	Tascode                string          `json:"tascode"`
	ProviderDocumentNumber string          `json:"provider_document_number"`
	ProviderName           string          `json:"provider_name"`
	PDFStoragePath         *string         `json:"-"`
	XMLStoragePath         *string         `json:"-"`
	PDFDownloadURL         *string         `json:"pdf_download_url,omitempty"`
	XMLDownloadURL         *string         `json:"xml_download_url,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
}

type ListSupportDocumentsRequest struct {
	Page                   int        `json:"page" validate:"omitempty,min=1"`
	PageSize               int        `json:"page_size" validate:"omitempty,min=1,max=100"`
	CreatedAtStart         *time.Time `json:"created_at_start" validate:"omitempty"`
	CreatedAtEnd           *time.Time `json:"created_at_end" validate:"omitempty"`
	ProviderDocumentNumber *string    `json:"provider_document_number" validate:"omitempty"`
}

type ListSupportDocumentsResponse struct {
	SupportDocuments []SupportDocumentListItem `json:"support_documents"`
	TotalCount       int64                     `json:"total_count"`
	Page             int                       `json:"page"`
	PageSize         int                       `json:"page_size"`
	TotalPages       int                       `json:"total_pages"`
}

type ExportSupportDocumentsRequest struct {
	CreatedAtStart         *time.Time `json:"created_at_start" validate:"omitempty"`
	CreatedAtEnd           *time.Time `json:"created_at_end" validate:"omitempty"`
	ProviderDocumentNumber *string    `json:"provider_document_number" validate:"omitempty"`
}

type SupportDocumentWithTascode struct {
	ID      string
	Tascode string
}
