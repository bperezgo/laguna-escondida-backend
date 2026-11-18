package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var orgSettingsRoutes = []string{http.MethodGet, http.MethodPatch}

// An unauthenticated request stops at 401 when the route exists and 404 when it does not.
func TestOrganizationSettingsRoutes_AbsentOnTheEdge(t *testing.T) {
	router := fullRouter(config.ModeEdge, service.NewJWTService("test-secret", tenant.AnyOrganization()))
	for _, method := range orgSettingsRoutes {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, "/api/organization/settings", nil))
		assert.Equal(t, http.StatusNotFound, rec.Code, method)
	}
}

func TestOrganizationSettingsRoutes_PresentInTheCloud(t *testing.T) {
	router := fullRouter(config.ModeCloud, service.NewJWTService("test-secret", tenant.AnyOrganization()))
	for _, method := range orgSettingsRoutes {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, "/api/organization/settings", nil))
		assert.Equal(t, http.StatusUnauthorized, rec.Code, method)
	}
}

func TestOrganizationSettingsRoutes_RequireOrderingRead(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	router := fullRouter(config.ModeCloud, jwtService)

	tests := []struct {
		name    string
		roleID  int
		allowed bool
	}{
		{"agent", permissions.RoleAgent, true},
		{"accountant", permissions.RoleAccountant, true},
		{"manager", permissions.RoleManager, true},
		{"server", permissions.RoleServer, false},
		{"cooker", permissions.RoleCooker, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000bb", testOrgID, "user", []int{tt.roleID})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodGet, "/api/organization/settings", strings.NewReader(""))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if tt.allowed {
				assert.NotEqual(t, http.StatusForbidden, rec.Code, "past the guard, the nil service turns into a 500")
				assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, rec.Code)
			}
		})
	}
}
