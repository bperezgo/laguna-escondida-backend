package handler

import (
	"net/http"

	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/gin-gonic/gin"
)

// RequirePermission middleware checks if user has the required permission
func RequirePermission(required permissions.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleIDs, exists := c.Get("role_ids")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		roles, ok := roleIDs.([]int)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role data"})
			c.Abort()
			return
		}

		if !permissions.HasPermission(roles, required) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient permissions",
				"required": string(required),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyPermission checks if user has at least one of the required permissions
func RequireAnyPermission(required ...permissions.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleIDs, exists := c.Get("role_ids")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		roles, ok := roleIDs.([]int)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role data"})
			c.Abort()
			return
		}

		if !permissions.HasAnyPermission(roles, required) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}
