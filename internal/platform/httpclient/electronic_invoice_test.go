package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestClient(server *httptest.Server) *ElectronicInvoiceClient {
	cfg := &config.Config{
		ElectronicInvoiceURL:      server.URL,
		ElectronicInvoiceUser:     "testuser",
		ElectronicInvoicePassword: "testpass",
	}

	httpClient := &Client{
		Client: server.Client(),
	}

	return NewElectronicInvoiceClient(cfg, httpClient)
}

func createTestBill() *dto.Bill {
	description := "Test Description"

	return &dto.Bill{
		ID:             "bill-123",
		TotalAmount:    decimal.NewFromFloat(100.00),
		DiscountAmount: decimal.NewFromFloat(10.00),
		TaxAmount:      decimal.NewFromFloat(19.00),
		PayAmount:      decimal.NewFromFloat(109.00),
		Customer: &dto.Customer{
			DocumentNumber: "123456789",
			DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
			Name:           "Test Customer",
			Email:          "test@example.com",
		},
		Products: []dto.BillProduct{
			{
				ProductID:   "prod-1",
				Name:        "Test Product",
				Quantity:    2,
				UnitPrice:   decimal.NewFromFloat(50.00),
				Description: &description,
				Category:    "Test Category",
				Code:        "SKU-001",
				Taxes: []dto.InvoiceTax{
					{TaxCode: dto.TaxCodeVAT, TaxAmount: "19.00", Percent: "19"},
				},
			},
		},
	}
}

func createSuccessResponse() invoiceResponse {
	return invoiceResponse{
		InvoiceResult: invoiceResult{
			Status: invoiceStatus{
				Code: 200,
				Text: "Success",
			},
			Document: invoiceDocument{
				Tascode: "TAS123456",
				CUFE:    "CUFE123456789",
			},
		},
	}
}

// Create Method Tests

func TestCreate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/facturacion.v30/invoice/", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("Authorization"))

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        createTestBill(),
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "TAS123456", resp.Tascode)
	assert.Equal(t, "CUFE123456789", resp.CUFE)
}

func TestCreate_SuccessWithDefaultCustomer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "222222222222", reqBody.Invoice.Customer.DocumentNumber)
		assert.Equal(t, "consumidor final", reqBody.Invoice.Customer.Name)
		assert.Equal(t, "noenviar@noenviar.com", reqBody.Invoice.Customer.Email)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	bill := createTestBill()
	bill.Customer = nil

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        bill,
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreate_SuccessWithNITCustomer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "31", reqBody.Invoice.Customer.DocumentType)
		assert.Equal(t, "1", reqBody.Invoice.Customer.AdditionalAccountID)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	bill := createTestBill()
	bill.Customer.DocumentType = dto.DocumentTypeNIT

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        bill,
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("Internal Server Error"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        createTestBill(),
	}

	resp, err := client.Create(context.Background(), req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invoice API returned status 500")
}

func TestCreate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := invoiceResponse{
			InvoiceResult: invoiceResult{
				Status: invoiceStatus{
					Code: 400,
					Text: "Invalid invoice data",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        createTestBill(),
	}

	resp, err := client.Create(context.Background(), req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid invoice data")
}

func TestCreate_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte("invalid json"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        createTestBill(),
	}

	resp, err := client.Create(context.Background(), req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestCreate_ProductWithEmptyCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// When category is empty, Brand and Model should be set to "unknown"
		assert.Equal(t, "unknown", reqBody.Invoice.Items[0].Brand)
		assert.Equal(t, "unknown", reqBody.Invoice.Items[0].Model)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	bill := createTestBill()
	bill.Products[0].Category = ""

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        bill,
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreate_ProductCategoryUsedAsBrandAndModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Category should be used for both Brand and Model
		assert.Equal(t, "Test Category", reqBody.Invoice.Items[0].Brand)
		assert.Equal(t, "Test Category", reqBody.Invoice.Items[0].Model)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        createTestBill(),
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreate_ProductWithEmptyName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "unknown", reqBody.Invoice.Items[0].Description)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	bill := createTestBill()
	bill.Products[0].Name = ""

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        bill,
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreate_WithAllowances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		require.Len(t, reqBody.Invoice.Items[0].Allowance, 1)
		assert.Equal(t, "false", reqBody.Invoice.Items[0].Allowance[0].Charge)
		assert.Equal(t, "00", reqBody.Invoice.Items[0].Allowance[0].ReasonCode)
		assert.Equal(t, "Discount", reqBody.Invoice.Items[0].Allowance[0].Description)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	bill := createTestBill()
	bill.Products[0].Allowance = []dto.InvoiceAllowance{
		{
			Charge:      "false",
			ReasonCode:  "00",
			Description: "Discount",
			BaseAmount:  "100.00",
			Amount:      "10.00",
		},
	}

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        bill,
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreate_WithICOTax(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody invoiceRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		require.Len(t, reqBody.Invoice.Items[0].Taxes, 1)
		assert.Equal(t, "04", reqBody.Invoice.Items[0].Taxes[0].ID)

		response := createSuccessResponse()
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	bill := createTestBill()
	bill.Products[0].Taxes = []dto.InvoiceTax{
		{TaxCode: dto.TaxCodeICO, TaxAmount: "5.00", Percent: "5"},
	}

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      "FE",
		Consecutive: 1001,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Bill:        bill,
	}

	resp, err := client.Create(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// Get Method Tests

func TestGet_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/facturacion.v30/invoice/", r.URL.Path)

		var reqBody verifyStatusRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "TAS123456", reqBody.VerifyStatus.Tascode)

		response := verifyStatusResponse{
			InvoiceResult: verifyStatusResult{
				Status: invoiceStatus{
					Code: 200,
					Text: "Success",
				},
				Document: verifyStatusDocument{
					PDF: "https://example.com/invoice.pdf",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	resp, err := client.Get(context.Background(), "TAS123456")

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Success", resp.StatusText)
	assert.Equal(t, "https://example.com/invoice.pdf", resp.PDF)
}

func TestGet_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("Internal Server Error"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	resp, err := client.Get(context.Background(), "TAS123456")

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invoice API returned status 500")
}

func TestGet_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte("invalid json"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := createTestClient(server)

	resp, err := client.Get(context.Background(), "TAS123456")

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

// Helper Function Tests

func TestMapTaxCodeToID(t *testing.T) {
	tests := []struct {
		name     string
		taxCode  dto.TaxCode
		expected string
	}{
		{"VAT returns 01", dto.TaxCodeVAT, "01"},
		{"ICO returns 04", dto.TaxCodeICO, "04"},
		{"Unknown returns itself", dto.TaxCode("UNKNOWN"), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapTaxCodeToID(tt.taxCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapDocumentTypeToCode(t *testing.T) {
	tests := []struct {
		name         string
		documentType dto.DocumentType
		expected     string
	}{
		{"NIT returns 31", dto.DocumentTypeNIT, "31"},
		{"CC returns 13", dto.DocumentTypeNationalIdentificationNumber, "13"},
		{"Unknown returns 13", dto.DocumentType("UNKNOWN"), "13"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapDocumentTypeToCode(tt.documentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapDocumentTypeToAdditionalAccountID(t *testing.T) {
	tests := []struct {
		name         string
		documentType dto.DocumentType
		expected     string
	}{
		{"NIT returns 1", dto.DocumentTypeNIT, "1"},
		{"CC returns 2", dto.DocumentTypeNationalIdentificationNumber, "2"},
		{"Unknown returns 2", dto.DocumentType("UNKNOWN"), "2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapDocumentTypeToAdditionalAccountID(tt.documentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Payment Code Tests

func TestCreate_WithDifferentPaymentCodes(t *testing.T) {
	tests := []struct {
		name        string
		paymentCode dto.ElectronicInvoicePaymentCode
		expected    string
	}{
		{"CreditCard", dto.ElectronicInvoicePaymentCodeCreditCard, "48"},
		{"DebitCard", dto.ElectronicInvoicePaymentCodeDebitCard, "49"},
		{"Cash", dto.ElectronicInvoicePaymentCodeCash, "10"},
		{"TransferCreditBank", dto.ElectronicInvoicePaymentCodeTransferCreditBank, "45"},
		{"TransferDebitInterbank", dto.ElectronicInvoicePaymentCodeTransferDebitInterbank, "46"},
		{"TransferDebitBank", dto.ElectronicInvoicePaymentCodeTransferDebitBank, "47"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var reqBody invoiceRequest
				err := json.NewDecoder(r.Body).Decode(&reqBody)
				require.NoError(t, err)

				assert.Equal(t, tt.expected, reqBody.Invoice.PaymentCode)

				response := createSuccessResponse()
				w.Header().Set("Content-Type", "application/json")
				err = json.NewEncoder(w).Encode(response)
				require.NoError(t, err)
			}))
			defer server.Close()

			client := createTestClient(server)

			req := &dto.CreateElectronicInvoiceRequest{
				Prefix:      "FE",
				Consecutive: 1001,
				PaymentCode: tt.paymentCode,
				Bill:        createTestBill(),
			}

			resp, err := client.Create(context.Background(), req)

			require.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}
