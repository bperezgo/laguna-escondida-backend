package bootstrap

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
)

// cloudHandlers back the cloud-only routes; telemetry is nil while the ingest is disabled.
type cloudHandlers struct {
	invoice         *handler.InvoiceHandler
	supportDocument *handler.SupportDocumentHandler
	sync            *handler.SyncHandler
	telemetry       *handler.TelemetryHandler
	organization    *handler.OrganizationHandler
	deliveryZone    *handler.DeliveryZoneHandler
	nodes           *service.NodeService
}

func registerCloudRoutes(router *gin.Engine, cfg *config.Config, jwtService *service.JWTService, h cloudHandlers) {
	// Admin routes (protected with admin API key)
	router.POST("/api/invoices/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), h.invoice.UpdateMissingDocumentURLsHandler)
	router.POST("/api/support-documents/update-missing-document-urls", handler.AdminAPIKeyMiddleware(cfg), h.supportDocument.UpdateMissingSupportDocumentURLsHandler)

	// Platform organization management: cross-org, so the admin API key until platform_admin exists.
	// No DELETE on purpose: an organization's invoices are legal records; retire it with status=suspended.
	organizations := router.Group("/api/platform/organizations", handler.AdminAPIKeyMiddleware(cfg))
	organizations.POST("", h.organization.CreateHandler)
	organizations.GET("", h.organization.ListHandler)
	organizations.GET("/:id", h.organization.GetHandler)
	organizations.PATCH("/:id", h.organization.UpdateHandler)
	organizations.PUT("/:id/einvoicing", h.organization.SetEInvoicingHandler)

	// Invoice routes
	router.POST("/api/invoices", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesCreate), h.invoice.CreateElectronicInvoiceHandler)
	router.GET("/api/invoices", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesRead), h.invoice.ListInvoicesHandler)
	router.GET("/api/invoices/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.InvoicesExport), h.invoice.ExportInvoicesCSVHandler)

	// Support Document routes
	router.POST("/api/support-documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsCreate), h.supportDocument.CreateSupportDocumentHandler)
	router.GET("/api/support-documents", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsRead), h.supportDocument.ListSupportDocumentsHandler)
	router.GET("/api/support-documents/export", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.SupportDocumentsExport), h.supportDocument.ExportSupportDocumentsCSVHandler)

	// Cloud sync aggregate: accept pushes from edge nodes and serve reference changes.
	// Each acts for the organization the calling node is registered to.
	nodeAuth := handler.NodeAuthMiddleware(cfg, h.nodes)
	router.POST("/api/sync/push", handler.PushNodeIDFromBody(), nodeAuth, h.sync.PushHandler)
	router.GET("/api/sync/pull", nodeAuth, h.sync.PullHandler)
	router.GET("/api/sync/pull/stock", nodeAuth, h.sync.PullStockHandler)

	if h.telemetry != nil {
		router.POST("/api/telemetry/v1/logs", nodeAuth, h.telemetry.IngestLogsHandler)
		router.POST("/api/telemetry/v1/traces", nodeAuth, h.telemetry.IngestTracesHandler)
		router.POST("/api/telemetry/v1/metrics", nodeAuth, h.telemetry.IngestMetricsHandler)
	}

	RegisterDeliveryZoneRoutes(router, h.deliveryZone, jwtService)
}

// RegisterDeliveryZoneRoutes is cloud-only: deliveries are taken in the cloud, and the edge's
// empty zone table keeps a delivery from being created there by accident.
func RegisterDeliveryZoneRoutes(router *gin.Engine, zoneHandler *handler.DeliveryZoneHandler, jwtService *service.JWTService) {
	zones := router.Group("/api/delivery-zones", handler.JWTAuthMiddleware(jwtService))
	zones.GET("", handler.RequirePermission(permissions.DeliveryZonesRead), zoneHandler.ListHandler)
	zones.GET("/check", handler.RequirePermission(permissions.OrdersCreate), zoneHandler.CheckHandler)
	zones.POST("", handler.RequirePermission(permissions.DeliveryZonesManage), zoneHandler.CreateHandler)
	zones.PUT("/:id", handler.RequirePermission(permissions.DeliveryZonesManage), zoneHandler.UpdateHandler)
	zones.DELETE("/:id", handler.RequirePermission(permissions.DeliveryZonesManage), zoneHandler.DeleteHandler)
}
