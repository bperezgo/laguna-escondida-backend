package testsupport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
// card token into a payment source, charges a source, answers transaction lookups and signs
// events. It counts every transaction request, which saving a card must never make.
// Everything else it answers as PaymentLinks' fake does.
type FakeBillingWompi struct {
	server *httptest.Server
	mu     sync.Mutex
	// Sources are the bodies of every payment source request, in order.
	Sources      []map[string]string
	Transactions int
	// Charges are the transactions created against a payment source, in order.
	Charges []FakeTransaction
	// nextStatus is the status the next created transaction answers with and keeps.
	nextStatus string
}

// FakeTransaction is a transaction as the fake holds it: what was asked and its current status.
type FakeTransaction struct {
	ID              string
	Reference       string
	Status          string
	AmountInCents   int64
	Currency        string
	CustomerEmail   string
	PaymentSourceID string
}

const (
	FakeBillingPublicKey  = "pub_test_accept_platform"
	FakeBillingPrivateKey = "prv_test_accept_platform"
	// FakeBillingEventsSecret signs the fake's events; FakeBillingIntegritySecret is what it checks a charge's signature with.
	FakeBillingEventsSecret    = "test_events_accept_platform" //nolint:gosec // a fake account's secret, used only against the fake
	FakeBillingIntegritySecret = "test_integrity_accept_platform"
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

// ChargingBilling is all four values: cards are saved and charged.
func (f *FakeBillingWompi) ChargingBilling() config.Billing {
	return config.Billing{WompiPublicKey: FakeBillingPublicKey, WompiPrivateKey: FakeBillingPrivateKey,
		WompiEventsSecret: FakeBillingEventsSecret, WompiIntegritySecret: FakeBillingIntegritySecret}
}

// APIURL is the fake's base address, as WOMPI_API_URL would be.
func (f *FakeBillingWompi) APIURL() string { return f.server.URL + "/v1" }

// AnswerWith makes every transaction created from now on answer, and stay, with status.
func (f *FakeBillingWompi) AnswerWith(status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextStatus = status
}

// ChargeList is a copy of the transactions created against payment sources.
func (f *FakeBillingWompi) ChargeList() []FakeTransaction {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeTransaction(nil), f.Charges...)
}

// SetStatus changes a transaction's status, as the bank's answer does.
func (f *FakeBillingWompi) SetStatus(id, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.Charges {
		if f.Charges[i].ID == id {
			f.Charges[i].Status = status
		}
	}
}

// Event is the transaction.updated event Wompi would send for the transaction as it is now,
// signed with secret.
func (f *FakeBillingWompi) Event(t *testing.T, id, secret string) []byte {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, tx := range f.Charges {
		if tx.ID != id {
			continue
		}
		const timestamp = 1530291411
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d%d%s", tx.ID, tx.Status, tx.AmountInCents, timestamp, secret)))
		body, err := json.Marshal(map[string]any{
			"event": "transaction.updated",
			"data": map[string]any{"transaction": map[string]any{
				"id": tx.ID, "status": tx.Status, "amount_in_cents": tx.AmountInCents, "currency": tx.Currency,
				"reference": tx.Reference, "customer_email": tx.CustomerEmail, "payment_method_type": "CARD",
			}},
			"signature": map[string]any{
				"properties": []string{"transaction.id", "transaction.status", "transaction.amount_in_cents"},
				"checksum":   hex.EncodeToString(sum[:]),
			},
			"timestamp": timestamp,
		})
		require.NoError(t, err)
		return body
	}
	t.Fatalf("the fake Wompi has no transaction %s", id)
	return nil
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
		f.transaction(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/transactions":
		f.transactionsByReference(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/transactions/"):
		f.transactionByID(w, r)
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

// transaction charges a payment source as Wompi does: it wants the private key, a source it
// issued, and the integrity signature of reference, amount and currency.
func (f *FakeBillingWompi) transaction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AmountInCents   int64           `json:"amount_in_cents"`
		Currency        string          `json:"currency"`
		Signature       string          `json:"signature"`
		CustomerEmail   string          `json:"customer_email"`
		Reference       string          `json:"reference"`
		PaymentSourceID json.RawMessage `json:"payment_source_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Transactions++
	if r.Header.Get("Authorization") != "Bearer "+FakeBillingPrivateKey {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"INVALID_ACCESS_TOKEN"}}`))
		return
	}
	want := sha256.Sum256([]byte(fmt.Sprintf("%s%d%s%s", body.Reference, body.AmountInCents, body.Currency, FakeBillingIntegritySecret)))
	source := strings.Trim(string(body.PaymentSourceID), `"`)
	if body.Signature != hex.EncodeToString(want[:]) || source == "" || source == "null" || body.CustomerEmail == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"type":"INPUT_VALIDATION_ERROR","messages":{"signature":["La firma es inválida"]}}}`))
		return
	}
	for _, existing := range f.Charges {
		if existing.Reference == body.Reference {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"type":"INPUT_VALIDATION_ERROR","messages":{"reference":["La referencia ya ha sido usada"]}}}`))
			return
		}
	}
	status := f.nextStatus
	if status == "" {
		status = "PENDING"
	}
	tx := FakeTransaction{ID: "tx_" + uuid.NewString()[:8], Reference: body.Reference, Status: status,
		AmountInCents: body.AmountInCents, Currency: body.Currency, CustomerEmail: body.CustomerEmail, PaymentSourceID: source}
	f.Charges = append(f.Charges, tx)
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, `{"data":%s}`, tx.json())
}

func (t FakeTransaction) json() string {
	raw, _ := json.Marshal(map[string]any{"id": t.ID, "reference": t.Reference, "status": t.Status,
		"amount_in_cents": t.AmountInCents, "currency": t.Currency, "customer_email": t.CustomerEmail, "payment_method_type": "CARD"})
	return string(raw)
}

func (f *FakeBillingWompi) transactionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/transactions/")
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") == "Bearer "+FakeBillingPrivateKey {
		for _, tx := range f.Charges {
			if tx.ID == id {
				_, _ = fmt.Fprintf(w, `{"data":%s}`, tx.json())
				return
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":{"type":"NOT_FOUND_ERROR"}}`))
}

func (f *FakeBillingWompi) transactionsByReference(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+FakeBillingPrivateKey {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"INVALID_ACCESS_TOKEN"}}`))
		return
	}
	found := []string{}
	for _, tx := range f.Charges {
		if tx.Reference == r.URL.Query().Get("reference") {
			found = append(found, tx.json())
		}
	}
	_, _ = fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(found, ","))
}
