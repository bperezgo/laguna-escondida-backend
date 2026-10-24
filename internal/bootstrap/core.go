package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/sse"
	"laguna-escondida/backend/internal/platform/syncstatus"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// ModeOptions are the choices that differ between modes; only cloud.go and edge.go fill them in.
type ModeOptions struct {
	Tenancy                     tenant.Scope
	SyncIdentity                dto.SyncIdentity
	StatusHandler               func(*service.EdgeStatusService, *syncstatus.Tracker) *handler.EdgeHandler
	PendingInvoiceInitialStatus dto.PendingInvoiceStatus
	StockMovementEmitter        ports.StockMovementEmitter
	StockAuthoring              bool
	// DeliveryEnabled is false on the box: deliveries are taken in the cloud.
	DeliveryEnabled bool
	// EInvoicingEnabled is false on the box, which drops organization_einvoicing.
	EInvoicingEnabled bool
}

// noDeliveryZones is the box's zone finder: it never reads delivery_zones, which the box drops.
type noDeliveryZones struct{}

func (noDeliveryZones) ListActive(context.Context) ([]*dto.DeliveryZone, error) { return nil, nil }

// core is what both modes share: the shared routes are already registered on router.
type core struct {
	router     *gin.Engine
	handlers   apiHandlers
	publisher  pkgports.EventBus
	jwtService *service.JWTService
	unitOfWork ports.UnitOfWork

	productRepo            ports.ProductRepository
	supplierRepo           ports.SupplierRepository
	supplierCatalogRepo    ports.SupplierCatalogRepository
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

func buildCore(cfg *config.Shared, infra Infra, opts ModeOptions) (*core, error) {
	if opts.Tenancy == nil || opts.StatusHandler == nil {
		return nil, errors.New("buildCore needs a tenancy scope and a status handler")
	}
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
	productIngredientRepo := repository.NewProductIngredientRepository(db)
	billRepo := repository.NewBillRepository(db)
	pendingInvoiceRepo := repository.NewPendingInvoiceRepository(db)

	sseHub := sse.NewHub()
	openBillProductHub := sse.NewOpenBillProductHub(logger)

	watermillLogger := watermill.NewSlogLogger(logger)
	publisher := eventbus.NewDetachedEventBus(infra.EventBus, postgres.Detach)

	jwtService := service.NewJWTService(cfg.JWTSecret, opts.Tenancy)

	unitOfWork := postgres.NewUnitOfWork(db)
	syncOutboxRepo := repository.NewSyncOutboxRepository(db)
	syncInboxRepo := repository.NewSyncInboxRepository(db)
	syncStateRepo := repository.NewSyncStateRepository(db)
	// Reference repo is both sides of pull: the cloud reads changed rows, the edge upserts.
	syncReferenceRepo := repository.NewSyncReferenceRepository(db)
	syncIdentity := opts.SyncIdentity
	// Edge status: pending-ops/lag come from this node's unsynced outbox (a domain use
	// case); connectivity comes from the sync scheduler via the in-memory tracker below.
	edgeStatusService := service.NewEdgeStatusService(syncOutboxRepo, syncIdentity)
	var deliveryZoneFinder ports.DeliveryZoneFinder = noDeliveryZones{}
	if opts.DeliveryEnabled {
		deliveryZoneFinder = deliveryZoneRepo
	}
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
		deliveryZoneFinder,
		cfg.PhoneHomeRegion,
		service.WithDeliveryEnabled(opts.DeliveryEnabled),
	)
	// The edge only reads products; the supplier repos back the cloud's bulk create.
	productService := service.NewProductService(productRepo, supplierRepo, supplierCatalogRepo, unitOfWork)
	stockService := service.NewStockService(stockRepo, productRepo, unitOfWork, opts.StockMovementEmitter)
	stockReconciliationService := service.NewStockReconciliationService(
		stockReconciliationRepo, stockRepo, unitOfWork, opts.StockMovementEmitter, logger,
	)
	organizationRepo := repository.NewEdgeOrganizationRepository(db)
	if opts.EInvoicingEnabled {
		organizationRepo = repository.NewOrganizationRepository(db)
	}
	userService := service.NewUserService(userRepo, roleRepo, userRoleRepo, organizationRepo, jwtService, unitOfWork, opts.Tenancy)
	einvoicingSettingsRepo := repository.NewEInvoicingSettingsRepository(db)
	organizationService := service.NewOrganizationService(organizationRepo, einvoicingSettingsRepo, unitOfWork)
	sessionOpener := postgres.NewSessionOpener(db)
	organizationRunner := service.NewOrganizationRunner(organizationRepo, sessionOpener)
	billOwnerService := service.NewBillOwnerService(billOwnerRepo)
	productIngredientService := service.NewProductIngredientService(productIngredientRepo, productRepo)

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
	router.Use(otelgin.Middleware(cfg.Observability.ServiceName))
	router.Use(handler.CORSMiddleware())
	router.Use(handler.LoggerMiddleware(logger))
	// Request counts/latencies for Prometheus (scraped by Alloy on :METRICS_PORT).
	router.Use(handler.MetricsMiddleware())
	router.Use(handler.DBSessionMiddleware(db))

	handlers := apiHandlers{
		order:             orderHandler,
		product:           handler.NewProductHandler(productService),
		stock:             handler.NewStockHandler(stockService, stockReconciliationService, organizationRunner),
		user:              handler.NewUserHandler(userService, organizationService, logger),
		billOwner:         handler.NewBillOwnerHandler(billOwnerService),
		productIngredient: handler.NewProductIngredientHandler(productIngredientService),
		sse:               handler.NewSSEHandler(sseHub, openBillProductHub, orderService, logger),
		edge:              opts.StatusHandler(edgeStatusService, syncStatusTracker),
	}
	registerAPIRoutes(router, cfg.AdminAPIKey, jwtService, handlers, opts.StockAuthoring)

	return &core{
		router:                     router,
		handlers:                   handlers,
		publisher:                  publisher,
		supplierRepo:               supplierRepo,
		supplierCatalogRepo:        supplierCatalogRepo,
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
			metricsPort: cfg.Observability.MetricsPort,
			subscribers: []namedSubscriber{
				{name: "SSE event subscriber", sub: eventSubscriber},
				{name: "stock event subscriber", sub: stockSubscriber},
			},
			sseHubs:  []closableHub{sseHub, openBillProductHub},
			eventBus: infra.EventBus,
		},
	}, nil
}
