package bootstrap

import (
	"errors"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/cron"
	"laguna-escondida/backend/internal/platform/device"
	"laguna-escondida/backend/internal/platform/edgeupdate"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/stockmovement"
)

// NewEdge assembles the restaurant's box: the shared core plus appliance updates, ticket
// printing (when the printer is reachable) and the push/pull sync loops.
func NewEdge(cfg *config.Config, infra Infra) (*App, error) {
	if cfg.AppMode != config.ModeEdge {
		return nil, errors.New("NewEdge needs an edge config")
	}
	if cfg.CloudSyncURL == "" || cfg.NodeSyncKey == "" {
		return nil, errors.New("edge sync push disabled: set CLOUD_SYNC_URL and NODE_SYNC_KEY to enable it")
	}
	logger := infra.Logger
	logger.Info("Running in EDGE mode", slog.String("app_mode", string(cfg.AppMode)))

	c, err := buildCore(cfg, infra, ModeOptions{
		PendingInvoiceInitialStatus: dto.PendingInvoiceStatusPendingCloud,
		// Only the restaurant queues stock movements; the cloud owns on-hand.
		StockMovementEmitter: stockmovement.NewOutboxEmitter(repository.NewSyncOutboxRepository(infra.DB), cfg.NodeID),
		StockAuthoring:       false,
	})
	if err != nil {
		return nil, err
	}

	var edgeUpdateService *service.EdgeUpdateService
	if cfg.LagunaStateDir == "" {
		logger.Info("Owner-triggered updates disabled: set LAGUNA_STATE_DIR to enable them")
	} else {
		edgeUpdateService = service.NewEdgeUpdateService(edgeupdate.NewStateStore(cfg.LagunaStateDir, logger))
	}
	registerEdgeUpdateRoutes(c.router, c.jwtService, handler.NewEdgeUpdateHandler(edgeUpdateService))

	// A printer that fails to init (windows transport off windows, unreachable target) skips
	// the route, so the box still boots and the frontend falls back to browser printing.
	deviceCfg := device.Config{
		Transport: cfg.PrinterTransport,
		Target:    cfg.PrinterTarget,
		WidthMM:   cfg.PrinterWidthMM,
		Codepage:  cfg.PrinterCodepage,
		Cut:       cfg.PrinterCut,
	}
	if receiptPrinter, printerErr := device.NewReceiptPrinterFromConfig(deviceCfg); printerErr != nil {
		logger.Error("Ticket printing disabled: failed to init printer transport", slog.Any("error", printerErr))
	} else {
		printService := service.NewPrintService(c.openBillRepo, receiptPrinter, dto.TicketBusinessInfo{
			Name:        cfg.BusinessName,
			NIT:         cfg.BusinessNIT,
			Address:     cfg.BusinessAddress,
			Footer:      cfg.TicketFooter,
			LegalNotice: cfg.TicketLegalNotice,
		})
		registerEdgeRoutes(c.router, c.jwtService, handler.NewDeviceHandler(printService))
		logger.Info("Ticket printing enabled", slog.String("transport", deviceCfg.Transport))
	}

	syncPushClient := httpclient.NewSyncPushClient(infra.HTTPClient, cfg.CloudSyncURL, cfg.NodeSyncKey, cfg.NodeID)
	syncPushService := service.NewSyncPushService(
		c.unitOfWork, c.syncOutboxRepo, c.syncStateRepo, syncPushClient,
		c.syncIdentity, 0, logger,
	)
	syncPullClient := httpclient.NewSyncPullClient(infra.HTTPClient, cfg.CloudSyncURL, cfg.NodeSyncKey, cfg.NodeID)
	syncPullService := service.NewSyncPullService(
		c.unitOfWork, syncPullClient, c.syncReferenceRepo, c.syncStateRepo,
		c.syncIdentity, logger,
	)
	// Stock pulls on a channel and a cursor of its own: daily, because the numbers should be
	// stable through a service day, and separate so neither job's failure hides the other's progress.
	stockPullClient := httpclient.NewSyncStockPullClient(infra.HTTPClient, cfg.CloudSyncURL, cfg.NodeSyncKey, cfg.NodeID)
	stockPullService := service.NewStockPullService(
		c.unitOfWork, stockPullClient, c.syncReferenceRepo, c.syncStateRepo,
		c.syncIdentity, logger,
	)
	c.app.schedulers = append(c.app.schedulers, namedScheduler{
		name: "edge sync scheduler",
		build: func() (scheduler, error) {
			return cron.NewEdgeSyncScheduler(
				c.organizationRunner, cfg.OrganizationID, syncPushService, syncPullService, stockPullService, c.syncStatusTracker,
				cfg.SyncPushCron, cfg.SyncPullCron, cfg.StockPullCron, logger,
			)
		},
	})

	return c.app, nil
}
