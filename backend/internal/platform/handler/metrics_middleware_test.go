package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A request through MetricsMiddleware must increment the request counter for the matched
// route template (not the raw path), keeping cardinality bounded.
func TestMetricsMiddleware_IncrementsCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MetricsMiddleware())
	router.GET("/api/metrics-test/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, "/api/metrics-test/:id", "200"))

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-test/123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, "/api/metrics-test/:id", "200"))
	assert.Equal(t, before+1, after)
}

// The /metrics endpoint served by promhttp must return 200 with a Prometheus text
// content-type so Alloy can scrape it.
func TestMetricsHandler_ServesPrometheus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	promhttp.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}
