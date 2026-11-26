package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type accessStub struct{ access *dto.OrganizationAccess }

func cloudJWT(t *testing.T, stub *accessStub) (*service.JWTService, *mocks.MockOrganizationAccessReader) {
	t.Helper()
	reader := mocks.NewMockOrganizationAccessReader(t)
	reader.EXPECT().GetAccess(mock.Anything, authOrgA).RunAndReturn(
		func(_ context.Context, _ string) (*dto.OrganizationAccess, error) { return stub.access, nil }).Maybe()
	jwt := service.NewJWTService("test-secret", tenant.AnyOrganization())
	jwt.EnforceOrganizationAccess(service.NewOrganizationAccessService(reader))
	return jwt, reader
}

func serveRoute(t *testing.T, jwt *service.JWTService, route, token string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(route, JWTAuthMiddleware(jwt), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, route, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func managerToken(t *testing.T) string {
	t.Helper()
	token, err := service.NewJWTService("test-secret", tenant.AnyOrganization()).GenerateToken("user-1", authOrgA, "ana", []int{permissions.RoleManager})
	require.NoError(t, err)
	return token
}

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var b map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	return b
}

func TestJWTAuthMiddleware_OrganizationAccess_Codes(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name     string
		access   dto.OrganizationAccess
		route    string
		wantCode string
		wantPlan string
	}{
		{"suspended", dto.OrganizationAccess{Status: "suspended", Plan: "full"}, "/api/products", "organization_suspended", ""},
		{"pending", dto.OrganizationAccess{Status: "pending", Plan: "full"}, "/api/products", "pending_approval", ""},
		{"pending starter on invoices gets pending_approval", dto.OrganizationAccess{Status: "pending", Plan: "starter"}, "/api/invoices", "pending_approval", ""},
		{"expired starter on invoices gets trial_expired", dto.OrganizationAccess{Status: "trial", Plan: "starter", TrialEndsAt: &past}, "/api/invoices", "trial_expired", ""},
		{"starter on invoices", dto.OrganizationAccess{Status: "active", Plan: "starter"}, "/api/invoices", "plan_required", "einvoicing"},
		{"einvoicing on reservations", dto.OrganizationAccess{Status: "active", Plan: "einvoicing"}, "/api/reservations", "plan_required", "full"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access := tc.access
			jwt, _ := cloudJWT(t, &accessStub{access: &access})

			rec := serveRoute(t, jwt, tc.route, managerToken(t))

			require.Equal(t, http.StatusForbidden, rec.Code)
			b := body(t, rec)
			assert.Equal(t, tc.wantCode, b["code"])
			assert.Equal(t, tc.wantPlan, b["required_plan"])
		})
	}
}

func TestJWTAuthMiddleware_OrganizationAccess_CoreRouteOnStarter(t *testing.T) {
	jwt, _ := cloudJWT(t, &accessStub{access: &dto.OrganizationAccess{Status: "active", Plan: "starter"}})
	assert.Equal(t, http.StatusOK, serveRoute(t, jwt, "/api/products", managerToken(t)).Code)
}

func TestJWTAuthMiddleware_OrganizationAccess_TokenIssuedBeforeExpiryIsRefusedAfter(t *testing.T) {
	future := time.Now().Add(time.Hour)
	stub := &accessStub{access: &dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: &future}}
	jwt, _ := cloudJWT(t, stub)
	token := managerToken(t)
	require.Equal(t, http.StatusOK, serveRoute(t, jwt, "/api/products", token).Code)

	past := time.Now().Add(-time.Second)
	stub.access = &dto.OrganizationAccess{Status: "trial", Plan: "full", TrialEndsAt: &past}
	rec := serveRoute(t, jwt, "/api/products", token)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "trial_expired", body(t, rec)["code"])
}

func TestJWTAuthMiddleware_OrganizationAccess_EdgeMakesNoCall(t *testing.T) {
	reader := mocks.NewMockOrganizationAccessReader(t)
	edge := service.NewJWTService("test-secret", tenant.PinnedTo(authOrgA))

	rec := serveRoute(t, edge, "/api/invoices", managerToken(t))

	assert.Equal(t, http.StatusOK, rec.Code)
	reader.AssertNotCalled(t, "GetAccess", mock.Anything, mock.Anything)
}
