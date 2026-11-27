package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
)

func getPlans(mode config.Mode) int {
	router := fullRouter(mode, service.NewJWTService("test-secret", tenant.AnyOrganization()))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/plans", nil))
	return rec.Code
}

func TestPlansRoute_PublicInTheCloud(t *testing.T) {
	assert.Equal(t, http.StatusOK, getPlans(config.ModeCloud))
}

func TestPlansRoute_NotServedOnTheEdge(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, getPlans(config.ModeEdge))
}
