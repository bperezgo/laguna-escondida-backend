package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func deliveryZoneRoutesFor(mode config.Mode) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler.RegisterDeliveryZoneRoutes(router, &config.Config{AppMode: mode},
		handler.NewDeliveryZoneHandler(nil), service.NewJWTService("test-secret", ""))
	return router
}

func deliveryZoneRoutes() []struct{ method, path string } {
	return []struct{ method, path string }{
		{http.MethodGet, "/api/delivery-zones"},
		{http.MethodGet, "/api/delivery-zones/check?lat=4.6&lng=-74.08"},
		{http.MethodPost, "/api/delivery-zones"},
		{http.MethodPut, "/api/delivery-zones/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/delivery-zones/00000000-0000-0000-0000-000000000001"},
	}
}

// An unauthenticated request stops at 401 when the route exists and 404 when it does not.
func TestRegisterDeliveryZoneRoutes_AbsentOnTheEdge(t *testing.T) {
	router := deliveryZoneRoutesFor(config.ModeEdge)
	for _, r := range deliveryZoneRoutes() {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(r.method, r.path, nil))
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s", r.method, r.path)
	}
}

func TestRegisterDeliveryZoneRoutes_PresentInTheCloud(t *testing.T) {
	router := deliveryZoneRoutesFor(config.ModeCloud)
	for _, r := range deliveryZoneRoutes() {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(r.method, r.path, nil))
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s", r.method, r.path)
	}
}
