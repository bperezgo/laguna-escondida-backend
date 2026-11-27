package httpclient

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chargeRequest() dto.BillingChargeRequest {
	return dto.BillingChargeRequest{PaymentSourceID: "3891", AmountInCents: 4000000, Currency: "COP",
		Reference: "kb-019a0000-20261215-1", CustomerEmail: "rosa@example.com"}
}

func TestWompiBilling_Charge_SendsASignedRequest(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"1234-1610641025-49201","status":"PENDING",
		"reference":"kb-019a0000-20261215-1","amount_in_cents":4000000,"currency":"COP"}}`}
	billing := newTestWompiBilling(t, fake)

	tx, err := billing.Charge(t.Context(), chargeRequest())

	require.NoError(t, err)
	assert.Equal(t, dto.BillingTransaction{ID: "1234-1610641025-49201", Reference: "kb-019a0000-20261215-1",
		Status: dto.BillingChargePending, AmountInCents: 4000000, Currency: "COP"}, tx)
	assert.Equal(t, http.MethodPost, fake.method)
	assert.Equal(t, "/v1/transactions", fake.path)
	assert.Equal(t, "Bearer prv_test_platform", fake.auth)
	assert.JSONEq(t, `3891`, string(fake.body["payment_source_id"]), "the source id goes as the number Wompi issued")
	assert.JSONEq(t, `4000000`, string(fake.body["amount_in_cents"]))
	assert.JSONEq(t, `"COP"`, string(fake.body["currency"]))
	assert.JSONEq(t, `"kb-019a0000-20261215-1"`, string(fake.body["reference"]))
	assert.JSONEq(t, `"rosa@example.com"`, string(fake.body["customer_email"]))
	assert.JSONEq(t, `{"installments":1}`, string(fake.body["payment_method"]))

	want := sha256.Sum256([]byte("kb-019a0000-20261215-1" + "4000000" + "COP" + "test_integrity_platform"))
	var signature string
	require.NoError(t, json.Unmarshal(fake.body["signature"], &signature))
	assert.Equal(t, hex.EncodeToString(want[:]), signature)
	assert.NotContains(t, string(fake.body["signature"]), "test_integrity_platform")
}

// The value Wompi's own documentation gives for its example reference, amount and secret.
func TestWompiBilling_Charge_SignatureMatchesWompisDocumentedExample(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"tx_1","status":"PENDING"}}`}
	srv := fake.server(t)
	billing := NewWompiBilling(NewClient(slog.New(slog.DiscardHandler)), srv.URL+"/v1", "pub_test_platform", "prv_test_platform",
		"prod_integrity_Z5mMke9x0k8gpErbDqwrJXMqsI6SFli6")

	_, err := billing.Charge(t.Context(), dto.BillingChargeRequest{PaymentSourceID: "3891", AmountInCents: 2490000, Currency: "COP",
		Reference: "sk8-438k4-xmxm392-sn2m", CustomerEmail: "rosa@example.com"})

	require.NoError(t, err)
	assert.JSONEq(t, `"37c8407747e595535433ef8f6a811d853cd943046624a0ec04662b17bbf33bf5"`, string(fake.body["signature"]))
}

func TestWompiBilling_Charge_StatusesMapToChargeStatuses(t *testing.T) {
	cases := []struct {
		wompi string
		want  dto.BillingChargeStatus
	}{
		{"PENDING", dto.BillingChargePending},
		{"APPROVED", dto.BillingChargeApproved},
		{"DECLINED", dto.BillingChargeDeclined},
		{"VOIDED", dto.BillingChargeVoided},
		{"ERROR", dto.BillingChargeError},
		{"SOMETHING_NEW", dto.BillingChargePending},
	}
	for _, tc := range cases {
		t.Run(tc.wompi, func(t *testing.T) {
			fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"tx_1","status":"` + tc.wompi + `","amount_in_cents":4000000,"currency":"COP"}}`}

			tx, err := newTestWompiBilling(t, fake).Charge(t.Context(), chargeRequest())

			require.NoError(t, err)
			assert.Equal(t, tc.want, tx.Status)
			assert.Equal(t, "kb-019a0000-20261215-1", tx.Reference, "an answer without the reference keeps the one asked")
		})
	}
}

func TestWompiBilling_Charge_Failures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		answer string
		want   error
	}{
		{"request refused", http.StatusUnprocessableEntity, `{"error":{"type":"INPUT_VALIDATION_ERROR"}}`, domainError.ErrCardRefused},
		{"unknown payment source", http.StatusNotFound, `{"error":{"type":"NOT_FOUND_ERROR"}}`, domainError.ErrCardRefused},
		{"our key is refused", http.StatusUnauthorized, `{"error":{"type":"INVALID_ACCESS_TOKEN"}}`, domainError.ErrPaymentProviderUnavailable},
		{"Wompi is down", http.StatusBadGateway, `bad gateway`, domainError.ErrPaymentProviderUnavailable},
		{"an answer without a transaction", http.StatusCreated, `{"data":{}}`, domainError.ErrPaymentProviderUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeWompi{status: tc.status, answer: tc.answer}

			_, err := newTestWompiBilling(t, fake).Charge(t.Context(), chargeRequest())

			require.ErrorIs(t, err, tc.want)
			assert.NotContains(t, err.Error(), "test_integrity_platform")
			assert.NotContains(t, err.Error(), "prv_test_platform")
		})
	}
}

func TestWompiBilling_Charge_WithoutAnIntegritySecretCallsNothing(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"tx_1","status":"PENDING"}}`}
	srv := fake.server(t)
	billing := NewWompiBilling(NewClient(slog.New(slog.DiscardHandler)), srv.URL+"/v1", "pub_test_platform", "prv_test_platform", "")

	_, err := billing.Charge(t.Context(), chargeRequest())

	require.Error(t, err)
	assert.Empty(t, fake.path)
}

func TestWompiBilling_Transaction(t *testing.T) {
	fake := &fakeWompi{status: http.StatusOK, answer: `{"data":{"id":"1292-1602113476-10985","reference":"kb-1","status":"APPROVED",
		"amount_in_cents":4000000,"currency":"COP","payment_method_type":"CARD"}}`}

	tx, err := newTestWompiBilling(t, fake).Transaction(t.Context(), "1292-1602113476-10985")

	require.NoError(t, err)
	assert.Equal(t, dto.BillingTransaction{ID: "1292-1602113476-10985", Reference: "kb-1", Status: dto.BillingChargeApproved,
		AmountInCents: 4000000, Currency: "COP"}, tx)
	assert.Equal(t, http.MethodGet, fake.method)
	assert.Equal(t, "/v1/transactions/1292-1602113476-10985", fake.path)
	assert.Equal(t, "Bearer prv_test_platform", fake.auth)
}

func TestWompiBilling_Transaction_UnknownIDIsNotFound(t *testing.T) {
	fake := &fakeWompi{status: http.StatusNotFound, answer: `{"error":{"type":"NOT_FOUND_ERROR"}}`}

	_, err := newTestWompiBilling(t, fake).Transaction(t.Context(), "nope")

	require.ErrorIs(t, err, domainError.ErrBillingTransactionNotFound)
}

func TestWompiBilling_Transaction_OutageIsNotNotFound(t *testing.T) {
	fake := &fakeWompi{status: http.StatusBadGateway, answer: `bad gateway`}

	_, err := newTestWompiBilling(t, fake).Transaction(t.Context(), "tx_1")

	require.ErrorIs(t, err, domainError.ErrPaymentProviderUnavailable)
	assert.NotErrorIs(t, err, domainError.ErrBillingTransactionNotFound, "an outage must not free a charge for a second try")
}

func TestWompiBilling_TransactionByReference(t *testing.T) {
	var query, auth string
	answer := `{"data":[{"id":"tx_other","reference":"kb-10","status":"DECLINED"},
		{"id":"tx_1","reference":"kb-1","status":"APPROVED","amount_in_cents":4000000,"currency":"COP"}]}`
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, auth = r.URL.RawQuery, r.Header.Get("Authorization")
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	billing := NewWompiBilling(NewClient(slog.New(slog.DiscardHandler)), srv.URL+"/v1", "pub_test_platform", "prv_test_platform", "test_integrity_platform")

	tx, err := billing.TransactionByReference(t.Context(), "kb-1")

	require.NoError(t, err)
	assert.Equal(t, "tx_1", tx.ID, "only the exact reference is taken")
	assert.Equal(t, dto.BillingChargeApproved, tx.Status)
	assert.Equal(t, "reference=kb-1", query)
	assert.Equal(t, "Bearer prv_test_platform", auth)

	answer = `{"data":[]}`
	_, err = billing.TransactionByReference(t.Context(), "kb-unknown")
	require.ErrorIs(t, err, domainError.ErrBillingTransactionNotFound)

	answer = `{"data":[{"id":"tx_other","reference":"kb-10","status":"APPROVED"}]}`
	_, err = billing.TransactionByReference(t.Context(), "kb-1")
	require.ErrorIs(t, err, domainError.ErrBillingTransactionNotFound, "another reference's transaction is never adopted")

	for _, down := range []int{http.StatusBadGateway, http.StatusNotFound, http.StatusUnauthorized} {
		status, answer = down, `{"error":{}}`
		_, err = billing.TransactionByReference(t.Context(), "kb-1")
		require.ErrorIs(t, err, domainError.ErrPaymentProviderUnavailable)
		assert.NotErrorIs(t, err, domainError.ErrBillingTransactionNotFound, "status %d must not free a charge for a second try", down)
	}
}
