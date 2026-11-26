package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func signupRouter(mode config.Mode, opts signupRouteOptions) *gin.Engine {
	router := fullRouter(mode, service.NewJWTService("test-secret", tenant.AnyOrganization()))
	if mode == config.ModeCloud {
		registerSignupRoutes(router, &handler.SignupHandler{}, opts)
	}
	return router
}

func post(router *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSignupRoutes_Registration(t *testing.T) {
	cases := []struct {
		name         string
		mode         config.Mode
		opts         signupRouteOptions
		googleSignIn int
		signup       int
	}{
		{"edge never serves them", config.ModeEdge, signupRouteOptions{googleEnabled: true, signupEnabled: true}, http.StatusNotFound, http.StatusNotFound},
		{"cloud without a client id", config.ModeCloud, signupRouteOptions{signupEnabled: true}, http.StatusNotFound, http.StatusNotFound},
		{"cloud with signup off", config.ModeCloud, signupRouteOptions{googleEnabled: true}, http.StatusBadRequest, http.StatusNotFound},
		{"cloud with signup on", config.ModeCloud, signupRouteOptions{googleEnabled: true, signupEnabled: true}, http.StatusBadRequest, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := signupRouter(tc.mode, tc.opts)
			assert.Equal(t, tc.googleSignIn, post(router, "/api/auth/google").Code)
			assert.Equal(t, tc.signup, post(router, "/api/signup/google").Code)
			assert.Equal(t, http.StatusNotFound, post(router, "/api/signup").Code, "there is no password signup")
		})
	}
}

func TestSignupRoutes_RateLimited(t *testing.T) {
	router := signupRouter(config.ModeCloud, signupRouteOptions{googleEnabled: true, signupEnabled: true})
	for i := 0; i < signupsPerAddress; i++ {
		assert.NotEqual(t, http.StatusTooManyRequests, post(router, "/api/signup/google").Code)
	}
	rec := post(router, "/api/signup/google")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	for i := 0; i < googleSignInsPerAddress; i++ {
		post(router, "/api/auth/google")
	}
	assert.Equal(t, http.StatusTooManyRequests, post(router, "/api/auth/google").Code)
}

func TestInvitationRoutes_CloudOnlyAndRateLimited(t *testing.T) {
	jwt := service.NewJWTService("test-secret", tenant.AnyOrganization())
	edge := fullRouter(config.ModeEdge, jwt)
	for _, path := range []string{"/api/invitations/tok/accept", "/api/admin/invitations"} {
		assert.Equal(t, http.StatusNotFound, post(edge, path).Code, path)
	}

	cloud := fullRouter(config.ModeCloud, jwt)
	registerInvitationRoutes(cloud, jwt, &handler.InvitationHandler{})
	assert.Equal(t, http.StatusUnauthorized, post(cloud, "/api/admin/invitations").Code)
	for i := 0; i < invitationLookupsPerAddress; i++ {
		post(cloud, "/api/invitations/tok/accept")
	}
	rec := post(cloud, "/api/invitations/tok/accept")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestEnrollmentCodeRoutes_CloudOnlyAndRateLimited(t *testing.T) {
	jwt := service.NewJWTService("test-secret", tenant.AnyOrganization())
	edge := fullRouter(config.ModeEdge, jwt)
	assert.Equal(t, http.StatusNotFound, post(edge, "/api/edge/enroll-with-code").Code)
	assert.Equal(t, http.StatusNotFound, post(edge, "/api/edge/enrollment-codes").Code)

	cloud := fullRouter(config.ModeCloud, jwt)
	registerEnrollmentCodeRoutes(cloud, jwt, &handler.EnrollmentCodeHandler{})
	for i := 0; i < codeEnrollmentsPerAddress; i++ {
		post(cloud, "/api/edge/enroll-with-code")
	}
	rec := post(cloud, "/api/edge/enroll-with-code")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "past the limit no code is checked")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
