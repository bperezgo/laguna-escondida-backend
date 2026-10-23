package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var platformOrganizationRoutes = []struct{ method, path string }{
	{http.MethodPost, "/api/platform/organizations"},
	{http.MethodGet, "/api/platform/organizations"},
	{http.MethodGet, "/api/platform/organizations/:id"},
	{http.MethodPatch, "/api/platform/organizations/:id"},
	{http.MethodPut, "/api/platform/organizations/:id/einvoicing"},
}

func servePlatformRoute(t *testing.T, mode config.Mode, method, path string, headers map[string]string) int {
	t.Helper()
	router := fullRouter(mode, service.NewJWTService("test-secret", ""))
	req := httptest.NewRequest(method, pathParams.Replace(path), strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestPlatformOrganizationRoutes_RequireTheAdminAPIKey(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", "")
	adminToken, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000ad", testOrgID, "admin", []int{permissions.RoleAdmin})
	require.NoError(t, err)
	agentToken, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000aa", testOrgID, "whatsapp-agent", []int{permissions.RoleAgent})
	require.NoError(t, err)

	callers := []struct {
		name    string
		headers map[string]string
	}{
		{"no credentials", nil},
		{"admin JWT", map[string]string{"Authorization": "Bearer " + adminToken}},
		{"agent JWT", map[string]string{"Authorization": "Bearer " + agentToken}},
		{"wrong API key", map[string]string{"X-API-Key": "not-the-key"}},
	}
	for _, route := range platformOrganizationRoutes {
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

func TestPlatformOrganizationRoutes_NotServedOnTheEdge(t *testing.T) {
	registered := map[string]bool{}
	for _, route := range fullRouter(config.ModeEdge, service.NewJWTService("test-secret", "")).Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, route := range platformOrganizationRoutes {
		assert.False(t, registered[route.method+" "+route.path], "%s %s must be cloud-only", route.method, route.path)
		assert.Equal(t, http.StatusNotFound,
			servePlatformRoute(t, config.ModeEdge, route.method, route.path, map[string]string{"X-API-Key": "admin-key"}))
	}
}

func TestPlatformOrganizationRoutes_NoDelete(t *testing.T) {
	code := servePlatformRoute(t, config.ModeCloud, http.MethodDelete, "/api/platform/organizations/:id", map[string]string{"X-API-Key": "admin-key"})

	assert.Equal(t, http.StatusNotFound, code)
}
