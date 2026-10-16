package handler

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/gin-gonic/gin"
)

// RegisterOrderRoutes wires the order endpoints in both modes. Exported so acceptance tests
// exercise the real route→permission mapping.
func RegisterOrderRoutes(router *gin.Engine, orderHandler *OrderHandler, jwtService *service.JWTService) {
	router.POST("/api/orders", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersCreate), orderHandler.CreateOrderHandler)
	router.GET("/api/orders", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersRead), orderHandler.GetAllActiveOpenBillsHandler)
	router.GET("/api/orders/closed", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersRead), orderHandler.GetClosedOpenBillsTodayHandler)
	router.GET("/api/orders/closed/:id", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersRead), orderHandler.GetClosedOpenBillWithProductsHandler)
	router.GET("/api/orders/:id", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersRead), orderHandler.GetOpenBillWithProductsHandler)
	router.PUT("/api/orders/:id", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersUpdate), orderHandler.UpdateOrderHandler)
	router.DELETE("/api/orders/:id", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersDelete), orderHandler.DeleteOrderHandler)
	router.POST("/api/orders/pay-order", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersPay), orderHandler.PayOrderHandler)

	// Order product status routes
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/complete", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersCompleteProduct), orderHandler.CompleteOpenBillProductHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/uncomplete", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersCompleteProduct), orderHandler.UncompleteOpenBillProductHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/in-progress", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersCompleteProduct), orderHandler.SetOpenBillProductInProgressHandler)
	router.PATCH("/api/orders/:id/products/:open_bill_product_id/cancel", JWTAuthMiddleware(jwtService), RequirePermission(permissions.OrdersUpdate), orderHandler.CancelOpenBillProductHandler)
}

// RegisterDeliveryZoneRoutes is cloud-only: deliveries are taken in the cloud, and the edge's
// empty zone table keeps a delivery from being created there by accident.
func RegisterDeliveryZoneRoutes(router *gin.Engine, cfg *config.Config, zoneHandler *DeliveryZoneHandler, jwtService *service.JWTService) {
	if cfg.AppMode != config.ModeCloud {
		return
	}

	zones := router.Group("/api/delivery-zones", JWTAuthMiddleware(jwtService))
	zones.GET("", RequirePermission(permissions.DeliveryZonesRead), zoneHandler.ListHandler)
	zones.GET("/check", RequirePermission(permissions.OrdersCreate), zoneHandler.CheckHandler)
	zones.POST("", RequirePermission(permissions.DeliveryZonesManage), zoneHandler.CreateHandler)
	zones.PUT("/:id", RequirePermission(permissions.DeliveryZonesManage), zoneHandler.UpdateHandler)
	zones.DELETE("/:id", RequirePermission(permissions.DeliveryZonesManage), zoneHandler.DeleteHandler)
}
