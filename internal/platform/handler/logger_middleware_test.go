package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerMiddleware_Level(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		status int
		want   string
	}{
		{"regular success logs info", "/api/products", http.StatusOK, "INFO"},
		{"sync pull success logs debug", "/api/sync/pull", http.StatusOK, "DEBUG"},
		{"sync pull stock success logs debug", "/api/sync/pull/stock", http.StatusOK, "DEBUG"},
		{"sync push success logs debug", "/api/sync/push", http.StatusOK, "DEBUG"},
		{"edge status success logs debug", "/api/edge/status", http.StatusOK, "DEBUG"},
		{"health success logs debug", "/api/health", http.StatusOK, "DEBUG"},
		{"sync pull client error stays warn", "/api/sync/pull", http.StatusUnauthorized, "WARN"},
		{"sync pull server error stays error", "/api/sync/pull", http.StatusInternalServerError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			router := gin.New()
			router.Use(LoggerMiddleware(logger))
			router.Any(tt.path, func(c *gin.Context) { c.Status(tt.status) })

			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))

			var entry struct {
				Level string `json:"level"`
			}
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, tt.want, entry.Level)
		})
	}
}
