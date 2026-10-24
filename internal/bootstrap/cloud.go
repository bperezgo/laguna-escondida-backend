package bootstrap

import (
	"errors"
	"log/slog"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/cron"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/stockmovement"
)

// NewCloud assembles the cloud: the shared core plus the back office (accounting and catalog and
// user writes), e-invoicing, crons, the sync API, telemetry ingest, organization admin,
// delivery zones and stock authoring.
func NewCloud(cfg *config.Config, infra Infra) (*App, error) {
	if cfg.AppMode != config.ModeCloud {
		return nil, errors.New("NewCloud needs a cloud config")
	}
	if infra.ElectronicInvoice == nil {
		return nil, errors.New("the cloud needs an electronic invoice client")
	}
	logger := infra.Logger
	logger.Info("Running in CLOUD mode", slog.String("app_mode", string(cfg.AppMode)))

	// The cloud owns on-hand and the restaurant learns it back through the daily pull, so a
	// cloud-origin stock op would never be delivered to anyone.
	c, err := buildCore(cfg, infra, ModeOptions{
		PendingInvoiceInitialStatus: dto.PendingInvoiceStatusPending,
		StockMovementEmitter:        stockmovement.NewNoopEmitter(),
		StockAuthoring:              true,
		DeliveryEnabled:             true,
		EInvoicingEnabled:           true,
	})
	if err != nil {
		return nil, err
	}
	db := infra.DB

	// Entity appliers turn a received op's payload snapshot into a local upsert (or a
	// soft-delete for a tombstone). The sync service dispatches on op.EntityType and
	// fails closed for any entity type without a registered applier.
	syncAppliers := map[dto.SyncEntityType]ports.SyncApplier{
		dto.SyncEntityOpenBill:       repository.NewOpenBillSyncApplier(db),
		dto.SyncEntityPurchaseEntry:  repository.NewPurchaseEntrySyncApplier(db),
		dto.SyncEntityBill:           repository.NewBillSyncApplier(db),
		dto.SyncEntityPendingInvoice: repository.NewPendingInvoiceSyncApplier(db),
		// Stock stays registered although it no longer writes an amount: an edge that
		// predates this change still emits snapshot ops, and an unregistered entity type
		// fails the whole push — which would stall that edge's outbox behind an op nobody
		// can apply. It keeps only the tombstone path (see StockSyncApplier).
		dto.SyncEntityStock:         repository.NewStockSyncApplier(db),
		dto.SyncEntityHistoricStock: repository.NewHistoricStockSyncApplier(db),
	}
	syncService := service.NewSyncService(c.unitOfWork, c.syncInboxRepo, syncAppliers, logger)
	syncReferenceService := service.NewSyncReferenceService(c.syncReferenceRepo, logger)

	electronicInvoiceClient := infra.ElectronicInvoice
	invoiceService := service.NewInvoiceService(
		electronicInvoiceClient, c.productRepo, c.billRepo, c.pendingInvoiceRepo, c.unitOfWork, infra.Storage,
	)
	supportDocRepo := repository.NewSupportDocumentRepository(db, electronicInvoiceClient)
	supportDocService := service.NewSupportDocumentService(electronicInvoiceClient, supportDocRepo, infra.Storage)
	invoiceSubmissionService := service.NewInvoiceSubmissionService(
		c.pendingInvoiceRepo, c.billRepo, electronicInvoiceClient, c.unitOfWork, c.syncOutboxRepo,
		c.einvoicingSettingsRepo, c.syncIdentity, logger,
	)

	c.app.schedulers = append(c.app.schedulers, namedScheduler{
		name: "cron scheduler",
		build: func() (scheduler, error) {
			return cron.NewScheduler(
				c.organizationRunner, invoiceService, supportDocService, invoiceSubmissionService, c.stockReconciliationService,
				cfg.InvoiceURLCron, cfg.SupportDocumentURLCron, cfg.InvoiceSubmitCron, cfg.StockReconcileCron,
				logger,
			)
		},
	})

	documentURLMaintenance := service.NewDocumentURLMaintenanceService(c.organizationRunner, invoiceService, supportDocService)

	var telemetryHandler *handler.TelemetryHandler
	if !cfg.TelemetryIngestEnabled() {
		logger.Info("Telemetry ingest disabled: set TELEMETRY_FORWARD_URL to the cloud Alloy OTLP/HTTP receiver to enable it")
	} else {
		telemetryForwarder := httpclient.NewTelemetryForwardClient(infra.HTTPClient, cfg.TelemetryForwardURL)
		telemetryIngestService := service.NewTelemetryIngestService(telemetryForwarder, cfg.TelemetryTenantID)
		telemetryHandler = handler.NewTelemetryHandler(telemetryIngestService)
		logger.Info("Telemetry ingest enabled", slog.String("collector", cfg.TelemetryForwardURL))
	}

	purchaseEntryRepo := repository.NewPurchaseEntryRepository(db)
	expenseRepo := repository.NewExpenseRepository(db)
	purchaseEntryService := service.NewPurchaseEntryService(
		purchaseEntryRepo, c.supplierRepo, c.supplierCatalogRepo, c.productRepo, infra.Storage, c.publisher,
		c.unitOfWork, c.syncOutboxRepo, c.syncIdentity, logger,
	)
	expenseService := service.NewExpenseService(repository.NewExpenseCategoryRepository(db), expenseRepo, c.supplierRepo, infra.Storage)
	registerCloudBackOfficeRoutes(c.router, cfg, c.jwtService, backOfficeHandlers{
		product:           c.handlers.product,
		productIngredient: c.handlers.productIngredient,
		user:              c.handlers.user,
		supplier:          handler.NewSupplierHandler(service.NewSupplierService(c.supplierRepo, c.supplierCatalogRepo, c.productRepo)),
		purchaseEntry:     handler.NewPurchaseEntryHandler(purchaseEntryService),
		expense:           handler.NewExpenseHandler(expenseService),
		financial:         handler.NewFinancialHandler(service.NewFinancialService(c.billRepo, expenseRepo, purchaseEntryRepo)),
	})

	registerCloudRoutes(c.router, cfg, c.jwtService, cloudHandlers{
		invoice:         handler.NewInvoiceHandler(invoiceService, documentURLMaintenance),
		supportDocument: handler.NewSupportDocumentHandler(supportDocService, documentURLMaintenance),
		sync:            handler.NewSyncHandler(syncService, syncReferenceService),
		telemetry:       telemetryHandler,
		organization:    handler.NewOrganizationHandler(c.organizationService),
		deliveryZone:    handler.NewDeliveryZoneHandler(service.NewDeliveryZoneService(c.deliveryZoneRepo)),
		nodes:           service.NewNodeService(repository.NewNodeRepository(db)),
	})

	return c.app, nil
}
