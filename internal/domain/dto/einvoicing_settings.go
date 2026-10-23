package dto

// EInvoicingSettings is one organization's electronic-invoicing configuration.
type EInvoicingSettings struct {
	OrganizationID        string `json:"-"`
	InvoicePrefix         string `json:"invoice_prefix"`
	SupportDocumentPrefix string `json:"support_document_prefix"`
}
