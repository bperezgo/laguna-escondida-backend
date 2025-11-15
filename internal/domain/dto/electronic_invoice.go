package dto

import "time"

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

type TaxCode string

const (
	TaxCodeVAT TaxCode = "VAT"
	TaxCodeICO TaxCode = "ICO"
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
	TaxCode   TaxCode `json:"taxCode"`
	TaxAmount string  `json:"taxAmount"`
	Percent   string  `json:"percent"`
}

type InvoiceItem struct {
	Quantity  int                `json:"quantity"`
	ProductID string             `json:"product_id"`
	Allowance []InvoiceAllowance `json:"allowance,omitempty"`
	Taxes     []InvoiceTax       `json:"taxes,omitempty"`
}

type ElectronicInvoice struct {
	Consecutive int                          `json:"consecutive"`
	IssueDate   string                       `json:"issue_date"`
	IssueTime   string                       `json:"issue_time"`
	PaymentCode ElectronicInvoicePaymentCode `json:"payment_code"`
	Customer    *Customer                    `json:"customer"`
	Amounts     InvoiceAmounts               `json:"amounts"`
	Items       []InvoiceItem                `json:"items"`
}

type BillProductForInvoice struct {
	ProductID string
	Quantity  int
	UnitPrice float64
	Allowance []InvoiceAllowance
	Taxes     []InvoiceTax
}

type BillForInvoice struct {
	ID             string
	TotalAmount    float64
	DiscountAmount float64
	TaxAmount      float64
	PayAmount      float64
	VAT            float64
	ICO            float64
	Tip            float64
	DocumentURL    *string
	Customer       *Customer
	Products       []BillProductForInvoice
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateElectronicInvoiceRequest struct {
	Prefix      string
	Consecutive int
	PaymentCode ElectronicInvoicePaymentCode
	Bill        *BillForInvoice
	Products    []*Product
}
