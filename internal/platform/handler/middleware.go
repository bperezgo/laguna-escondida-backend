package handler

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
)

const (
	IdempotencyKeyHeader     = "X-Idempotency-Key"
	IdempotentReplayedHeader = "Idempotent-Replayed"
)

// CORSMiddleware handles CORS headers and OPTIONS requests
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, "+IdempotencyKeyHeader)
		c.Header("Access-Control-Expose-Headers", IdempotentReplayedHeader)

		// Handle OPTIONS preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// AdminAPIKeyMiddleware validates the admin API key from the request header
func AdminAPIKeyMiddleware(adminAPIKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
			c.Abort()
			return
		}

		if apiKey != adminAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// NodeIDHeader names the calling edge node; its registered organization is the one it syncs for.
const NodeIDHeader = "X-Node-ID"

// NodeAuthMiddleware authenticates an edge node calling the cloud sync endpoints via the
// X-Node-Key header, then acts for the organization its X-Node-ID is registered to. It fails
// closed: no node key configured, or a node the cloud doesn't know, rejects the request.
func NodeAuthMiddleware(nodeSyncKey string, nodes *service.NodeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeKey := c.GetHeader("X-Node-Key")
		// Constant-time compare so a wrong key can't be recovered byte-by-byte via a timing
		// side-channel. ConstantTimeCompare also returns 0 on a length mismatch (empty header).
		if nodeSyncKey == "" || subtle.ConstantTimeCompare([]byte(nodeKey), []byte(nodeSyncKey)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid node key"})
			c.Abort()
			return
		}

		nodeID := c.GetHeader(NodeIDHeader)
		orgID, err := nodes.OrganizationOf(c.Request.Context(), nodeID)
		if err != nil {
			_ = c.Error(err)
			if errors.Is(err, domainError.ErrNodeNotFound) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown node"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve node"})
			}
			c.Abort()
			return
		}

		c.Set("node_id", nodeID)
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), orgID))
		c.Next()
	}
}

// PushNodeIDFromBody fills X-Node-ID from a push body's node_id when the edge sent no header.
// Edge boxes running the previous release name themselves only there; remove once every box
// sends the header.
func PushNodeIDFromBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(NodeIDHeader) != "" || c.Request.Body == nil {
			c.Next()
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		var push struct {
			NodeID string `json:"node_id"`
		}
		if json.Unmarshal(body, &push) == nil && push.NodeID != "" {
			c.Request.Header.Set(NodeIDHeader, push.NodeID)
		}
		c.Next()
	}
}

// invalidTokenBody is the one 401 for a bad JWT and for every refused API key, so the
// response can't tell which keys, users or organizations exist.
var invalidTokenBody = gin.H{"error": "Invalid or expired token"}

type bearerPrincipal struct {
	userID   string
	orgID    string
	username string
	roleIDs  []int
}

// JWTAuthMiddleware validates the bearer, a JWT or (where accepted) an organization API key,
// and optionally checks for required roles.
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
		var principal bearerPrincipal
		if strings.HasPrefix(token, dto.APIKeyPrefix) {
			p, ok := authenticateAPIKey(c, jwtService.APIKeys(), token)
			if !ok {
				return
			}
			principal = p
		} else {
			claims, err := jwtService.ValidateToken(token)
			if err != nil {
				c.JSON(http.StatusUnauthorized, invalidTokenBody)
				c.Abort()
				return
			}
			principal = bearerPrincipal{userID: claims.UserID, orgID: claims.OrgID, username: claims.Username, roleIDs: claims.RoleIDs}
		}

		if len(requiredRoles) > 0 {
			hasRequiredRole := false
			for _, requiredRole := range requiredRoles {
				if slices.Contains(principal.roleIDs, requiredRole) {
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

		c.Set("user_id", principal.userID)
		c.Set("org_id", principal.orgID)
		c.Set("username", principal.username)
		c.Set("role_ids", principal.roleIDs)
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), principal.orgID))
		c.Next()
	}
}

// authenticateAPIKey writes the response itself on failure. The reason goes to the access log
// through c.Error, never the key.
func authenticateAPIKey(c *gin.Context, keys *service.APIKeyService, key string) (bearerPrincipal, bool) {
	if keys == nil {
		_ = c.Error(&domainError.APIKeyRefusedError{Reason: "api keys are not accepted here"})
		c.JSON(http.StatusUnauthorized, invalidTokenBody)
		c.Abort()
		return bearerPrincipal{}, false
	}
	principal, err := keys.Authenticate(c.Request.Context(), key)
	if err != nil {
		_ = c.Error(err)
		if errors.Is(err, domainError.ErrCredentialRefused) {
			c.JSON(http.StatusUnauthorized, invalidTokenBody)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to authenticate"})
		}
		c.Abort()
		return bearerPrincipal{}, false
	}
	return bearerPrincipal{
		userID:   principal.UserID,
		orgID:    principal.OrganizationID,
		username: principal.Username,
		roleIDs:  principal.RoleIDs,
	}, true
}

// LoggerMiddleware logs HTTP requests using the slog logger in JSON format
func LoggerMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		statusCode := c.Writer.Status()
		duration := time.Since(start)
		success := statusCode >= 200 && statusCode < 400

		attrs := []slog.Attr{
			slog.String("method", method),
			slog.String("path", path),
			slog.Int("status_code", statusCode),
			slog.Duration("duration", duration),
			slog.Bool("success", success),
		}

		// Handlers attach the failure reason via c.Error(); surface it here so the single
		// access-log line carries the detail, correlated with the request's trace_id.
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		level := slog.LevelInfo
		switch {
		case statusCode >= 500:
			level = slog.LevelError
		case !success:
			level = slog.LevelWarn
		}

		// slog is context-native: passing the request context lets otelgin's active
		// trace_id/span_id attach to the OTLP log record for log↔trace correlation.
		logger.LogAttrs(c.Request.Context(), level, "HTTP Request", attrs...)
	}
}
