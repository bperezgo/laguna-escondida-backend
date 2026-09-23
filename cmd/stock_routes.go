package main

import (
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/stockmovement"

	"github.com/gin-gonic/gin"
)

// registerStockRoutes wires the stock endpoints for a run mode. Reading on-hand is served on
// both nodes; authoring it is cloud-only. The dividing line is what the write is derived from:
// a sale decrement is a self-describing delta that folds correctly from anywhere, but creating,
// adjusting, deleting or counting stock is computed against the number a person read — and only
// the cloud's number is the real one. On the edge these paths are simply absent (404).
func registerStockRoutes(
	router *gin.Engine,
	cfg *config.Config,
	stockHandler *handler.StockHandler,
	jwtService *service.JWTService,
) {
	mode := cfg.AppMode

	router.GET("/api/stock",
		handler.JWTAuthMiddleware(jwtService),
		handler.RequirePermission(permissions.StockRead),
		stockHandler.GetAllStocksHandler)

	if mode != config.ModeCloud {
		return
	}

	// How stale the cloud's view of the restaurant is. Cloud-only because it describes what
	// the cloud has received, and it exists for the counting screen that now lives here.
	router.GET("/api/stock/sync-freshness",
		handler.JWTAuthMiddleware(jwtService),
		handler.RequirePermission(permissions.StockRead),
		stockHandler.GetStockSyncFreshnessHandler)

	router.POST("/api/stock",
		handler.JWTAuthMiddleware(jwtService),
		handler.RequirePermission(permissions.StockCreate),
		stockHandler.CreateStockHandler)
	router.PUT("/api/stock/:product_id/add-or-decrease",
		handler.JWTAuthMiddleware(jwtService),
		handler.RequirePermission(permissions.StockUpdate),
		stockHandler.AddOrDecreaseStockHandler)
	router.DELETE("/api/stock/:product_id",
		handler.JWTAuthMiddleware(jwtService),
		handler.RequirePermission(permissions.StockDelete),
		stockHandler.DeleteStockHandler)
	router.POST("/api/stock/bulk",
		handler.JWTAuthMiddleware(jwtService),
		handler.RequirePermission(permissions.StockCreate),
		stockHandler.BulkStockCreationOrUpdatingHandler)

	// Cutover only, and admin-gated rather than permission-gated: this is an operational
	// step run once when the cloud adopts on-hand, not something a stock manager does.
	router.POST("/api/stock/opening-balances",
		handler.AdminAPIKeyMiddleware(cfg),
		stockHandler.WriteStockOpeningBalancesHandler)
}

// stockMovementEmitterFor picks how this node replicates a stock movement. Only the
// restaurant queues one: the cloud owns on-hand and the restaurant learns it back through
// the daily pull, so a cloud-origin stock op would never be delivered to anyone.
func stockMovementEmitterFor(
	mode config.Mode,
	outboxRepo ports.SyncOutboxRepository,
	nodeID string,
) ports.StockMovementEmitter {
	if mode == config.ModeEdge {
		return stockmovement.NewOutboxEmitter(outboxRepo, nodeID)
	}
	return stockmovement.NewNoopEmitter()
}
