package dto

import "time"

const (
	OrganizationStatusActive    = "active"
	OrganizationStatusSuspended = "suspended"
)

// Organization is one restaurant on the platform. Optional settings are nil when unset.
type Organization struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Slug              string              `json:"slug"`
	Status            string              `json:"status"`
	Timezone          string              `json:"timezone"`
	Country           string              `json:"country"`
	Locale            string              `json:"locale"`
	Currency          string              `json:"currency"`
	LegalName         *string             `json:"legal_name"`
	NIT               *string             `json:"nit"`
	Address           *string             `json:"address"`
	Phone             *string             `json:"phone"`
	TicketFooter      *string             `json:"ticket_footer"`
	TicketLegalNotice *string             `json:"ticket_legal_notice"`
	DisplayName       *string             `json:"display_name"`
	LogoURL           *string             `json:"logo_url"`
	PrimaryColor      *string             `json:"primary_color"`
	SecondaryColor    *string             `json:"secondary_color"`
	EInvoicing        *EInvoicingSettings `json:"einvoicing"`
	// Settings is read-only here: it changes through the tenant's /api/organization/settings.
	Settings  OrganizationSettings `json:"settings"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// CurrentOrganization is what a signed-in user sees of their organization: settings, not status or e-invoicing.
type CurrentOrganization struct {
	ID                string  `json:"id"`
	Slug              string  `json:"slug"`
	Name              string  `json:"name"`
	Timezone          string  `json:"timezone"`
	Country           string  `json:"country"`
	Locale            string  `json:"locale"`
	Currency          string  `json:"currency"`
	LegalName         *string `json:"legal_name"`
	NIT               *string `json:"nit"`
	Address           *string `json:"address"`
	Phone             *string `json:"phone"`
	TicketFooter      *string `json:"ticket_footer"`
	TicketLegalNotice *string `json:"ticket_legal_notice"`
	DisplayName       *string `json:"display_name"`
	LogoURL           *string `json:"logo_url"`
	PrimaryColor      *string `json:"primary_color"`
	SecondaryColor    *string `json:"secondary_color"`
}

// CreateOrganizationRequest leaves a field empty to take its default (required settings) or stay unset (optional ones).
type CreateOrganizationRequest struct {
	Name              string                `json:"name"`
	Slug              string                `json:"slug"`
	Status            string                `json:"status"`
	Timezone          string                `json:"timezone"`
	Country           string                `json:"country"`
	Locale            string                `json:"locale"`
	Currency          string                `json:"currency"`
	LegalName         string                `json:"legal_name"`
	NIT               string                `json:"nit"`
	Address           string                `json:"address"`
	Phone             string                `json:"phone"`
	TicketFooter      string                `json:"ticket_footer"`
	TicketLegalNotice string                `json:"ticket_legal_notice"`
	DisplayName       string                `json:"display_name"`
	LogoURL           string                `json:"logo_url"`
	PrimaryColor      string                `json:"primary_color"`
	SecondaryColor    string                `json:"secondary_color"`
	EInvoicing        *SetEInvoicingRequest `json:"einvoicing"`
}

// UpdateOrganizationRequest: nil leaves a field unchanged; an empty string clears an optional field.
type UpdateOrganizationRequest struct {
	Name              *string `json:"name"`
	Status            *string `json:"status"`
	Timezone          *string `json:"timezone"`
	Country           *string `json:"country"`
	Locale            *string `json:"locale"`
	Currency          *string `json:"currency"`
	LegalName         *string `json:"legal_name"`
	NIT               *string `json:"nit"`
	Address           *string `json:"address"`
	Phone             *string `json:"phone"`
	TicketFooter      *string `json:"ticket_footer"`
	TicketLegalNotice *string `json:"ticket_legal_notice"`
	DisplayName       *string `json:"display_name"`
	LogoURL           *string `json:"logo_url"`
	PrimaryColor      *string `json:"primary_color"`
	SecondaryColor    *string `json:"secondary_color"`
}

type SetEInvoicingRequest struct {
	InvoicePrefix         string `json:"invoice_prefix"`
	SupportDocumentPrefix string `json:"support_document_prefix"`
}
