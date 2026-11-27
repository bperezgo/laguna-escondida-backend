package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type setupProbe struct {
	orgID, userID string
	reached       bool
}

// serveWithSetupAuth runs one request through SetupAuthMiddleware for an organization in status.
func serveWithSetupAuth(t *testing.T, status, authorization string) (*httptest.ResponseRecorder, setupProbe) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	reader := mocks.NewMockOrganizationAccessReader(t)
	if status == "" {
		reader.EXPECT().GetAccess(mock.Anything, authOrgA).Return(nil, domainError.ErrOrganizationNotFound).Maybe()
	} else {
		reader.EXPECT().GetAccess(mock.Anything, authOrgA).Return(&dto.OrganizationAccess{Status: status, Plan: "starter"}, nil).Maybe()
	}
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	var probe setupProbe
	router := gin.New()
	router.GET("/api/billing/setup", SetupAuthMiddleware(jwtService, service.NewOrganizationAccessService(reader)), func(c *gin.Context) {
		probe.reached = true
		probe.orgID, _ = tenant.OrgFrom(c.Request.Context())
		probe.userID = c.GetString("user_id")
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/billing/setup", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec, probe
}

func setupToken(t *testing.T) string {
	t.Helper()
	token, err := service.NewJWTService("test-secret", tenant.AnyOrganization()).GenerateSetupToken("user-1", authOrgA)
	require.NoError(t, err)
	return token
}

func TestSetupAuthMiddleware_NoToken_Returns401(t *testing.T) {
	rec, probe := serveWithSetupAuth(t, "pending", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, probe.reached)
}

func TestSetupAuthMiddleware_SignInToken_Returns401(t *testing.T) {
	signIn, err := service.NewJWTService("test-secret", tenant.AnyOrganization()).GenerateToken("user-1", authOrgA, "rosa", []int{permissions.RoleAdmin})
	require.NoError(t, err)

	rec, probe := serveWithSetupAuth(t, "trial", "Bearer "+signIn)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"Invalid or expired token"}`, rec.Body.String())
	assert.False(t, probe.reached)
}

func TestSetupAuthMiddleware_APIKey_Returns401(t *testing.T) {
	rec, probe := serveWithSetupAuth(t, "pending", "Bearer "+dto.APIKeyPrefix+"abc")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, probe.reached)
}

func TestSetupAuthMiddleware_TrialAlreadyStarted_Returns409(t *testing.T) {
	rec, probe := serveWithSetupAuth(t, "trial", "Bearer "+setupToken(t))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `"not_pending"`, string(decodeBody(t, rec)["code"]))
	assert.False(t, probe.reached)
}

func TestSetupAuthMiddleware_PendingOrganization_ReachesTheHandler(t *testing.T) {
	rec, probe := serveWithSetupAuth(t, "pending", "Bearer "+setupToken(t))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, authOrgA, probe.orgID)
	assert.Equal(t, "user-1", probe.userID)
}

func TestSetupAuthMiddleware_UnknownOrganization_Returns401(t *testing.T) {
	rec, probe := serveWithSetupAuth(t, "", "Bearer "+setupToken(t))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, probe.reached)
}

// A setup token is no sign-in anywhere else.
func TestJWTAuthMiddleware_SetupToken_Returns401(t *testing.T) {
	rec, seen := serveWithAuth(t, service.NewJWTService("test-secret", tenant.AnyOrganization()), setupToken(t))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, seen)
}
