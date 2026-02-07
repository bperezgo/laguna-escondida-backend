package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SSEMiddleware removes write deadline for SSE endpoints to allow long-lived connections
func SSEMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is an SSE endpoint by path
		if strings.HasPrefix(c.Request.URL.Path, "/api/sse/") {
			// Set SSE headers early
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")

			// For SSE, we want to remove the write deadline entirely
			// This is done by setting the deadline to zero time using http.ResponseController
			// This is the proper way to handle timeouts for long-lived connections in Go 1.20+
			if rw, ok := c.Writer.(http.ResponseWriter); ok {
				rc := http.NewResponseController(rw)
				// Set write deadline to zero (no timeout)
				_ = rc.SetWriteDeadline(time.Time{})
			}
		}

		c.Next()
	}
}
