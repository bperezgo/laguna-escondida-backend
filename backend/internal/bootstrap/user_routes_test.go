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

func TestUserRoutes_GeneratePasswordIsGone(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	token, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000aa", testOrgID, "admin", []int{permissions.RoleAdmin})
	require.NoError(t, err)
	router := fullRouter(config.ModeCloud, jwtService)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/some-id/generate-password", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUserRoutes_ResetPasswordNeedsUsersUpdate(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	token, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000bb", testOrgID, "mesero", []int{permissions.RoleServer})
	require.NoError(t, err)
	router := fullRouter(config.ModeCloud, jwtService)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/some-id/reset-password", strings.NewReader(`{"password":"NuevaClave123"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
