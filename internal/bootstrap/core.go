package bootstrap

import (
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/sse"
	"laguna-escondida/backend/internal/platform/syncstatus"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// ModeOptions are the choices that differ between modes; only cloud.go and edge.go fill them in.
type ModeOptions struct {
	PendingInvoiceInitialStatus dto.PendingInvoiceStatus
	StockMovementEmitter        ports.StockMovementEmitter
	StockAuthoring              bool
}

// core is what both modes share: the shared routes are already registered on router.
type core struct {
	router     *gin.Engine
	jwtService *service.JWTService
	unitOfWork ports.UnitOfWork

	productRepo            ports.ProductRepository
	openBillRepo           ports.OpenBillRepository
	billRepo               ports.BillRepository
	pendingInvoiceRepo     ports.PendingInvoiceRepository
	syncOutboxRepo         ports.SyncOutboxRepository
	syncInboxRepo          ports.SyncInboxRepository
	syncStateRepo          ports.SyncStateRepository
	syncReferenceRepo      *repository.SyncReferenceRepository
	deliveryZoneRepo       ports.DeliveryZoneRepository
	einvoicingSettingsRepo ports.EInvoicingSettingsRepository

	syncIdentity               dto.SyncIdentity
	stockReconciliationService *service.StockReconciliationService
	organizationService        *service.OrganizationService
	organizationRunner         *service.OrganizationRunner
	syncStatusTracker          *syncstatus.Tracker

	app *App
}

func buildCore(cfg *config.Config, infra Infra, opts ModeOptions) (*core, error) {
	db := infra.DB
	logger := infra.Logger

	productRepo := repository.NewProductRepository(db)
	openBillRepo := repository.NewOpenBillRepository(db)
	deliveryZoneRepo := repository.NewDeliveryZoneRepository(db)
	stockRepo := repository.NewStockRepository(db)
	stockReconciliationRepo := repository.NewStockReconciliationRepository(db)
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	userRoleRepo := repository.NewUserRoleRepository(db)
	billOwnerRepo := repository.NewBillOwnerRepository(db)
	supplierRepo := repository.NewSupplierRepository(db)
	supplierCatalogRepo := repository.NewSupplierCatalogRepository(db)
	purchaseEntryRepo := repository.NewPurchaseEntryRepository(db)
	expenseCategoryRepo := repository.NewExpenseCategoryRepository(db)
	expenseRepo := repository.NewExpenseRepository(db)
	productIngredientRepo := repository.NewProductIngredientRepository(db)
	billRepo := repository.NewBillRepository(db)
	pendingInvoiceRepo := repository.NewPendingInvoiceRepository(db)

	sseHub := sse.NewHub()
	openBillProductHub := sse.NewOpenBillProductHub(logger)

	watermillLogger := watermill.NewSlogLogger(logger)
	publisher := eventbus.NewDetachedEventBus(infra.EventBus, postgres.Detach)

	jwtService := service.NewJWTService(cfg.JWTSecret, cfg.OrganizationID)

	unitOfWork := postgres.NewUnitOfWork(db)
	syncOutboxRepo := repository.NewSyncOutboxRepository(db)
	syncInboxRepo := repository.NewSyncInboxRepository(db)
	syncStateRepo := repository.NewSyncStateRepository(db)
	// Reference repo is both sides of pull: the cloud reads changed rows, the edge upserts.
	syncReferenceRepo := repository.NewSyncReferenceRepository(db)
	// This install's sync identity, built once and injected as a unit into every
	// sync-participating component (order/purchase outbox writers + push/pull loops).
	syncIdentity := dto.SyncIdentity{NodeID: cfg.NodeID, CloudNodeID: cfg.CloudNodeID}
	// Edge status: pending-ops/lag come from this node's unsynced outbox (a domain use
	// case); connectivity comes from the sync scheduler via the in-memory tracker below.
	edgeStatusService := service.NewEdgeStatusService(syncOutboxRepo, syncIdentity)
	orderService := service.NewOrderServiceWithSSE(
		logger,
		openBillRepo,
		productRepo,
		billRepo,
		pendingInvoiceRepo,
		opts.PendingInvoiceInitialStatus,
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
	stockService := service.NewStockService(stockRepo, productRepo, unitOfWork, opts.StockMovementEmitter)
	stockReconciliationService := service.NewStockReconciliationService(
		stockReconciliationRepo, stockRepo, unitOfWork, opts.StockMovementEmitter, logger,
	)
	organizationRepo := repository.NewOrganizationRepository(db)
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, organizationRepo, jwtService, unitOfWork, cfg.OrganizationID)
	einvoicingSettingsRepo := repository.NewEInvoicingSettingsRepository(db)
	organizationService := service.NewOrganizationService(organizationRepo, einvoicingSettingsRepo, unitOfWork)
	sessionOpener := postgres.NewSessionOpener(db)
	organizationRunner := service.NewOrganizationRunner(organizationRepo, sessionOpener)
	billOwnerService := service.NewBillOwnerService(billOwnerRepo)
	supplierService := service.NewSupplierService(supplierRepo, supplierCatalogRepo, productRepo)
	purchaseEntryService := service.NewPurchaseEntryService(purchaseEntryRepo, supplierRepo, supplierCatalogRepo, productRepo, infra.Storage, publisher, unitOfWork, syncOutboxRepo, syncIdentity, logger)
	expenseService := service.NewExpenseService(expenseCategoryRepo, expenseRepo, supplierRepo, infra.Storage)
	productIngredientService := service.NewProductIngredientService(productIngredientRepo, productRepo)
	financialService := service.NewFinancialService(billRepo, expenseRepo, purchaseEntryRepo)

	eventSubscriber, err := eventbus.NewGoChannelEventSubscriber(infra.EventBus.PubSub(), watermillLogger)
	if err != nil {
		return nil, fmt.Errorf("create event subscriber: %w", err)
	}
	if err = subscribeKitchenFeed(eventSubscriber, orderService, sessionOpener, logger); err != nil {
		return nil, fmt.Errorf("subscribe kitchen feed handlers: %w", err)
	}

	productLockManager := eventbus.NewProductLockManager()
	stockEventHandler := service.NewStockEventHandler(
		stockRepo,
		productRepo,
		productIngredientRepo,
		productLockManager,
		unitOfWork,
		opts.StockMovementEmitter,
		logger,
	)
	stockSubscriber, err := eventbus.NewGoChannelEventSubscriber(infra.EventBus.PubSub(), watermillLogger)
	if err != nil {
		return nil, fmt.Errorf("create stock subscriber: %w", err)
	}
	if err = subscribeStockHandlers(stockSubscriber, stockEventHandler, sessionOpener, logger); err != nil {
		return nil, fmt.Errorf("subscribe stock handlers: %w", err)
	}

	orderHandler := handler.NewOrderHandler(orderService,
		service.NewOrderIdempotencyService(orderService, repository.NewIdempotencyKeyRepository(db)))
	// Online flips to offline if no successful pull lands within ~2.5 pull-cron intervals.
	// The edge scheduler records pull outcomes into this tracker; in the cloud it is never
	// written and the status handler short-circuits to online.
	syncStatusTracker := syncstatus.NewTracker(150 * time.Second)

	router := gin.Default()
	// Server spans + trace-context propagation; a no-op tracer when observability is disabled.
	router.Use(otelgin.Middleware(cfg.ServiceName))
	router.Use(handler.CORSMiddleware())
	router.Use(handler.LoggerMiddleware(logger))
	// Request counts/latencies for Prometheus (scraped by Alloy on :METRICS_PORT).
	router.Use(handler.MetricsMiddleware())
	router.Use(handler.DBSessionMiddleware(db))

	registerAPIRoutes(router, cfg, jwtService, apiHandlers{
		order:             orderHandler,
		product:           handler.NewProductHandler(productService),
		stock:             handler.NewStockHandler(stockService, stockReconciliationService, organizationRunner),
		user:              handler.NewUserHandler(userService, organizationService, logger),
		billOwner:         handler.NewBillOwnerHandler(billOwnerService),
		supplier:          handler.NewSupplierHandler(supplierService),
		purchaseEntry:     handler.NewPurchaseEntryHandler(purchaseEntryService),
		expense:           handler.NewExpenseHandler(expenseService),
		productIngredient: handler.NewProductIngredientHandler(productIngredientService),
		financial:         handler.NewFinancialHandler(financialService),
		sse:               handler.NewSSEHandler(sseHub, openBillProductHub, orderService, logger),
		edge:              handler.NewEdgeHandler(cfg, edgeStatusService, syncStatusTracker),
	}, opts.StockAuthoring)

	return &core{
		router:                     router,
		jwtService:                 jwtService,
		unitOfWork:                 unitOfWork,
		productRepo:                productRepo,
		openBillRepo:               openBillRepo,
		billRepo:                   billRepo,
		pendingInvoiceRepo:         pendingInvoiceRepo,
		syncOutboxRepo:             syncOutboxRepo,
		syncInboxRepo:              syncInboxRepo,
		syncStateRepo:              syncStateRepo,
		syncReferenceRepo:          syncReferenceRepo,
		deliveryZoneRepo:           deliveryZoneRepo,
		einvoicingSettingsRepo:     einvoicingSettingsRepo,
		syncIdentity:               syncIdentity,
		stockReconciliationService: stockReconciliationService,
		organizationService:        organizationService,
		organizationRunner:         organizationRunner,
		syncStatusTracker:          syncStatusTracker,
		app: &App{
			Router:      router,
			logger:      logger,
			metricsPort: cfg.MetricsPort,
			subscribers: []namedSubscriber{
				{name: "SSE event subscriber", sub: eventSubscriber},
				{name: "stock event subscriber", sub: stockSubscriber},
			},
			sseHubs:  []closableHub{sseHub, openBillProductHub},
			eventBus: infra.EventBus,
		},
	}, nil
}
