package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/shared/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/samber/lo"
)

type ElectronicInvoiceClient struct {
	client   *http.Client
	url      string
	user     string
	password string
}

func NewElectronicInvoiceClient(cfg *config.Config) *ElectronicInvoiceClient {
	return &ElectronicInvoiceClient{
		client:   &http.Client{},
		url:      cfg.ElectronicInvoiceURL,
		user:     cfg.ElectronicInvoiceUser,
		password: cfg.ElectronicInvoicePassword,
	}
}

func mapTaxCodeToID(taxCode dto.TaxCode) string {
	switch string(taxCode) {
	case "IVA":
		return "01"
	case "ICO":
		return "04"
	default:
		return string(taxCode)
	}
}

type invoiceRequest struct {
	Invoice invoiceRequestData `json:"invoice"`
}

type invoiceRequestData struct {
	Prefix      string          `json:"prefix"`
	IntID       string          `json:"intID"`
	IssueDate   string          `json:"issueDate"`
	IssueTime   string          `json:"issueTime"`
	PaymentType string          `json:"paymentType"`
	PaymentCode string          `json:"paymentCode"`
	Note1       string          `json:"note1"`
	Customer    invoiceCustomer `json:"customer"`
	Amounts     invoiceAmounts  `json:"amounts"`
	Items       []invoiceItem   `json:"items"`
}

type invoiceCustomer struct {
	AdditionalAccountID string `json:"additionalAccountID"`
	Name                string `json:"name"`
	City                string `json:"city"`
	CountrySubentity    string `json:"countrySubentity"`
	AddressLine         string `json:"addressLine"`
	DocumentNumber      string `json:"documentNumber"`
	DocumentType        string `json:"documentType"`
	Telephone           string `json:"telephone"`
	Email               string `json:"email"`
}

type invoiceAmounts struct {
	TotalAmount    string `json:"totalAmount"`
	DiscountAmount string `json:"discountAmount"`
	TaxAmount      string `json:"taxAmount"`
	PayAmount      string `json:"payAmount"`
}

type invoiceItem struct {
	Quantity    string             `json:"quantity"`
	UnitPrice   string             `json:"unitPrice"`
	Total       string             `json:"total"`
	Description string             `json:"description"`
	Brand       string             `json:"brand"`
	Model       string             `json:"model"`
	Code        string             `json:"code"`
	Allowance   []invoiceAllowance `json:"allowance,omitempty"`
	Taxes       []invoiceTax       `json:"taxes,omitempty"`
}

type invoiceAllowance struct {
	Charge      string `json:"charge"`
	ReasonCode  string `json:"reasonCode"`
	Description string `json:"description"`
	BaseAmount  string `json:"baseAmount"`
	Amount      string `json:"amount"`
}

type invoiceTax struct {
	ID        string `json:"ID"`
	TaxAmount string `json:"taxAmount"`
	Percent   string `json:"percent"`
}

type invoiceResponse struct {
	InvoiceResult invoiceResult `json:"invoiceResult"`
}

type invoiceResult struct {
	Status    invoiceStatus    `json:"status"`
	Documento invoiceDocumento `json:"documento"`
	Prefix    invoicePrefix    `json:"prefix"`
}

type invoiceStatus struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

type invoiceDocumento struct {
	Type     string `json:"type"`
	Mode     string `json:"mode"`
	Tascode  string `json:"tascode"`
	IntID    string `json:"intID"`
	Document string `json:"document"`
	Process  string `json:"process"`
	Retries  string `json:"retries"`
	Customer string `json:"customer"`
	CUFE     string `json:"CUFE"`
}

type verifyStatusRequest struct {
	VerifyStatus verifyStatusData `json:"verifyStatus"`
}

type verifyStatusData struct {
	Tascode string `json:"tascode"`
}

type verifyStatusResponse struct {
	InvoiceResult verifyStatusResult `json:"invoiceResult"`
}

type verifyStatusResult struct {
	Status   invoiceStatus        `json:"status"`
	Document verifyStatusDocument `json:"document"`
}

type verifyStatusDocument struct {
	Type         string   `json:"type"`
	Mode         string   `json:"mode"`
	Tascode      string   `json:"tascode"`
	IntID        string   `json:"intID"`
	Document     string   `json:"document"`
	Process      int      `json:"process"`
	Retries      int      `json:"retries"`
	Customer     string   `json:"customer"`
	EnhancedInfo []string `json:"enhancedInfo"`
	CUFE         string   `json:"CUFE"`
	URL          string   `json:"URL"`
	PDF          string   `json:"PDF"`
	ATTACHED     string   `json:"ATTACHED"`
}

type invoicePrefix struct {
	Prefix      string `json:"prefix"`
	From        string `json:"from"`
	To          string `json:"to"`
	Last        string `json:"last"`
	Remaining   string `json:"remaining"`
	FirstDate   string `json:"firstDate"`
	LastDate    string `json:"lastDate"`
	Description string `json:"description"`
	DIANKey     string `json:"DIANKey"`
	Auth        string `json:"auth"`
}

func (c *ElectronicInvoiceClient) Create(
	ctx context.Context,
	createReq *dto.CreateElectronicInvoiceRequest,
) error {
	now := time.Now()
	issueDate := now.Format("20060102")
	issueTime := now.Format("150405")

	totalAmount := strconv.FormatFloat(createReq.Bill.TotalAmount, 'f', -1, 64)
	discountAmount := strconv.FormatFloat(createReq.Bill.DiscountAmount, 'f', -1, 64)
	taxAmount := strconv.FormatFloat(createReq.Bill.TaxAmount, 'f', -1, 64)
	payAmount := strconv.FormatFloat(createReq.Bill.PayAmount, 'f', -1, 64)

	productMap := make(map[string]*dto.Product)
	for _, product := range createReq.Products {
		productMap[product.ID] = product
	}

	customer := createReq.Bill.Customer
	if customer == nil {
		return fmt.Errorf("customer is required")
	}

	requestData := invoiceRequest{
		Invoice: invoiceRequestData{
			Prefix:      createReq.Prefix,
			IntID:       strconv.Itoa(createReq.Consecutive),
			IssueDate:   issueDate,
			IssueTime:   issueTime,
			PaymentType: "1", // Contado->1 / Credito->2 // We are not using loans to pay anything in our system
			PaymentCode: paymentCodeToCode(createReq.PaymentCode),
			Note1:       utils.NumberToWords(payAmount),
			Customer: invoiceCustomer{
				// TODO: map additional account id to code
				AdditionalAccountID: "1",
				Name:                customer.Name,
				City:                "No reporta",
				CountrySubentity:    "No reporta",
				AddressLine:         "No reporta",
				DocumentNumber:      customer.DocumentNumber,
				// TODO: map document type to code
				DocumentType: string(customer.DocumentType),
				Telephone:    "No reporta",
				Email:        customer.Email,
			},
			Amounts: invoiceAmounts{
				TotalAmount:    totalAmount,
				DiscountAmount: discountAmount,
				TaxAmount:      taxAmount,
				PayAmount:      payAmount,
			},
			Items: lo.Map(createReq.Bill.Products, func(billProduct dto.BillProductForInvoice, _ int) invoiceItem {
				product := productMap[billProduct.ProductID]
				total := billProduct.UnitPrice * float64(billProduct.Quantity)

				description := ""
				if product != nil && product.Description != nil {
					description = *product.Description
				}

				brand := ""
				if product != nil && product.Brand != nil {
					brand = *product.Brand
				}

				model := ""
				if product != nil && product.Model != nil {
					model = *product.Model
				}

				code := ""
				if product != nil {
					code = product.SKU
				}

				return invoiceItem{
					Quantity:    strconv.Itoa(billProduct.Quantity),
					UnitPrice:   strconv.FormatFloat(billProduct.UnitPrice, 'f', -1, 64),
					Total:       strconv.FormatFloat(total, 'f', -1, 64),
					Description: description,
					Brand:       brand,
					Model:       model,
					Code:        code,
					Allowance: lo.Map(billProduct.Allowance, func(allowance dto.InvoiceAllowance, index int) invoiceAllowance {
						return invoiceAllowance{
							Charge:      allowance.Charge,
							ReasonCode:  allowance.ReasonCode,
							Description: allowance.Description,
							BaseAmount:  allowance.BaseAmount,
							Amount:      allowance.Amount,
						}
					}),
					Taxes: lo.Map(billProduct.Taxes, func(tax dto.InvoiceTax, index int) invoiceTax {
						return invoiceTax{
							ID:        mapTaxCodeToID(tax.TaxCode),
							TaxAmount: tax.TaxAmount,
							Percent:   tax.Percent,
						}
					}),
				}
			}),
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal invoice request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/facturacion.v30/invoice/", c.url), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.user + ":" + c.password))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invoice API returned status %d: %s", resp.StatusCode, string(body))
	}

	var invoiceResp invoiceResponse
	if err := json.Unmarshal(body, &invoiceResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if invoiceResp.InvoiceResult.Status.Code != 200 {
		return fmt.Errorf("invoice API error: %s", invoiceResp.InvoiceResult.Status.Text)
	}

	return nil
}

func (c *ElectronicInvoiceClient) Get(ctx context.Context, invoiceID string) (*dto.ElectronicInvoice, error) {
	requestData := verifyStatusRequest{
		VerifyStatus: verifyStatusData{
			Tascode: invoiceID,
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal verify status request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/facturacion.v30/invoice/", c.url), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.user + ":" + c.password))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invoice API returned status %d: %s", resp.StatusCode, string(body))
	}

	var verifyResp verifyStatusResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if verifyResp.InvoiceResult.Status.Code != 200 {
		return nil, fmt.Errorf("invoice API error: %s", verifyResp.InvoiceResult.Status.Text)
	}

	return &dto.ElectronicInvoice{}, nil
}
