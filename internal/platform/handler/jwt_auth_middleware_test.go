package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	authOrgA = "019a0000-0000-7000-8000-00000000000a"
	authOrgB = "019a0000-0000-7000-8000-00000000000b"
)

// serveWithAuth runs one request through JWTAuthMiddleware and reports the org the handler saw.
func serveWithAuth(t *testing.T, jwtService *service.JWTService, token string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var seen string
	router := gin.New()
	router.GET("/probe", JWTAuthMiddleware(jwtService), func(c *gin.Context) {
		seen, _ = tenant.OrgFrom(c.Request.Context())
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec, seen
}

func issue(t *testing.T, orgID string) string {
	t.Helper()
	token, err := service.NewJWTService("test-secret", "").GenerateToken("user-1", orgID, "cajero1", []int{permissions.RoleServer})
	require.NoError(t, err)
	return token
}

func TestJWTAuthMiddleware_PutsTheTokensOrganizationInTheContext(t *testing.T) {
	rec, seen := serveWithAuth(t, service.NewJWTService("test-secret", ""), issue(t, authOrgB))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, authOrgB, seen)
}

func TestJWTAuthMiddleware_TokenWithoutOrganization_Returns401(t *testing.T) {
	rec, seen := serveWithAuth(t, service.NewJWTService("test-secret", ""), issue(t, ""))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, seen)
}

func TestJWTAuthMiddleware_EdgeBox_TokenOfAnotherOrganization_Returns401(t *testing.T) {
	box := service.NewJWTService("test-secret", authOrgA)

	rec, _ := serveWithAuth(t, box, issue(t, authOrgB))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec, seen := serveWithAuth(t, box, issue(t, authOrgA))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, authOrgA, seen)
}
