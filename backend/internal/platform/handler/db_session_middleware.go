package handler

import (
	"strings"

	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DBSessionMiddleware gives each request one pinned connection carrying app.org_id. The org
// is the one authentication puts in the context, read when the request first touches the
// database; a request without one gets no session. SSE streams are skipped so a long-lived
// stream never holds a pool connection.
func DBSessionMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/sse/") {
			c.Next()
			return
		}
		ctx, release := postgres.WithSession(c.Request.Context(), db)
		defer release()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
