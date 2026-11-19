package testsupport

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"laguna-escondida/backend/internal/platform/config"

	"github.com/google/uuid"
)

// orgSlugPool is how many organizations a suite can seed with payment links configured.
const orgSlugPool = 500

var nextOrgSlug atomic.Int64

// NextOrgSlug is the slug for the next seeded organization. Every one has payment links in
// PaymentLinks, so an agent order is taken there as in a configured restaurant.
func NextOrgSlug() string {
	n := nextOrgSlug.Add(1)
	if n > orgSlugPool {
		panic("testsupport: more seeded organizations than the payment link slug pool")
	}
	return orgSlug(n)
}

func orgSlug(n int64) string {
	return fmt.Sprintf("accept-org-%03d", n)
}

// PaymentLinks configures every NextOrgSlug organization against a fake Wompi that creates a
// link for any key and deactivates any link, so suites that aren't about payments can take agent orders.
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
	keys, secrets := map[string]string{}, map[string]string{}
	for n := int64(1); n <= orgSlugPool; n++ {
		keys[orgSlug(n)], secrets[orgSlug(n)] = "prv_test_accept", "test_events_accept"
	}
	return config.NewPaymentLinks(wompi.URL+"/v1", 30*time.Minute, keys, secrets)
}
