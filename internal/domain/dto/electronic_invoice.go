package dto

type ElectronicInvoicePaymentCode string

const (
	ElectronicInvoicePaymentCodeCreditCard             ElectronicInvoicePaymentCode = "credit_card"
	ElectronicInvoicePaymentCodeDebitCard              ElectronicInvoicePaymentCode = "debit_card"
	ElectronicInvoicePaymentCodeCash                   ElectronicInvoicePaymentCode = "cash"
	ElectronicInvoicePaymentCodeTransferDebitBank      ElectronicInvoicePaymentCode = "transfer_debit_bank"
	ElectronicInvoicePaymentCodeTransferCreditBank     ElectronicInvoicePaymentCode = "transfer_credit_bank"
	ElectronicInvoicePaymentCodeTransferDebitInterbank ElectronicInvoicePaymentCode = "transfer_debit_interbank"
)

type DocumentType string

const (
	DocumentTypeNationalIdentificationNumber DocumentType = "CC"
	DocumentTypeNIT                          DocumentType = "NIT"
)

type Customer struct {
	DocumentNumber string       `json:"id"`
	DocumentType   DocumentType `json:"document_type"`
	Name           string       `json:"name"`
	Email          string       `json:"email"`
}

type InvoiceAmounts struct {
	TotalAmount    string `json:"totalAmount"`
	DiscountAmount string `json:"discountAmount"`
	TaxAmount      string `json:"taxAmount"`
	PayAmount      string `json:"payAmount"`
}

type InvoiceAllowance struct {
	Charge      string `json:"charge"`
	ReasonCode  string `json:"reasonCode"`
	Description string `json:"description"`
	BaseAmount  string `json:"baseAmount"`
	Amount      string `json:"amount"`
}

type InvoiceTax struct {
	ID        string `json:"ID"`
	TaxAmount string `json:"taxAmount"`
	Percent   string `json:"percent"`
}

type InvoiceItem struct {
	Quantity    string             `json:"quantity"`
	UnitPrice   string             `json:"unitPrice"`
	Total       string             `json:"total"`
	Description string             `json:"description"`
	Brand       string             `json:"brand"`
	Model       string             `json:"model"`
	Code        string             `json:"code"`
	Allowance   []InvoiceAllowance `json:"allowance,omitempty"`
	Taxes       []InvoiceTax       `json:"taxes,omitempty"`
}

type ElectronicInvoice struct {
	Consecutive int                          `json:"consecutive"`
	IssueDate   string                       `json:"issue_date"`
	IssueTime   string                       `json:"issue_time"`
	PaymentCode ElectronicInvoicePaymentCode `json:"payment_code"`
	Customer    Customer                     `json:"customer"`
	Amounts     InvoiceAmounts               `json:"amounts"`
	Items       []InvoiceItem                `json:"items"`
}
