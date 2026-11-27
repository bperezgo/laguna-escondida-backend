package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func billingConfig(t *testing.T, events, integrity string) config.Billing {
	t.Helper()
	keys, err := config.NewBilling("pub_test_platform", "prv_test_platform")
	require.NoError(t, err)
	billing, err := keys.WithChargeSecrets(events, integrity)
	require.NoError(t, err)
	return billing
}

func routeExists(app *App, method, path string) bool {
	for _, r := range app.Router.Routes() {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}

// The app starts in each of the three billing configurations, and only the last one charges.
func TestCloud_BillingConfigurations(t *testing.T) {
	cases := []struct {
		name                       string
		billing                    func(t *testing.T) config.Billing
		cardRoutes, chargingRoutes bool
	}{
		{"no billing keys", func(*testing.T) config.Billing { return config.Billing{} }, false, false},
		{"keys without the charge secrets", func(t *testing.T) config.Billing { return billingConfig(t, "", "") }, true, false},
		{"all four values", func(t *testing.T) config.Billing {
			return billingConfig(t, "test_events_platform", "test_integrity_platform")
		}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testCloudConfig()
			cfg.Billing = tc.billing(t)

			app, err := NewCloud(cfg, testInfra(t))

			require.NoError(t, err)
			for _, always := range []struct{ method, path string }{
				{http.MethodGet, "/api/billing"}, {http.MethodGet, "/api/billing/card-setup"},
				{http.MethodPost, "/api/billing/cancel"}, {http.MethodPost, "/api/billing/resume"},
				{http.MethodGet, "/api/billing/recovery"}, {http.MethodPut, "/api/billing/recovery/selection"},
			} {
				assert.True(t, routeExists(app, always.method, always.path), "%s %s", always.method, always.path)
			}
			assert.Equal(t, tc.cardRoutes, routeExists(app, http.MethodPut, "/api/billing/card"))
			assert.Equal(t, tc.cardRoutes, routeExists(app, http.MethodPost, "/api/billing/setup/card"))
			assert.Equal(t, tc.chargingRoutes, routeExists(app, http.MethodPost, "/api/billing/recovery/pay"))
			assert.Equal(t, tc.chargingRoutes, routeExists(app, http.MethodPost, "/api/webhooks/wompi-billing"))
			if tc.chargingRoutes {
				assert.Contains(t, schedulerNames(app), "charge scheduler")
			} else {
				assert.NotContains(t, schedulerNames(app), "charge scheduler", "nothing is charged without the secrets")
			}
		})
	}
}

// "While the cloud has no events secret, the route SHALL answer 404."
func TestCloud_BillingWebhook_AbsentWithoutTheSecret(t *testing.T) {
	cfg := testCloudConfig()
	cfg.Billing = billingConfig(t, "", "")
	app, err := NewCloud(cfg, testInfra(t))
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/webhooks/wompi-billing", strings.NewReader(`{}`)))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// The webhook needs no credential, and an unsigned event changes nothing: it never reaches the database.
func TestCloud_BillingWebhook_UnsignedEventIs401(t *testing.T) {
	cfg := testCloudConfig()
	cfg.Billing = billingConfig(t, "test_events_platform", "test_integrity_platform")
	app, err := NewCloud(cfg, testInfra(t))
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/webhooks/wompi-billing",
		strings.NewReader(`{"event":"transaction.updated","data":{"transaction":{"id":"tx_1","status":"APPROVED","reference":"kb-x"}},"timestamp":1}`)))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBillingRoutes_NeedTheirCredential(t *testing.T) {
	cfg := testCloudConfig()
	cfg.Billing = billingConfig(t, "test_events_platform", "test_integrity_platform")
	app, err := NewCloud(cfg, testInfra(t))
	require.NoError(t, err)

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/billing"}, {http.MethodGet, "/api/billing/card-setup"}, {http.MethodPut, "/api/billing/card"},
		{http.MethodPost, "/api/billing/cancel"}, {http.MethodPost, "/api/billing/resume"},
		{http.MethodGet, "/api/billing/recovery"}, {http.MethodPut, "/api/billing/recovery/selection"}, {http.MethodPost, "/api/billing/recovery/pay"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.Router.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
			assert.Equal(t, http.StatusUnauthorized, rec.Code)

			rec = httptest.NewRecorder()
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			req.Header.Set("X-API-Key", "admin-key")
			app.Router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "the platform key opens no billing route")
		})
	}
}

func TestEdge_ServesNoBillingRoute(t *testing.T) {
	app, err := NewEdge(testEdgeConfig(), testInfra(t))
	require.NoError(t, err)

	for _, r := range app.Router.Routes() {
		assert.NotContains(t, r.Path, "/api/billing", "an edge box has no billing")
		assert.NotContains(t, r.Path, "wompi-billing")
	}
}
