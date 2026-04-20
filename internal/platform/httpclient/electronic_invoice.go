package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/shared/utils"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

const unknown = "unknown"

type ElectronicInvoiceClient struct {
	client   *Client
	url      string
	user     string
	password string
}

func NewElectronicInvoiceClient(cfg *config.Config, client *Client) *ElectronicInvoiceClient {
	return &ElectronicInvoiceClient{
		client:   client,
		url:      cfg.ElectronicInvoiceURL,
		user:     cfg.ElectronicInvoiceUser,
		password: cfg.ElectronicInvoicePassword,
	}
}

func mapTaxCodeToID(taxCode dto.TaxCode) string {
	switch taxCode {
	case dto.TaxCodeVAT:
		return "01"
	case dto.TaxCodeICO:
		return "04"
	default:
		return string(taxCode)
	}
}

func mapDocumentTypeToCode(documentType dto.DocumentType) string {
	switch documentType {
	case dto.DocumentTypeNIT:
		return "31"
	case dto.DocumentTypeNationalIdentificationNumber:
		return "13"
	default:
		return "13"
	}
}

func mapDocumentTypeToAdditionalAccountID(documentType dto.DocumentType) string {
	switch documentType {
	case dto.DocumentTypeNIT:
		return "1"
	case dto.DocumentTypeNationalIdentificationNumber:
		return "2"
	default:
		return "2"
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

type invoiceItemSupplier struct {
	Description string `json:"description"`
	StartDate   string `json:"startDate"`
}

type invoiceItem struct {
	Quantity    string               `json:"quantity"`
	UnitPrice   string               `json:"unitPrice"`
	Total       string               `json:"total"`
	Description string               `json:"description"`
	Brand       string               `json:"brand"`
	Model       string               `json:"model"`
	Code        string               `json:"code"`
	Supplier    *invoiceItemSupplier `json:"supplier,omitempty"`
	Allowance   []invoiceAllowance   `json:"allowance,omitempty"`
	Taxes       []invoiceTax         `json:"taxes,omitempty"`
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
	Status   invoiceStatus   `json:"status"`
	Document invoiceDocument `json:"documento"`
	Prefix   invoicePrefix   `json:"prefix"`
}

type invoiceStatus struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

type invoiceDocument struct {
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

type supportDocumentResponse struct {
	InvoiceResult supportDocumentResult `json:"invoiceResult"`
}

type supportDocumentResult struct {
	Status   invoiceStatus              `json:"status"`
	Document supportDocumentResponseDoc `json:"document"`
	Prefix   invoicePrefix              `json:"prefix"`
}

type supportDocumentResponseDoc struct {
	Type     string `json:"type"`
	Mode     string `json:"mode"`
	Tascode  string `json:"tascode"`
	IntID    string `json:"intID"`
	Document string `json:"document"`
	Process  int    `json:"process"`
	Retries  int    `json:"retries"`
	Customer string `json:"customer"`
	CUDS     string `json:"CUDS"`
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
) (res *dto.CreateElectronicInvoiceResponse, err error) {
	loc, locErr := time.LoadLocation("America/Bogota")
	if locErr != nil {
		loc = time.FixedZone("UTC-5", -5*60*60)
	}
	now := time.Now().In(loc)
	issueDate := now.Format("20060102")
	issueTime := now.Format("150405")

	totalAmount := createReq.Bill.TotalAmount.StringFixed(2)
	discountAmount := createReq.Bill.DiscountAmount.StringFixed(2)
	taxAmount := createReq.Bill.TaxAmount.StringFixed(2)
	payAmount := createReq.Bill.PayAmount.StringFixed(2)

	customer := createReq.Bill.Customer
	if customer == nil {
		customer = &dto.Customer{
			DocumentNumber: "222222222222",
			DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
			Name:           "consumidor final",
			Email:          "noenviar@noenviar.com",
		}
	}

	requestData := invoiceRequest{
		Invoice: invoiceRequestData{
			Prefix:      createReq.Prefix,
			IntID:       strconv.Itoa(createReq.Consecutive),
			IssueDate:   issueDate,
			IssueTime:   issueTime,
			PaymentType: "1", // Contado->1 / Credito->2 // We are not using loans to pay anything in our system so always use "1"
			PaymentCode: paymentCodeToCode(createReq.PaymentCode),
			Note1:       utils.NumberToWords(payAmount),
			Customer: invoiceCustomer{
				AdditionalAccountID: mapDocumentTypeToAdditionalAccountID(customer.DocumentType),
				Name:                customer.Name,
				City:                "No Reporta",
				CountrySubentity:    "11001",
				AddressLine:         "No Reporta",
				DocumentNumber:      customer.DocumentNumber,
				DocumentType:        mapDocumentTypeToCode(customer.DocumentType),
				Telephone:           "00000000",
				Email:               customer.Email,
			},
			Amounts: invoiceAmounts{
				TotalAmount:    totalAmount,
				DiscountAmount: discountAmount,
				TaxAmount:      taxAmount,
				PayAmount:      payAmount,
			},
			Items: lo.Map(createReq.Bill.Products, func(billProduct dto.BillProduct, _ int) invoiceItem {
				total := billProduct.UnitPrice.Mul(decimal.NewFromInt(int64(billProduct.Quantity)))

				name := billProduct.Name

				if name == "" {
					name = unknown
				}

				category := billProduct.Category
				if category == "" {
					category = unknown
				}

				code := billProduct.Code

				return invoiceItem{
					Quantity:    decimal.NewFromInt(int64(billProduct.Quantity)).StringFixed(2),
					UnitPrice:   billProduct.UnitPrice.StringFixed(2),
					Total:       total.StringFixed(2),
					Description: name,
					Brand:       category,
					Model:       category,
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
		return nil, fmt.Errorf("failed to marshal invoice request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/facturacion.v30/invoice/", c.url), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.user + ":" + c.password))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invoice API returned status %d: %s", resp.StatusCode, string(body))
	}

	var invoiceResp invoiceResponse
	if err := json.Unmarshal(body, &invoiceResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if invoiceResp.InvoiceResult.Status.Code != 200 {
		return nil, fmt.Errorf("invoice API error: %s", invoiceResp.InvoiceResult.Status.Text)
	}

	return &dto.CreateElectronicInvoiceResponse{
		Tascode: invoiceResp.InvoiceResult.Document.Tascode,
		CUFE:    invoiceResp.InvoiceResult.Document.CUFE,
	}, nil
}

type supportDocumentRequest struct {
	Invoice supportDocumentRequestData `json:"invoice"`
}

type supportDocumentRequestData struct {
	Prefix      string          `json:"prefix"`
	IntID       string          `json:"intID"`
	IssueDate   string          `json:"issueDate"`
	IssueTime   string          `json:"issueTime"`
	PaymentType string          `json:"paymentType"`
	PaymentCode string          `json:"paymentCode"`
	Note1       string          `json:"note1"`
	Supplier    invoiceSupplier `json:"supplier"`
	Amounts     invoiceAmounts  `json:"amounts"`
	Items       []invoiceItem   `json:"items"`
}

type invoiceSupplier struct {
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

func (c *ElectronicInvoiceClient) CreateSupportDocument(
	ctx context.Context,
	createReq *dto.CreateSupportDocumentRequest,
) (res *dto.CreateSupportDocumentResponse, err error) {
	loc, locErr := time.LoadLocation("America/Bogota")
	if locErr != nil {
		loc = time.FixedZone("UTC-5", -5*60*60)
	}
	now := time.Now().In(loc)
	issueDate := now.Format("20060102")
	issueTime := now.Format("150405")

	totalAmount := createReq.Bill.TotalAmount.StringFixed(2)
	discountAmount := createReq.Bill.DiscountAmount.StringFixed(2)
	taxAmount := createReq.Bill.TaxAmount.StringFixed(2)
	payAmount := createReq.Bill.PayAmount.StringFixed(2)

	provider := createReq.Bill.Provider

	requestData := supportDocumentRequest{
		Invoice: supportDocumentRequestData{
			Prefix:      createReq.Prefix,
			IntID:       strconv.Itoa(createReq.Consecutive),
			IssueDate:   issueDate,
			IssueTime:   issueTime,
			PaymentType: "1",
			PaymentCode: paymentCodeToCode(createReq.PaymentCode),
			Note1:       utils.NumberToWords(payAmount),
			Supplier: invoiceSupplier{
				AdditionalAccountID: mapDocumentTypeToAdditionalAccountID(provider.DocumentType),
				Name:                provider.Name,
				City:                "No Reporta",
				CountrySubentity:    "11001",
				AddressLine:         "No Reporta",
				DocumentNumber:      provider.DocumentNumber,
				DocumentType:        "31", // always 31, no matter
				Telephone:           "00000000",
				Email:               provider.Email,
			},
			Amounts: invoiceAmounts{
				TotalAmount:    totalAmount,
				DiscountAmount: discountAmount,
				TaxAmount:      taxAmount,
				PayAmount:      payAmount,
			},
			Items: lo.Map(createReq.Bill.Products, func(billProduct dto.BillProduct, _ int) invoiceItem {
				total := billProduct.UnitPrice.Mul(decimal.NewFromInt(int64(billProduct.Quantity)))

				name := billProduct.Name
				if name == "" {
					name = unknown
				}

				category := billProduct.Category
				if category == "" {
					category = unknown
				}

				code := billProduct.Code
				if code == "" {
					code = unknown
				}

				return invoiceItem{
					Quantity:    decimal.NewFromInt(int64(billProduct.Quantity)).StringFixed(2),
					UnitPrice:   billProduct.UnitPrice.StringFixed(2),
					Total:       total.StringFixed(2),
					Description: name,
					Brand:       category,
					Model:       category,
					Code:        code,
					Supplier: &invoiceItemSupplier{
						Description: "1",
						StartDate:   issueDate,
					},
				}
			}),
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal support document request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/facturacion.v30/invoice/", c.url), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.user + ":" + c.password))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("support document API returned status %d: %s", resp.StatusCode, string(body))
	}

	var sdResp supportDocumentResponse
	if err := json.Unmarshal(body, &sdResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if sdResp.InvoiceResult.Status.Code != 200 {
		return nil, fmt.Errorf("support document API error: %s", sdResp.InvoiceResult.Status.Text)
	}

	return &dto.CreateSupportDocumentResponse{
		Tascode: sdResp.InvoiceResult.Document.Tascode,
		CUDS:    sdResp.InvoiceResult.Document.CUDS,
	}, nil
}

func (c *ElectronicInvoiceClient) Get(ctx context.Context, invoiceID string) (res *dto.VerifyInvoiceStatusResponse, err error) {
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = closeErr
		}
	}()

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

	return &dto.VerifyInvoiceStatusResponse{
		StatusCode: verifyResp.InvoiceResult.Status.Code,
		StatusText: verifyResp.InvoiceResult.Status.Text,
		PDF:        verifyResp.InvoiceResult.Document.PDF,
		XML:        verifyResp.InvoiceResult.Document.ATTACHED,
	}, nil
}
