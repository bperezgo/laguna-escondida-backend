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
	Note2       string          `json:"note2"`
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

func (c *ElectronicInvoiceClient) Create(ctx context.Context, bill *dto.Bill) error {
	now := time.Now()
	issueDate := now.Format("20060102")
	issueTime := now.Format("150405")

	totalAmount := bill.TotalPrice
	discountAmount := 0.0
	taxAmount := bill.VAT
	payAmount := totalAmount + taxAmount + bill.ICO + bill.Tip

	var items []invoiceItem
	for _, product := range bill.Products {
		quantity := 1.0
		unitPrice := product.Price
		itemTotal := unitPrice * quantity

		item := invoiceItem{
			Quantity:    formatFloat(quantity),
			UnitPrice:   formatFloat(unitPrice),
			Total:       formatFloat(itemTotal),
			Description: product.Name,
			Brand:       "LF",
			Model:       product.Category,
			Code:        product.ID[:8],
		}

		if product.VAT > 0 {
			vatPercent := (product.VAT / product.Price) * 100
			vatAmount := itemTotal * (vatPercent / 100)
			item.Taxes = []invoiceTax{
				{
					ID:        "01",
					TaxAmount: formatFloat(vatAmount),
					Percent:   formatFloat(vatPercent),
				},
			}
		}

		items = append(items, item)
	}

	requestData := invoiceRequest{
		Invoice: invoiceRequestData{
			Prefix:      "SETP",
			IntID:       "1",
			IssueDate:   issueDate,
			IssueTime:   issueTime,
			PaymentType: "2",
			PaymentCode: "1",
			Note1:       utils.NumberToWords(payAmount),
			Note2:       "",
			Customer: invoiceCustomer{
				AdditionalAccountID: "1",
				Name:                "Cliente General",
				City:                "Bogotá D.C.",
				CountrySubentity:    "11001",
				AddressLine:         "Calle Principal",
				DocumentNumber:      "900900651",
				DocumentType:        "31",
				Telephone:           "3112196952",
				Email:               "cliente@ejemplo.com",
			},
			Amounts: invoiceAmounts{
				TotalAmount:    formatFloat(totalAmount),
				DiscountAmount: formatFloat(discountAmount),
				TaxAmount:      formatFloat(taxAmount),
				PayAmount:      formatFloat(payAmount),
			},
			Items: items,
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal invoice request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/facturacion.v30/invoice/", c.url), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.user + ":" + c.password))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.client.Do(req)
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

func (c *ElectronicInvoiceClient) Get(ctx context.Context, billID string) (*dto.Bill, error) {
	requestData := verifyStatusRequest{
		VerifyStatus: verifyStatusData{
			Tascode: billID,
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

	document := verifyResp.InvoiceResult.Document
	bill := &dto.Bill{
		DocumentURL: document.PDF,
	}

	return bill, nil
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
