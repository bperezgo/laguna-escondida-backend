package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/crypto"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// integrationsKey encrypts every suite's organization integrations.
var integrationsKey = []byte("acceptance-integrations-key-0032")

// Integrations is the cloud setting that lets a suite store organizations' Wompi keys.
func Integrations() config.Integrations {
	return config.Integrations{SecretsKey: integrationsKey}
}

// PaymentLinks points the cloud at a fake Wompi that creates a link for any key and deactivates
// any link, so suites that aren't about payments can take agent orders.
func PaymentLinks(t *testing.T) config.PaymentLinks {
	t.Helper()
	wompi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"data":{"id":"wl_%s","active":true}}`, uuid.NewString()[:8])
			return
		}
		_, _ = w.Write([]byte(`{"data":{"active":false}}`))
	}))
	t.Cleanup(wompi.Close)
	return config.NewPaymentLinks(wompi.URL+"/v1", 30*time.Minute)
}

// IntegrationService is the cloud's integration service on db, under Integrations' key.
func IntegrationService(t *testing.T, db *gorm.DB, wompiAPIURL string) *service.OrganizationIntegrationService {
	t.Helper()
	cipher, err := crypto.NewAESGCMCipher(integrationsKey)
	require.NoError(t, err)
	return service.NewOrganizationIntegrationService(repository.NewOrganizationIntegrationRepository(db),
		repository.NewOrganizationRepository(db), cipher, wompiAPIURL, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// ConfigureWompi gives the organization a Wompi integration, as a restaurant with payment links has.
func ConfigureWompi(t *testing.T, db *gorm.DB, orgID string) {
	t.Helper()
	_, err := IntegrationService(t, db, "http://127.0.0.1/v1").SetWompi(context.Background(), orgID,
		dto.WompiCredentials{PrivateKey: "prv_test_accept", EventsSecret: "test_events_accept"})
	require.NoError(t, err)
}

// FakeBillingWompi is the platform's own Wompi account: it answers the merchant's terms, turns a
// card token into a payment source, and counts transactions, which saving a card must never create.
// Everything else it answers as PaymentLinks' fake does.
type FakeBillingWompi struct {
	server *httptest.Server
	mu     sync.Mutex
	// Sources are the bodies of every payment source request, in order.
	Sources      []map[string]string
	Transactions int
}

const (
	FakeBillingPublicKey  = "pub_test_accept_platform"
	FakeBillingPrivateKey = "prv_test_accept_platform"
	// RefusedCardToken is a card token the fake refuses, as Wompi refuses one it does not know.
	RefusedCardToken = "tok_test_refused"
)

func BillingWompi(t *testing.T) *FakeBillingWompi {
	t.Helper()
	fake := &FakeBillingWompi{}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *FakeBillingWompi) PaymentLinks() config.PaymentLinks {
	return config.NewPaymentLinks(f.server.URL+"/v1", 30*time.Minute)
}

func (f *FakeBillingWompi) Billing() config.Billing {
	return config.Billing{WompiPublicKey: FakeBillingPublicKey, WompiPrivateKey: FakeBillingPrivateKey}
}

func (f *FakeBillingWompi) Counts() (sources, transactions int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Sources), f.Transactions
}

func (f *FakeBillingWompi) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/merchants/"+FakeBillingPublicKey:
		_, _ = fmt.Fprintf(w, `{"data":{"email":"platform@example.com",
			"presigned_acceptance":{"acceptance_token":"acc_%s","permalink":"https://wompi.example/terms.pdf"},
			"presigned_personal_data_auth":{"acceptance_token":"pda_%s","permalink":"https://wompi.example/data.pdf"}}}`,
			uuid.NewString()[:8], uuid.NewString()[:8])
	case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_sources":
		f.paymentSource(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/transactions":
		f.mu.Lock()
		f.Transactions++
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"data":{"id":"tx_%s","status":"PENDING"}}`, uuid.NewString()[:8])
	case r.Method == http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"data":{"id":"wl_%s","active":true}}`, uuid.NewString()[:8])
	default:
		_, _ = w.Write([]byte(`{"data":{"active":false}}`))
	}
}

func (f *FakeBillingWompi) paymentSource(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	if r.Header.Get("Authorization") != "Bearer "+FakeBillingPrivateKey {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"INVALID_ACCESS_TOKEN"}}`))
		return
	}
	f.mu.Lock()
	f.Sources = append(f.Sources, body)
	id := 3000 + len(f.Sources)
	f.mu.Unlock()
	if body["token"] == RefusedCardToken || body["acceptance_token"] == "" || body["accept_personal_auth"] == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"type":"INPUT_VALIDATION_ERROR","messages":{"token":["El token no existe"]}}}`))
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"data":{"id":%d,"type":"CARD","status":"AVAILABLE","customer_email":%q,
		"public_data":{"type":"CARD","bin":"424242","last_four":"4242","exp_month":"12","exp_year":"29"}}}`, id, body["customer_email"])
}
