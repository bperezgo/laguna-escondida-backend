package handler

import (
	"net/http"
	"slices"
	"strings"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles CORS headers and OPTIONS requests
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle OPTIONS preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// AdminAPIKeyMiddleware validates the admin API key from the request header
func AdminAPIKeyMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
			c.Abort()
			return
		}

		if apiKey != cfg.AdminAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// JWTAuthMiddleware validates the JWT token and optionally checks for required roles
func JWTAuthMiddleware(jwtService *service.JWTService, requiredRoles ...int) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format. Use: Bearer <token>"})
			c.Abort()
			return
		}

		token := tokenParts[1]
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		if len(requiredRoles) > 0 {
			hasRequiredRole := false
			for _, requiredRole := range requiredRoles {
				if slices.Contains(claims.RoleIDs, requiredRole) {
					hasRequiredRole = true
					break
				}
			}

			if !hasRequiredRole {
				c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
				c.Abort()
				return
			}
		}

		c.Set("username", claims.Username)
		c.Set("role_ids", claims.RoleIDs)
		c.Next()
	}
}
