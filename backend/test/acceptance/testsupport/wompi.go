package testsupport

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
