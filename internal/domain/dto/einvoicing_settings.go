package dto

// EInvoicingSettings is one organization's electronic-invoicing configuration.
type EInvoicingSettings struct {
	OrganizationID        string
	InvoicePrefix         string
	SupportDocumentPrefix string
}
