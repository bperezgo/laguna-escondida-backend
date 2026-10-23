package bootstrap

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
)

// registerEdgeRoutes wires ticket printing, only once the printer transport is up.
func registerEdgeRoutes(router *gin.Engine, jwtService *service.JWTService, device *handler.DeviceHandler) {
	router.POST("/api/device/print", handler.JWTAuthMiddleware(jwtService, permissions.RoleAdmin, permissions.RoleManager), device.PrintTicketHandler)
}

// registerEdgeUpdateRoutes wires owner-triggered appliance updates (edge only).
func registerEdgeUpdateRoutes(router *gin.Engine, jwtService *service.JWTService, update *handler.EdgeUpdateHandler) {
	edgeUpdate := router.Group("/api/edge/update", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.EdgeUpdate))
	edgeUpdate.GET("", update.GetUpdateHandler)
	edgeUpdate.POST("", update.RequestUpdateHandler)
}
