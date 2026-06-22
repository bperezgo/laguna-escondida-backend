package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/platform/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNodeAuthMiddleware(t *testing.T) {
	cfg := &config.Config{NodeSyncKey: "secret-key"}

	tests := []struct {
		name       string
		setHeader  bool
		headerVal  string
		wantStatus int
	}{
		{name: "valid key", setHeader: true, headerVal: "secret-key", wantStatus: http.StatusOK},
		{name: "wrong key", setHeader: true, headerVal: "nope", wantStatus: http.StatusUnauthorized},
		{name: "missing key", setHeader: false, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/sync/push", NodeAuthMiddleware(cfg), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest(http.MethodPost, "/api/sync/push", nil)
			if tt.setHeader {
				req.Header.Set("X-Node-Key", tt.headerVal)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestNodeAuthMiddleware_FailsClosedWhenUnconfigured proves an install with no node
// key configured rejects everything rather than allowing unauthenticated access.
func TestNodeAuthMiddleware_FailsClosedWhenUnconfigured(t *testing.T) {
	cfg := &config.Config{NodeSyncKey: ""}
	router := gin.New()
	router.POST("/api/sync/push", NodeAuthMiddleware(cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/sync/push", nil)
	req.Header.Set("X-Node-Key", "anything")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
