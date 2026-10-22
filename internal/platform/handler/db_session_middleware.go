package handler

import (
	"strings"

	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DBSessionMiddleware gives each request one pinned connection carrying app.org_id.
// SSE streams are skipped so a long-lived stream never holds a pool connection.
func DBSessionMiddleware(db *gorm.DB, orgID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/sse/") {
			c.Next()
			return
		}
		ctx, release := postgres.WithSession(c.Request.Context(), db, orgID)
		defer release()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
