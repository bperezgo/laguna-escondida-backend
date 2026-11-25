package httpclient

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const merchantAnswer = `{"data":{"email":"restaurante@example.com",
	"presigned_acceptance":{"acceptance_token":"acc-1","permalink":"https://wompi.com/terms.pdf","type":"END_USER_POLICY"},
	"presigned_personal_data_auth":{"acceptance_token":"per-1","permalink":"https://wompi.com/data.pdf","type":"PERSONAL_DATA_AUTH"}}}`

func TestWompiClient_MerchantTerms(t *testing.T) {
	fake := &fakeWompi{status: http.StatusOK, answer: merchantAnswer}

	terms, err := newTestWompiClient(t, fake).MerchantTerms(context.Background(), "pub_test_abc")

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, fake.method)
	assert.Equal(t, "/v1/merchants/pub_test_abc", fake.path)
	assert.Empty(t, fake.auth, "the public key is in the path, not a bearer")
	assert.Equal(t, dto.WompiMerchantTerms{
		TermsURL: "https://wompi.com/terms.pdf", PersonalDataURL: "https://wompi.com/data.pdf", Email: "restaurante@example.com",
		AcceptanceToken: "acc-1", PersonalAuthToken: "per-1",
	}, terms)
}

func TestWompiClient_MerchantTerms_Failures(t *testing.T) {
	for name, fake := range map[string]*fakeWompi{
		"refused":                {status: http.StatusUnprocessableEntity, answer: `{"error":{}}`},
		"not JSON":               {status: http.StatusOK, answer: `<html>`},
		"the documents are gone": {status: http.StatusOK, answer: `{"data":{"email":"a@b.co"}}`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newTestWompiClient(t, fake).MerchantTerms(context.Background(), "pub_test_abc")

			require.Error(t, err)
		})
	}
}

func TestWompiClient_CreateQRTransaction(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"12022516-1-1","status":"PENDING"}}`}
	expires := time.Date(2026, 11, 24, 18, 30, 0, 0, time.UTC)

	id, err := newTestWompiClient(t, fake).CreateQRTransaction(context.Background(), "prv_test_k", dto.QRTransactionRequest{
		AcceptanceToken: "acc-1", PersonalAuthToken: "per-1", AmountInCents: 4500000, Currency: "COP", Reference: "ref-1",
		CustomerEmail: "restaurante@example.com", Signature: "sig", ExpiresAt: &expires,
	})

	require.NoError(t, err)
	assert.Equal(t, "12022516-1-1", id)
	assert.Equal(t, http.MethodPost, fake.method)
	assert.Equal(t, "/v1/transactions", fake.path)
	assert.Equal(t, "Bearer prv_test_k", fake.auth)
	assert.JSONEq(t, `"acc-1"`, string(fake.body["acceptance_token"]))
	assert.JSONEq(t, `"per-1"`, string(fake.body["accept_personal_auth"]))
	assert.JSONEq(t, `4500000`, string(fake.body["amount_in_cents"]))
	assert.JSONEq(t, `"ref-1"`, string(fake.body["reference"]))
	assert.JSONEq(t, `"sig"`, string(fake.body["signature"]))
	assert.JSONEq(t, `"2026-11-24T18:30:00.000Z"`, string(fake.body["expiration_time"]))
	assert.Contains(t, string(fake.body["payment_method"]), `"type":"BANCOLOMBIA_QR"`)
	assert.NotContains(t, string(fake.body["payment_method"]), "sandbox_status", "the test server is not Wompi's sandbox")
}

func TestWompiClient_CreateQRTransaction_WithoutExpiry(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"tx-1"}}`}

	_, err := newTestWompiClient(t, fake).CreateQRTransaction(context.Background(), "prv_test_k", dto.QRTransactionRequest{Reference: "ref-1"})

	require.NoError(t, err)
	assert.NotContains(t, fake.body, "expiration_time")
}

func TestWompiClient_CreateQRTransaction_AsksTheSandboxForAnOutcome(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"tx-1"}}`}
	client := newTestWompiClient(t, fake)
	client.baseURL = "https://sandbox.wompi.co/v1"
	assert.Equal(t, "APPROVED", client.sandboxStatus())
}

func TestWompiClient_CreateQRTransaction_Failures(t *testing.T) {
	for name, fake := range map[string]*fakeWompi{
		"refused":  {status: http.StatusUnprocessableEntity, answer: `{"error":{"type":"INPUT_VALIDATION_ERROR"}}`},
		"no id":    {status: http.StatusCreated, answer: `{"data":{}}`},
		"not JSON": {status: http.StatusCreated, answer: `nope`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newTestWompiClient(t, fake).CreateQRTransaction(context.Background(), "prv_test_k", dto.QRTransactionRequest{})

			require.Error(t, err)
		})
	}
}

func TestWompiClient_QRImage(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"/>`
	fake := &fakeWompi{status: http.StatusOK,
		answer: `{"data":{"id":"tx-1","payment_method":{"extra":{"qr_image":"` + base64.StdEncoding.EncodeToString([]byte(svg)) + `"}}}}`}

	got, ready, err := newTestWompiClient(t, fake).QRImage(context.Background(), "prv_test_k", "tx-1")

	require.NoError(t, err)
	assert.True(t, ready)
	assert.Equal(t, svg, got)
	assert.Equal(t, http.MethodGet, fake.method)
	assert.Equal(t, "/v1/transactions/tx-1", fake.path)
}

func TestWompiClient_QRImage_NotYet(t *testing.T) {
	fake := &fakeWompi{status: http.StatusOK, answer: `{"data":{"id":"tx-1","payment_method":{"extra":{}}}}`}

	_, ready, err := newTestWompiClient(t, fake).QRImage(context.Background(), "prv_test_k", "tx-1")

	require.NoError(t, err)
	assert.False(t, ready)
}

func TestWompiClient_QRImage_Failures(t *testing.T) {
	for name, fake := range map[string]*fakeWompi{
		"unknown":    {status: http.StatusNotFound, answer: `{}`},
		"not base64": {status: http.StatusOK, answer: `{"data":{"payment_method":{"extra":{"qr_image":"***"}}}}`},
		"not JSON":   {status: http.StatusOK, answer: `nope`},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := newTestWompiClient(t, fake).QRImage(context.Background(), "prv_test_k", "tx-1")

			require.Error(t, err)
		})
	}
}

func TestWompiClient_QRImage_ARealSizedAnswer(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(`<path d="M0,0h1v1h-1z"/>`, 4800) + `</svg>`
	encoded := base64.StdEncoding.EncodeToString([]byte(svg))
	require.Greater(t, len(encoded), 100<<10, "Wompi's real answer is far past 64 KB")
	fake := &fakeWompi{status: http.StatusOK, answer: `{"data":{"id":"tx-1","payment_method":{"extra":{"qr_image":"` + encoded + `"}}}}`}

	got, ready, err := newTestWompiClient(t, fake).QRImage(context.Background(), "prv_test_k", "tx-1")

	require.NoError(t, err)
	assert.True(t, ready)
	assert.Equal(t, svg, got)
}
