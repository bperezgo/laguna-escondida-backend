package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// stockRoutesFor builds a router with only the stock routes a given mode registers. The
// handler's collaborators are never reached: every write is behind JWT auth, so an
// unauthenticated request stops at 401 when the route exists and 404 when it does not.
func stockRoutesFor(mode config.Mode) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerStockRoutes(router, &config.Config{AppMode: mode, AdminAPIKey: "admin-key"},
		handler.NewStockHandler(nil, nil), service.NewJWTService("test-secret"))
	return router
}

func stockAuthoringRoutes() []struct{ method, path string } {
	return []struct{ method, path string }{
		{http.MethodPost, "/api/stock"},
		{http.MethodPut, "/api/stock/00000000-0000-0000-0000-000000000001/add-or-decrease"},
		{http.MethodDelete, "/api/stock/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/stock/bulk"},
		{http.MethodGet, "/api/stock/sync-freshness"},
	}
}

func TestRegisterStockRoutes_AuthoringIsAbsentOnTheEdge(t *testing.T) {
	router := stockRoutesFor(config.ModeEdge)

	for _, route := range stockAuthoringRoutes() {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestRegisterStockRoutes_AuthoringIsServedOnTheCloud(t *testing.T) {
	router := stockRoutesFor(config.ModeCloud)

	for _, route := range stockAuthoringRoutes() {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "registered, and guarded by JWT auth")
		})
	}
}

func TestRegisterStockRoutes_ReadIsServedInBothModes(t *testing.T) {
	for _, mode := range []config.Mode{config.ModeEdge, config.ModeCloud} {
		t.Run(string(mode), func(t *testing.T) {
			rec := httptest.NewRecorder()
			stockRoutesFor(mode).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stock", nil))
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}
