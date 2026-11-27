package httpclient

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWompiBilling(t *testing.T, fake *fakeWompi) *WompiBilling {
	srv := fake.server(t)
	billing, ok := NewWompiBilling(NewClient(slog.New(slog.DiscardHandler)), srv.URL+"/v1/", "pub_test_platform", "prv_test_platform", "test_integrity_platform").(*WompiBilling)
	require.True(t, ok)
	return billing
}

func sourceRequest() dto.PaymentSourceRequest {
	return dto.PaymentSourceRequest{CardToken: "tok_test_1_abc", CustomerEmail: "rosa@example.com", AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2"}
}

func TestWompiBilling_SavePaymentSource(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":3891,"type":"CARD","status":"AVAILABLE","customer_email":"rosa@example.com",
		"public_data":{"type":"CARD","bin":"424242","last_four":"4242","exp_month":"12","exp_year":"29"}}}`}
	billing := newTestWompiBilling(t, fake)

	source, err := billing.SavePaymentSource(t.Context(), sourceRequest())

	require.NoError(t, err)
	assert.Equal(t, dto.PaymentSource{ID: "3891", Brand: "VISA", LastFour: "4242"}, source)
	assert.Equal(t, http.MethodPost, fake.method)
	assert.Equal(t, "/v1/payment_sources", fake.path)
	assert.Equal(t, "Bearer prv_test_platform", fake.auth)
	assert.JSONEq(t, `"CARD"`, string(fake.body["type"]))
	assert.JSONEq(t, `"tok_test_1_abc"`, string(fake.body["token"]))
	assert.JSONEq(t, `"rosa@example.com"`, string(fake.body["customer_email"]))
	assert.JSONEq(t, `"acc-1"`, string(fake.body["acceptance_token"]))
	assert.JSONEq(t, `"acc-2"`, string(fake.body["accept_personal_auth"]))
	assert.NotContains(t, fake.body, "amount_in_cents", "saving a card charges nothing")
}

func TestWompiBilling_SavePaymentSource_BrandWhenWompiSendsIt(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"src_9","status":"AVAILABLE","public_data":{"brand":"mastercard","last_four":"1234"}}}`}

	source, err := newTestWompiBilling(t, fake).SavePaymentSource(t.Context(), sourceRequest())

	require.NoError(t, err)
	assert.Equal(t, dto.PaymentSource{ID: "src_9", Brand: "MASTERCARD", LastFour: "1234"}, source)
}

func TestWompiBilling_SavePaymentSource_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		status int
		answer string
	}{
		{"token refused", http.StatusUnprocessableEntity, `{"error":{"type":"INPUT_VALIDATION_ERROR","messages":{"token":["El token no existe"]}}}`},
		{"source not available", http.StatusCreated, `{"data":{"id":3892,"status":"PENDING","public_data":{}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestWompiBilling(t, &fakeWompi{status: tc.status, answer: tc.answer}).SavePaymentSource(t.Context(), sourceRequest())

			assert.ErrorIs(t, err, domainError.ErrCardRefused)
			assert.NotContains(t, err.Error(), "prv_test_platform")
		})
	}
}

func TestWompiBilling_SavePaymentSource_Unavailable(t *testing.T) {
	cases := []struct {
		name   string
		status int
		answer string
	}{
		{"wompi fails", http.StatusInternalServerError, `oops`},
		{"our key is refused", http.StatusUnauthorized, `{"error":{"type":"INVALID_ACCESS_TOKEN"}}`},
		{"answer without an id", http.StatusCreated, `{"data":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestWompiBilling(t, &fakeWompi{status: tc.status, answer: tc.answer}).SavePaymentSource(t.Context(), sourceRequest())

			assert.ErrorIs(t, err, domainError.ErrPaymentProviderUnavailable)
			assert.NotErrorIs(t, err, domainError.ErrCardRefused)
		})
	}
}

func TestWompiBilling_SavePaymentSource_Timeout(t *testing.T) {
	release := make(chan struct{})
	stalled := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(func() {
		close(release)
		stalled.Close()
	})
	billing := &WompiBilling{wompi: &WompiClient{client: NewClient(slog.New(slog.DiscardHandler)), baseURL: stalled.URL + "/v1"}, privateKey: "prv_test_platform"}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := billing.SavePaymentSource(ctx, sourceRequest())

	assert.ErrorIs(t, err, domainError.ErrPaymentProviderUnavailable)
}

func TestWompiBilling_Terms_UsesThePlatformsPublicKey(t *testing.T) {
	fake := &fakeWompi{status: http.StatusOK, answer: `{"data":{"email":"merchant@example.com",
		"presigned_acceptance":{"acceptance_token":"acc-1","permalink":"https://wompi.co/terms.pdf"},
		"presigned_personal_data_auth":{"acceptance_token":"acc-2","permalink":"https://wompi.co/data.pdf"}}}`}

	terms, err := newTestWompiBilling(t, fake).Terms(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "/v1/merchants/pub_test_platform", fake.path)
	assert.Empty(t, fake.auth, "the merchant read carries no private key")
	assert.Equal(t, "acc-1", terms.AcceptanceToken)
	assert.Equal(t, "acc-2", terms.PersonalAuthToken)
}

func TestCardBrandFromBin(t *testing.T) {
	assert.Equal(t, "VISA", cardBrandFromBin("424242"))
	assert.Equal(t, "MASTERCARD", cardBrandFromBin("555555"))
	assert.Equal(t, "MASTERCARD", cardBrandFromBin("222300"))
	assert.Equal(t, "AMEX", cardBrandFromBin("371449"))
	assert.Empty(t, cardBrandFromBin(""))
	assert.Empty(t, cardBrandFromBin("601100"))
}
