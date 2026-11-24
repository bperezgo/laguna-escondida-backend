package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestEdgeVersionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		floor  string
		header string
		want   int
	}{
		{"below floor", "v1.2.0", "v1.1.9", http.StatusUpgradeRequired},
		{"at floor", "v1.2.0", "v1.2.0", http.StatusOK},
		{"above floor", "v1.2.0", "v1.10.0", http.StatusOK},
		{"missing header", "v1.2.0", "", http.StatusOK},
		{"dev build", "v1.2.0", "dev", http.StatusOK},
		{"no floor", "", "v0.0.1", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/sync", EdgeVersionMiddleware(tt.floor), func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodGet, "/sync", nil)
			if tt.header != "" {
				req.Header.Set(EdgeVersionHeader, tt.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.want, w.Code)
			if tt.want == http.StatusUpgradeRequired {
				assert.JSONEq(t, `{"error":"edge version is below the minimum supported","min_version":"v1.2.0"}`, w.Body.String())
			}
		})
	}
}
