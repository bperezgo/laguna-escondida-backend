package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/cron"
	"laguna-escondida/backend/internal/platform/device"
	"laguna-escondida/backend/internal/platform/edgeupdate"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/observability"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/sse"
	"laguna-escondida/backend/internal/platform/storage"
	"laguna-escondida/backend/internal/platform/syncstatus"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Observability bootstrap: traces + logs export over OTLP to the local Alloy sidecar.
	// No-op when OBSERVABILITY_ENABLED is false or no endpoint is set. Installed before the
	// logger so the slog OTLP bridge picks up the global logger provider.
	rootCtx := context.Background()
	providers, err := observability.Init(rootCtx, observability.InitConfig{
		Enabled:          cfg.ObservabilityEnabled,
		ServiceName:      cfg.ServiceName,
		ServiceVersion:   cfg.ServiceVersion,
		Environment:      cfg.Environment,
		OTLPEndpoint:     cfg.OTLPEndpoint,
		TraceSampleRatio: cfg.TraceSampleRatio,
		NodeID:           cfg.NodeID,
		OrganizationID:   cfg.OrganizationID,
		Mode:             string(cfg.AppMode),
	})
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}

	// The application logger: slog with JSON to stdout, plus an OTLP bridge to Loki when
	// observability is enabled. This single logger is passed to every component; batched
	// OTLP records are flushed by providers.Shutdown on exit (slog needs no Sync).
	logger := observability.NewSlogLogger(cfg.ObservabilityEnabled)

	// Database connection
	dsn := getDSN(getEnv("DB_USER", "postgres"), getEnv("DB_PASSWORD", "postgres"))

	// Migrations run as the schema owner; the app may connect as a pos_app member, which can't run DDL.
	log.Println("Running database migrations...")
	if err = runMigrations(getMigrationDSN()); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	db, err := repository.NewDatabase(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	httpClient := httpclient.NewClient(logger)

	// Initialize storage client
	storageClient, err := storage.NewS3Client(cfg)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}

	// Initialize repositories
	productRepo := repository.NewProductRepository(db.DB)
	openBillRepo := repository.NewOpenBillRepository(db.DB)
	deliveryZoneRepo := repository.NewDeliveryZoneRepository(db.DB)
	stockRepo := repository.NewStockRepository(db.DB)
	stockReconciliationRepo := repository.NewStockReconciliationRepository(db.DB)
	userRepo := repository.NewUserRepository(db.DB)
	roleRepo := repository.NewRoleRepository(db.DB)
	userRoleRepo := repository.NewUserRoleRepository(db.DB)
	billOwnerRepo := repository.NewBillOwnerRepository(db.DB)
	supplierRepo := repository.NewSupplierRepository(db.DB)
	supplierCatalogRepo := repository.NewSupplierCatalogRepository(db.DB)
	purchaseEntryRepo := repository.NewPurchaseEntryRepository(db.DB)
	expenseCategoryRepo := repository.NewExpenseCategoryRepository(db.DB)
	expenseRepo := repository.NewExpenseRepository(db.DB)
	productIngredientRepo := repository.NewProductIngredientRepository(db.DB)
	billRepo := repository.NewBillRepository(db.DB)
	pendingInvoiceRepo := repository.NewPendingInvoiceRepository(db.DB)

	// Initialize SSE Hubs
	sseHub := sse.NewHub()
	openBillProductHub := sse.NewOpenBillProductHub(logger)

	// Initialize Watermill Event Bus for pub/sub messaging
	watermillLogger := watermill.NewSlogLogger(logger)
	eventBusImpl := eventbus.NewGoChannelEventBus(watermillLogger)
	defer func() {
		if errEventBus := eventBusImpl.Close(); errEventBus != nil {
			log.Printf("Failed to close event bus: %v", errEventBus)
		}
	}()
	publisher := eventbus.NewDetachedEventBus(eventBusImpl, postgres.Detach)

	// Initialize JWT service
	jwtService := service.NewJWTService(cfg.JWTSecret)

	// Initialize services
	unitOfWork := postgres.NewUnitOfWork(db.DB)
	syncOutboxRepo := repository.NewSyncOutboxRepository(db.DB)
	syncInboxRepo := repository.NewSyncInboxRepository(db.DB)
	syncStateRepo := repository.NewSyncStateRepository(db.DB)
	// Reference repo is both sides of pull: the cloud reads changed rows, the edge upserts.
	syncReferenceRepo := repository.NewSyncReferenceRepository(db.DB)
	// Entity appliers turn a received op's payload snapshot into a local upsert (or a
	// soft-delete for a tombstone). The sync service dispatches on op.EntityType and
	// fails closed for any entity type without a registered applier.
	syncAppliers := map[dto.SyncEntityType]ports.SyncApplier{
		dto.SyncEntityOpenBill:       repository.NewOpenBillSyncApplier(db.DB),
		dto.SyncEntityPurchaseEntry:  repository.NewPurchaseEntrySyncApplier(db.DB),
		dto.SyncEntityBill:           repository.NewBillSyncApplier(db.DB),
		dto.SyncEntityPendingInvoice: repository.NewPendingInvoiceSyncApplier(db.DB),
		// Stock stays registered although it no longer writes an amount: an edge that
		// predates this change still emits snapshot ops, and an unregistered entity type
		// fails the whole push — which would stall that edge's outbox behind an op nobody
		// can apply. It keeps only the tombstone path (see StockSyncApplier).
		dto.SyncEntityStock:         repository.NewStockSyncApplier(db.DB),
		dto.SyncEntityHistoricStock: repository.NewHistoricStockSyncApplier(db.DB),
	}
	syncService := service.NewSyncService(unitOfWork, syncInboxRepo, syncAppliers, logger)
	syncReferenceService := service.NewSyncReferenceService(syncReferenceRepo, logger)
	// This install's sync identity, built once and injected as a unit into every
	// sync-participating component (order/purchase outbox writers + push/pull loops).
	syncIdentity := dto.SyncIdentity{NodeID: cfg.NodeID, CloudNodeID: cfg.CloudNodeID}
	// Edge status: pending-ops/lag come from this node's unsynced outbox (a domain use
	// case); connectivity comes from the sync scheduler via the in-memory tracker below.
	edgeStatusService := service.NewEdgeStatusService(syncOutboxRepo, syncIdentity)
	pendingInvoiceInitialStatus := dto.PendingInvoiceStatusPendingCloud
	if cfg.AppMode == config.ModeCloud {
		pendingInvoiceInitialStatus = dto.PendingInvoiceStatusPending
	}
	orderService := service.NewOrderServiceWithSSE(
		logger,
		openBillRepo,
		productRepo,
		billRepo,
		pendingInvoiceRepo,
		pendingInvoiceInitialStatus,
		billOwnerRepo,
		unitOfWork,
		publisher,
		userRepo,
		openBillProductHub,
		syncOutboxRepo,
		syncIdentity,
		productIngredientRepo,
		deliveryZoneRepo,
		cfg.PhoneHomeRegion,
	)
	productService := service.NewProductService(productRepo, supplierRepo, supplierCatalogRepo, unitOfWork)
	stockMovementEmitter := stockMovementEmitterFor(cfg.AppMode, syncOutboxRepo, syncIdentity.NodeID)
	stockService := service.NewStockService(stockRepo, productRepo, unitOfWork, stockMovementEmitter)
	stockReconciliationService := service.NewStockReconciliationService(
		stockReconciliationRepo, stockRepo, unitOfWork, stockMovementEmitter, logger,
	)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, jwtService, unitOfWork)
	billOwnerService := service.NewBillOwnerService(billOwnerRepo)
	supplierService := service.NewSupplierService(supplierRepo, supplierCatalogRepo, productRepo)
	purchaseEntryService := service.NewPurchaseEntryService(purchaseEntryRepo, supplierRepo, supplierCatalogRepo, productRepo, storageClient, publisher, unitOfWork, syncOutboxRepo, syncIdentity, logger, cfg.OrganizationID)
	expenseService := service.NewExpenseService(expenseCategoryRepo, expenseRepo, supplierRepo, storageClient, cfg.OrganizationID)
	productIngredientService := service.NewProductIngredientService(productIngredientRepo, productRepo)
	financialService := service.NewFinancialService(billRepo, expenseRepo, purchaseEntryRepo)
	deliveryZoneService := service.NewDeliveryZoneService(deliveryZoneRepo)

	// Initialize Event Subscriber
	eventSubscriber, err := eventbus.NewGoChannelEventSubscriber(eventBusImpl.PubSub(), watermillLogger)
	if err != nil {
		log.Fatalf("Failed to create event subscriber: %v", err)
	}
	defer func() {
		if errEventSubscriber := eventSubscriber.Close(); errEventSubscriber != nil {
			log.Printf("Failed to close event subscriber: %v", errEventSubscriber)
		}
	}()

	if err = subscribeKitchenFeed(eventSubscriber, orderService); err != nil {
		log.Fatalf("Failed to subscribe kitchen feed handlers: %v", err)
	}

	// Start event subscriber in background
	logger.Info("Starting SSE event subscriber for order events")
	go func() {
		logger.Info("SSE event subscriber goroutine started")
		if errEventSubscriber := eventSubscriber.Start(context.Background()); errEventSubscriber != nil {
			logger.Error("Event subscriber stopped", slog.Any("error", errEventSubscriber))
		} else {
			logger.Info("Event subscriber completed successfully")
		}
	}()

	// Initialize Stock Event Handler with Product Lock Manager
	productLockManager := eventbus.NewProductLockManager()
	stockEventHandler := service.NewStockEventHandler(
		stockRepo,
		productRepo,
		productIngredientRepo,
		productLockManager,
		unitOfWork,
		stockMovementEmitter,
		logger,
	)

	// Create separate subscriber for stock handlers
	stockSubscriber, err := eventbus.NewGoChannelEventSubscriber(eventBusImpl.PubSub(), watermillLogger)
	if err != nil {
		log.Fatalf("Failed to create stock subscriber: %v", err)
	}
	defer func() {
		if errStockSubscriber := stockSubscriber.Close(); errStockSubscriber != nil {
			log.Printf("Failed to close stock subscriber: %v", errStockSubscriber)
		}
	}()

	if err = subscribeStockHandlers(stockSubscriber, stockEventHandler); err != nil {
		log.Fatalf("Failed to subscribe stock handlers: %v", err)
	}

	// Start stock subscriber in background
	logger.Info("Starting stock event subscriber")
	go func() {
		logger.Info("Stock event subscriber goroutine started")
		if errStockSubscriber := stockSubscriber.Start(context.Background()); errStockSubscriber != nil {
			logger.Error("Stock subscriber stopped", slog.Any("error", errStockSubscriber))
		} else {
			logger.Info("Stock subscriber completed successfully")
		}
	}()

	// Initialize handlers
	orderHandler := handler.NewOrderHandler(orderService,
		service.NewOrderIdempotencyService(orderService, repository.NewIdempotencyKeyRepository(db.DB)))
	productHandler := handler.NewProductHandler(productService)
	stockHandler := handler.NewStockHandler(stockService, stockReconciliationService)
	userHandler := handler.NewUserHandler(userService, logger)
	billOwnerHandler := handler.NewBillOwnerHandler(billOwnerService)
	supplierHandler := handler.NewSupplierHandler(supplierService)
	purchaseEntryHandler := handler.NewPurchaseEntryHandler(purchaseEntryService)
	expenseHandler := handler.NewExpenseHandler(expenseService)
	productIngredientHandler := handler.NewProductIngredientHandler(productIngredientService)
	financialHandler := handler.NewFinancialHandler(financialService)
	deliveryZoneHandler := handler.NewDeliveryZoneHandler(deliveryZoneService)
	sseHandler := handler.NewSSEHandler(sseHub, openBillProductHub, orderService, logger)
	// Online flips to offline if no successful pull lands within ~2.5 pull-cron intervals.
	// In edge mode the scheduler (below) records pull outcomes into this tracker; in cloud
	// mode it is never written and the status handler short-circuits to online.
	syncStatusTracker := syncstatus.NewTracker(150 * time.Second)
	edgeHandler := handler.NewEdgeHandler(cfg, edgeStatusService, syncStatusTracker)

	// Setup routes
	router := gin.Default()

	// Server spans + trace-context propagation. otelgin puts the active span in
	// c.Request.Context(); handlers/services already take context first, so spans flow
	// through the existing call chain with no signature changes. No-op tracer when
	// observability is disabled.
	router.Use(otelgin.Middleware(cfg.ServiceName))

	// Apply CORS middleware globally
	router.Use(handler.CORSMiddleware())

	// Apply Logger middleware globally
	router.Use(handler.LoggerMiddleware(logger))

	// Record request counts/latencies for Prometheus (scraped by Alloy on :METRICS_PORT).
	router.Use(handler.MetricsMiddleware())

	router.Use(handler.DBSessionMiddleware(db.DB, cfg.OrganizationID))

	registerAPIRoutes(router, cfg, jwtService, apiHandlers{
		order:             orderHandler,
		product:           productHandler,
		stock:             stockHandler,
		user:              userHandler,
		billOwner:         billOwnerHandler,
		supplier:          supplierHandler,
		purchaseEntry:     purchaseEntryHandler,
		expense:           expenseHandler,
		productIngredient: productIngredientHandler,
		financial:         financialHandler,
		deliveryZone:      deliveryZoneHandler,
		sse:               sseHandler,
		edge:              edgeHandler,
	})

	// ---------------------------------------------------------------------
	// Mode-specific wiring seam (edge vs cloud).
	//
	// Single attachment point for components that only exist in one run mode.
	// Every shared dependency is already constructed above (cfg, logger, the
	// services, the unit of work, and the fully-routed gin engine), so later
	// steps wire here without restructuring main:
	//   - EDGE  (Step 8/9, §6): sync push/pull loops + pending-invoice retry.
	//   - CLOUD (Step 7):        POST /api/sync/push + NodeAuthMiddleware.
	// Both branches only log today so the seam is observable and lint-clean.
	switch cfg.AppMode {
	case config.ModeEdge:
		logger.Info("Running in EDGE mode", slog.String("app_mode", string(cfg.AppMode)))

		var edgeUpdateService *service.EdgeUpdateService
		if cfg.LagunaStateDir == "" {
			logger.Info("Owner-triggered updates disabled: set LAGUNA_STATE_DIR to enable them")
		} else {
			edgeUpdateService = service.NewEdgeUpdateService(edgeupdate.NewStateStore(cfg.LagunaStateDir, logger))
		}
		registerEdgeUpdateRoutes(router, jwtService, handler.NewEdgeUpdateHandler(edgeUpdateService))

		// Ticket printing (POST /api/device/print) — edge only. Build the printer
		// transport from PRINTER_* config; if it fails (e.g. windows transport on a
		// non-windows host, or an unreachable target), log and skip the route so the
		// node still boots and the frontend falls back to browser printing.
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
			printService := service.NewPrintService(openBillRepo, receiptPrinter, dto.TicketBusinessInfo{
				Name:        cfg.BusinessName,
				NIT:         cfg.BusinessNIT,
				Address:     cfg.BusinessAddress,
				Footer:      cfg.TicketFooter,
				LegalNotice: cfg.TicketLegalNotice,
			})
			deviceHandler := handler.NewDeviceHandler(printService)
			registerEdgeRoutes(router, jwtService, deviceHandler)
			logger.Info("Ticket printing enabled", slog.String("transport", deviceCfg.Transport))
		}

		if cfg.CloudSyncURL == "" || cfg.NodeSyncKey == "" {
			logger.Error("Edge sync push disabled: set CLOUD_SYNC_URL and NODE_SYNC_KEY to enable it")
			os.Exit(1)
		}
		syncPushClient := httpclient.NewSyncPushClient(httpClient, cfg.CloudSyncURL, cfg.NodeSyncKey)
		syncPushService := service.NewSyncPushService(
			unitOfWork, syncOutboxRepo, syncStateRepo, syncPushClient,
			syncIdentity, 0, logger,
		)
		syncPullClient := httpclient.NewSyncPullClient(httpClient, cfg.CloudSyncURL, cfg.NodeSyncKey)
		syncPullService := service.NewSyncPullService(
			unitOfWork, syncPullClient, syncReferenceRepo, syncStateRepo,
			syncIdentity, logger,
		)
		// Stock pulls on a channel and a cursor of its own: daily, because the numbers
		// should be stable through a service day, and separate so neither job's failure
		// hides the other's progress (design D4).
		stockPullClient := httpclient.NewSyncStockPullClient(httpClient, cfg.CloudSyncURL, cfg.NodeSyncKey)
		stockPullService := service.NewStockPullService(
			unitOfWork, stockPullClient, syncReferenceRepo, syncStateRepo,
			syncIdentity, logger,
		)
		edgeScheduler, edgeErr := cron.NewEdgeSyncScheduler(
			syncPushService, syncPullService, stockPullService, syncStatusTracker,
			cfg.SyncPushCron, cfg.SyncPullCron, cfg.StockPullCron, logger,
		)
		if edgeErr != nil {
			log.Fatalf("Failed to create edge sync scheduler: %v", edgeErr)
		}
		if edgeErr := edgeScheduler.Start(); edgeErr != nil {
			log.Fatalf("Failed to start edge sync scheduler: %v", edgeErr)
		}
		defer func() {
			if stopErr := edgeScheduler.Stop(); stopErr != nil {
				log.Printf("Failed to stop edge sync scheduler: %v", stopErr)
			}
		}()
	case config.ModeCloud:
		logger.Info("Running in CLOUD mode", slog.String("app_mode", string(cfg.AppMode)))

		electronicInvoiceClient := httpclient.NewElectronicInvoiceClient(cfg, httpClient)
		invoiceService := service.NewInvoiceService(
			electronicInvoiceClient, productRepo, billRepo, pendingInvoiceRepo, unitOfWork, storageClient, cfg.OrganizationID,
		)
		supportDocRepo := repository.NewSupportDocumentRepository(db.DB, electronicInvoiceClient, cfg.OrganizationID)
		supportDocService := service.NewSupportDocumentService(electronicInvoiceClient, supportDocRepo, storageClient, cfg.OrganizationID)
		invoiceSubmissionService := service.NewInvoiceSubmissionService(
			pendingInvoiceRepo, billRepo, electronicInvoiceClient, unitOfWork, syncOutboxRepo,
			repository.NewEInvoicingSettingsRepository(db.DB), syncIdentity, cfg.OrganizationID, logger,
		)

		cronScheduler, cronErr := cron.NewScheduler(
			invoiceService, supportDocService, invoiceSubmissionService, stockReconciliationService,
			cfg.InvoiceURLCron, cfg.SupportDocumentURLCron, cfg.InvoiceSubmitCron, cfg.StockReconcileCron,
			logger,
		)
		if cronErr != nil {
			log.Fatalf("Failed to create cron scheduler: %v", cronErr)
		}
		if cronErr = cronScheduler.Start(); cronErr != nil {
			log.Fatalf("Failed to start cron scheduler: %v", cronErr)
		}
		defer func() {
			if cronErr = cronScheduler.Stop(); cronErr != nil {
				log.Printf("Failed to stop cron scheduler: %v", cronErr)
			}
		}()

		invoiceHandler := handler.NewInvoiceHandler(invoiceService)
		supportDocHandler := handler.NewSupportDocumentHandler(supportDocService)

		syncHandler := handler.NewSyncHandler(syncService, syncReferenceService)

		var telemetryHandler *handler.TelemetryHandler
		if !cfg.TelemetryIngestEnabled() {
			logger.Info("Telemetry ingest disabled: set TELEMETRY_FORWARD_URL to the cloud Alloy OTLP/HTTP receiver to enable it")
		} else {
			telemetryForwarder := httpclient.NewTelemetryForwardClient(httpClient, cfg.TelemetryForwardURL)
			telemetryIngestService := service.NewTelemetryIngestService(telemetryForwarder, cfg.TelemetryTenantID, cfg.OrganizationID)
			telemetryHandler = handler.NewTelemetryHandler(telemetryIngestService)
			logger.Info("Telemetry ingest enabled", slog.String("collector", cfg.TelemetryForwardURL))
		}

		registerCloudRoutes(router, cfg, jwtService, cloudHandlers{
			invoice:         invoiceHandler,
			supportDocument: supportDocHandler,
			sync:            syncHandler,
			telemetry:       telemetryHandler,
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is not set")
	}

	// Metrics listener on a separate internal port so /metrics is never exposed via the
	// public Gin server / Cloudflare Tunnel. Alloy scrapes localhost:METRICS_PORT/metrics.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("Metrics server starting on port %s", cfg.MetricsPort)
		if errMetrics := metricsSrv.ListenAndServe(); errMetrics != nil && errMetrics != http.ErrServerClosed {
			logger.Error("metrics server failed", slog.Any("error", errMetrics))
		}
	}()

	// Create HTTP server
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Close SSE hubs to disconnect all SSE clients
	log.Println("Closing SSE connections...")
	sseHub.Close()
	openBillProductHub.Close()

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Stop the metrics listener, then flush batched spans/logs so nothing is dropped on
	// SIGTERM. Order: stop HTTP servers → flush providers → exit.
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Printf("Metrics server forced to shutdown: %v", err)
	}
	if err := providers.Shutdown(ctx); err != nil {
		log.Printf("observability shutdown: %v", err)
	}

	log.Println("Server exited")
}

func runMigrations(dsn string) (err error) {
	migrationURL := convertDSNToURL(dsn)

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to open embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, migrationURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
			err = fmt.Errorf("failed to close migrate instance: %w", sourceErr)
			err = fmt.Errorf("failed to close migrate instance: %w", databaseErr)
		}
	}()

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Migrations completed successfully")
	}

	return nil
}

func convertDSNToURL(dsn string) string {
	// Parse DSN components
	var host, port, user, password, dbname, sslmode string

	// Simple parser for key=value format
	parts := make(map[string]string)
	for _, part := range splitDSN(dsn) {
		if kv := splitKeyValue(part); len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	host = parts["host"]
	port = parts["port"]
	user = parts["user"]
	password = parts["password"]
	dbname = parts["dbname"]
	sslmode = parts["sslmode"]

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}

func splitDSN(dsn string) []string {
	var parts []string
	var current string
	for _, char := range dsn {
		if char == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func splitKeyValue(kv string) []string {
	for i, char := range kv {
		if char == '=' {
			return []string{kv[:i], kv[i+1:]}
		}
	}
	return []string{kv}
}

func getDSN(user, password string) string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	dbname := getEnv("DB_NAME", "laguna_escondida")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// TimeZone=UTC pins the connection's session timezone so timestamptz values
	// are read back and serialized in UTC (…Z) consistently, regardless of the
	// host's local timezone. (convertDSNToURL ignores this key, so the migrator
	// URL is unaffected.)
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		host, port, user, password, dbname, sslmode)
}

// getMigrationDSN falls back to the app credentials, so nothing changes until DB_MIGRATION_USER is set.
func getMigrationDSN() string {
	user := getEnv("DB_MIGRATION_USER", getEnv("DB_USER", "postgres"))
	password := getEnv("DB_MIGRATION_PASSWORD", getEnv("DB_PASSWORD", "postgres"))
	return getDSN(user, password)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
