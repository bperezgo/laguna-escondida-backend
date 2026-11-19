package httpclient

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWompi answers the payment links API as Wompi's sandbox does, and records what it was sent.
type fakeWompi struct {
	method, path, auth string
	body               map[string]json.RawMessage
	status             int
	answer             string
}

func (f *fakeWompi) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.method, f.path, f.auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		f.body = nil
		_ = json.Unmarshal(raw, &f.body)
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.answer))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestWompiClient(t *testing.T, fake *fakeWompi) *WompiClient {
	srv := fake.server(t)
	return &WompiClient{client: NewClient(slog.New(slog.DiscardHandler)), baseURL: srv.URL + "/v1"}
}

func TestNewWompiClient_TrimsTheBaseURL(t *testing.T) {
	client, ok := NewWompiClient(NewClient(slog.New(slog.DiscardHandler)), "https://sandbox.wompi.co/v1/").(*WompiClient)

	require.True(t, ok)
	assert.Equal(t, "https://sandbox.wompi.co/v1", client.baseURL)
}

func TestWompiClient_CreateLink(t *testing.T) {
	fake := &fakeWompi{status: http.StatusCreated, answer: `{"data":{"id":"3Z0Cfi","active":true}}`}
	client := newTestWompiClient(t, fake)

	id, err := client.CreateLink(t.Context(), "prv_test_x", dto.PaymentLinkRequest{
		Name: "Laguna Escondida", Description: "Pedido 0ABC12", AmountInCents: 4500000, Currency: "COP",
		ExpiresAt: time.Date(2026, 11, 18, 12, 30, 0, 0, time.FixedZone("COT", -5*3600)), SKU: "019a0000-0000-7000-8000-0000000abc12",
	})

	require.NoError(t, err)
	assert.Equal(t, "3Z0Cfi", id)
	assert.Equal(t, http.MethodPost, fake.method)
	assert.Equal(t, "/v1/payment_links", fake.path)
	assert.Equal(t, "Bearer prv_test_x", fake.auth)
	assert.JSONEq(t, `true`, string(fake.body["single_use"]))
	assert.JSONEq(t, `false`, string(fake.body["collect_shipping"]))
	assert.JSONEq(t, `4500000`, string(fake.body["amount_in_cents"]))
	assert.JSONEq(t, `"COP"`, string(fake.body["currency"]))
	assert.JSONEq(t, `"2026-11-18T17:30:00Z"`, string(fake.body["expires_at"]), "UTC, as Wompi asks")
	assert.JSONEq(t, `"019a0000-0000-7000-8000-0000000abc12"`, string(fake.body["sku"]))
	assert.Equal(t, "https://checkout.wompi.co/l/3Z0Cfi", client.CheckoutURL(id))
}

func TestWompiClient_CreateLink_Refused(t *testing.T) {
	for name, fake := range map[string]*fakeWompi{
		"an error status": {status: http.StatusUnprocessableEntity, answer: `{"error":{"type":"INPUT_VALIDATION_ERROR"}}`},
		"no id":           {status: http.StatusCreated, answer: `{"data":{}}`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newTestWompiClient(t, fake).CreateLink(t.Context(), "prv_test_x", dto.PaymentLinkRequest{AmountInCents: 1})

			require.Error(t, err)
		})
	}
}

func TestWompiClient_DeactivateLink(t *testing.T) {
	fake := &fakeWompi{status: http.StatusOK, answer: `{"data":{"id":"3Z0Cfi","active":false}}`}

	require.NoError(t, newTestWompiClient(t, fake).DeactivateLink(t.Context(), "prv_test_x", "3Z0Cfi"))

	assert.Equal(t, http.MethodPatch, fake.method)
	assert.Equal(t, "/v1/payment_links/3Z0Cfi", fake.path)
	assert.Equal(t, "Bearer prv_test_x", fake.auth)
	assert.JSONEq(t, `false`, string(fake.body["active"]))
}

func TestWompiClient_DeactivateLink_Refused(t *testing.T) {
	fake := &fakeWompi{status: http.StatusNotFound, answer: `{"error":"not found"}`}

	require.ErrorContains(t, newTestWompiClient(t, fake).DeactivateLink(t.Context(), "prv_test_x", "nope"), "404")
}
