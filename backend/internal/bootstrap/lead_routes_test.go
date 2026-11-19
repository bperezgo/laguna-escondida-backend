package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var platformLeadRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/platform/leads"},
	{http.MethodGet, "/api/platform/leads/:id"},
	{http.MethodPatch, "/api/platform/leads/:id"},
	{http.MethodDelete, "/api/platform/leads/:id"},
}

func TestPlatformLeadRoutes_RequireTheAdminAPIKey(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	adminToken, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000ad", testOrgID, "admin", []int{permissions.RoleAdmin})
	require.NoError(t, err)

	callers := []struct {
		name    string
		headers map[string]string
	}{
		{"no credentials", nil},
		{"admin user token", map[string]string{"Authorization": "Bearer " + adminToken}},
		{"wrong API key", map[string]string{"X-API-Key": "not-the-key"}},
	}
	for _, route := range platformLeadRoutes {
		for _, caller := range callers {
			t.Run(route.method+" "+route.path+" "+caller.name, func(t *testing.T) {
				assert.Equal(t, http.StatusUnauthorized, servePlatformRoute(t, config.ModeCloud, route.method, route.path, caller.headers))
			})
		}
		t.Run(route.method+" "+route.path+" admin API key", func(t *testing.T) {
			code := servePlatformRoute(t, config.ModeCloud, route.method, route.path, map[string]string{"X-API-Key": "admin-key"})
			assert.NotEqual(t, http.StatusUnauthorized, code, "the key gets past the guard")
			assert.NotEqual(t, http.StatusNotFound, code)
		})
	}
}

func TestLeadRoutes_NotServedOnTheEdge(t *testing.T) {
	all := append([]struct{ method, path string }{{http.MethodPost, "/api/public/leads"}}, platformLeadRoutes...)
	registered := map[string]bool{}
	for _, route := range fullRouter(config.ModeEdge, service.NewJWTService("test-secret", tenant.AnyOrganization())).Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, route := range all {
		assert.False(t, registered[route.method+" "+route.path], "%s %s must be cloud-only", route.method, route.path)
		assert.Equal(t, http.StatusNotFound,
			servePlatformRoute(t, config.ModeEdge, route.method, route.path, map[string]string{"X-API-Key": "admin-key"}))
	}
}

func TestPublicLeadRoute_NeedsNoCredential(t *testing.T) {
	code := servePlatformRoute(t, config.ModeCloud, http.MethodPost, "/api/public/leads", nil)

	assert.NotEqual(t, http.StatusUnauthorized, code)
	assert.NotEqual(t, http.StatusNotFound, code)
}

func clientIPBehind(t *testing.T, proxies []string, peer, forwardedFor string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, applyTrustedProxies(router, proxies))
	router.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = peer + ":4000"
	req.Header.Set("X-Forwarded-For", forwardedFor)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestApplyTrustedProxies_ForgedForwardedForIsIgnoredFromAnUntrustedPeer(t *testing.T) {
	got := clientIPBehind(t, []string{"10.0.0.0/16"}, "203.0.113.9", "1.2.3.4")

	assert.Equal(t, "203.0.113.9", got)
}

func TestApplyTrustedProxies_ForwardedForIsHonouredFromATrustedPeer(t *testing.T) {
	got := clientIPBehind(t, []string{"10.0.0.0/16"}, "10.0.1.7", "198.51.100.20")

	assert.Equal(t, "198.51.100.20", got)
}

func TestApplyTrustedProxies_UnsetKeepsTheCurrentBehaviour(t *testing.T) {
	got := clientIPBehind(t, nil, "203.0.113.9", "1.2.3.4")

	assert.Equal(t, "1.2.3.4", got)
}

func schedulerNames(app *App) []string {
	names := make([]string, len(app.schedulers))
	for i, s := range app.schedulers {
		names[i] = s.name
	}
	return names
}

func TestNewCloud_LeadAlertSchedulerOnlyWhenTheWebhookIsConfigured(t *testing.T) {
	without, err := NewCloud(testCloudConfig(), testInfra(t))
	require.NoError(t, err)
	assert.NotContains(t, schedulerNames(without), "lead alert scheduler")

	cfg := testCloudConfig()
	cfg.LeadAlerts = config.LeadAlerts{DiscordWebhookURL: "https://discord.com/api/webhooks/1/x"}
	with, err := NewCloud(cfg, testInfra(t))
	require.NoError(t, err)
	assert.Contains(t, schedulerNames(with), "lead alert scheduler")
}
