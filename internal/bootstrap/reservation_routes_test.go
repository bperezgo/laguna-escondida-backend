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

const testReservationID = "019a0000-0000-7000-8000-000000000001"

func reservationRoutes() []struct{ method, path string } {
	return []struct{ method, path string }{
		{http.MethodGet, "/api/reservations"},
		{http.MethodGet, "/api/reservations/" + testReservationID},
		{http.MethodPost, "/api/reservations"},
		{http.MethodPost, "/api/reservations/" + testReservationID + "/confirm"},
		{http.MethodPost, "/api/reservations/" + testReservationID + "/decline"},
		{http.MethodPost, "/api/reservations/" + testReservationID + "/cancel"},
		{http.MethodPost, "/api/reservations/" + testReservationID + "/no-show"},
		{http.MethodGet, "/api/contacts"},
		{http.MethodGet, "/api/contacts/" + testReservationID},
		{http.MethodGet, "/api/feedback"},
		{http.MethodPost, "/api/feedback"},
	}
}

func tokenFor(t *testing.T, jwtService *service.JWTService, roles ...int) string {
	t.Helper()
	token, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000bb", testOrgID, "someone", roles)
	require.NoError(t, err)
	return token
}

func serve(t *testing.T, mode config.Mode, jwtService *service.JWTService, token, method, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	fullRouter(mode, jwtService).ServeHTTP(rec, req)
	return rec.Code
}

func TestRegisterReservationRoutes_AbsentOnTheEdge(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	for _, r := range reservationRoutes() {
		assert.Equal(t, http.StatusNotFound, serve(t, config.ModeEdge, jwtService, "", r.method, r.path, "{}"), "%s %s", r.method, r.path)
	}
}

func TestRegisterReservationRoutes_PresentInTheCloud(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	for _, r := range reservationRoutes() {
		assert.Equal(t, http.StatusUnauthorized, serve(t, config.ModeCloud, jwtService, "", r.method, r.path, "{}"), "%s %s", r.method, r.path)
	}
}

func TestReservationRoutes_RefuseAUserWithoutThePermission(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	accountant := tokenFor(t, jwtService, permissions.RoleAccountant)
	for _, r := range reservationRoutes() {
		assert.Equal(t, http.StatusForbidden, serve(t, config.ModeCloud, jwtService, accountant, r.method, r.path, "{}"), "%s %s", r.method, r.path)
	}
}

func TestReservationRoutes_TheAgentCannotDoStaffWork(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	agent := tokenFor(t, jwtService, permissions.RoleAgent)
	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/reservations/" + testReservationID + "/confirm", "{}"},
		{http.MethodPost, "/api/reservations/" + testReservationID + "/decline", "{}"},
		{http.MethodPost, "/api/reservations/" + testReservationID + "/no-show", "{}"},
		{http.MethodGet, "/api/feedback", ""},
		// A confirmed booking is staff's to enter.
		{http.MethodPost, "/api/reservations", `{"source":"phone","name":"Ana","party_size":2,"starts_at":"2030-01-01T20:00:00-05:00","customer_phone":"+573005551234"}`},
		{http.MethodPatch, "/api/organization/settings", `{"feedback":{"review_link_enabled":true}}`},
	}
	for _, tc := range cases {
		assert.Equal(t, http.StatusForbidden, serve(t, config.ModeCloud, jwtService, agent, tc.method, tc.path, tc.body), "%s %s", tc.method, tc.path)
	}
}
