package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/cache"
	"laguna-escondida/backend/internal/platform/cache/memory"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/cron"
	"laguna-escondida/backend/internal/platform/crypto"
	platformeventbus "laguna-escondida/backend/internal/platform/eventbus"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/sse"
	"laguna-escondida/backend/internal/platform/stockmovement"
	"laguna-escondida/backend/internal/platform/syncstatus"
)

// cloudOriginNodeID is the origin_node_id on the cloud's outbox rows. It is the value the cloud
// already wrote under its old NODE_ID, so existing and new rows share one origin.
const cloudOriginNodeID = "45598059-e12e-530c-86b6-758477c4423d"

// NewCloud assembles the cloud: the shared core plus the back office (accounting and catalog and
// user writes), e-invoicing, crons, the sync API, telemetry ingest, organization admin,
// delivery zones and stock authoring.
func NewCloud(cfg *config.Cloud, infra Infra) (*App, error) {
	if infra.ElectronicInvoice == nil {
		return nil, errors.New("the cloud needs an electronic invoice client")
	}
	logger := infra.Logger
	logger.Info("Running in CLOUD mode")

	// One replica, so a process-local cache; a shared one replaces memory.New without touching the decorator.
	settingsCache := memory.New()

	// Contacts and marketing events are cloud-only: the box never takes agent orders or reservations.
	db := infra.DB
	contactRepo := repository.NewContactRepository(db)
	reservationRepo := repository.NewReservationRepository(db)
	feedbackRepo := repository.NewFeedbackRepository(db)
	marketingOutbox := repository.NewMarketingEventOutboxRepository(db)
	cloudOrganizations := repository.NewOrganizationRepository(db)
	contactService := service.NewContactService(contactRepo, reservationRepo, feedbackRepo)

	// Integrations hold each organization's provider credentials; without the key none is saved or read.
	var integrationCipher ports.SecretCipher
	if cfg.Integrations.Enabled() {
		cipher, cipherErr := crypto.NewAESGCMCipher(cfg.Integrations.SecretsKey)
		if cipherErr != nil {
			return nil, cipherErr
		}
		integrationCipher = cipher
	} else {
		logger.Info("Organization integrations disabled: set INTEGRATION_SECRETS_KEY (base64, 32 bytes) to enable them; agent orders are refused and no marketing events are recorded")
	}
	integrations := service.NewOrganizationIntegrationService(repository.NewOrganizationIntegrationRepository(db),
		cloudOrganizations, integrationCipher, cfg.PaymentLinks.APIURL, logger, nil)
	// Each organization's signing secret is its marketing_app integration, read on every event.
	marketingSecrets := service.MarketingAppSecrets{Integrations: integrations}
	marketingEvents := service.NewMarketingEventService(marketingOutbox, cloudOrganizations, marketingSecrets, cfg.MarketingEvents.Enabled())
	logIgnoredMarketingEventsEnv(logger, cfg.MarketingEvents)

	// Payment links are cloud-only too: the box neither creates links nor receives Wompi's events.
	paymentLinkRepo := repository.NewPaymentLinkRepository(db)
	paymentTransactionRepo := repository.NewPaymentTransactionRepository(db)
	paymentLinks := service.NewPaymentLinkService(paymentLinkRepo, httpclient.NewWompiClient(infra.HTTPClient, cfg.PaymentLinks.APIURL),
		integrations, cloudOrganizations, cfg.PaymentLinks.TTL, logger, nil)

	// Durable events: without the key no staff-notification consumer is registered, so nothing is recorded.
	// The bridge forwards nothing yet; it is registered so the relay can carry in-memory consumers later.
	eventConsumers := []ports.EventConsumer{}
	bridge, err := platformeventbus.NewInMemoryBridgeConsumer(infra.EventBus, []string{})
	if err != nil {
		return nil, err
	}
	eventConsumers = append(eventConsumers, bridge)
	staffNotifications := cfg.StaffNotifications.Enabled()
	notificationRuleRepo := repository.NewNotificationRuleRepository(db)
	notificationChannelRepo := repository.NewNotificationChannelRepository(db)
	staffNotificationOutbox := repository.NewStaffNotificationOutboxRepository(db)
	if staffNotifications {
		eventConsumers = append(eventConsumers, service.NewStaffNotificationConsumer(
			notificationRuleRepo, staffNotificationOutbox, cloudOrganizations, repository.NewDeliveryZoneRepository(db), nil))
	} else {
		logger.Info("Staff notifications disabled: set NOTIFICATION_SECRETS_KEY (base64, 32 bytes) to enable them")
	}
	eventRegistry, err := eventconsumer.NewRegistry(eventConsumers...)
	if err != nil {
		return nil, err
	}
	domainEvents := repository.NewDomainEventRepository(db, eventRegistry)

	deliveryRepo := repository.NewDeliveryRepository(db)

	// The cloud owns on-hand and the restaurant learns it back through the daily pull, so a
	// cloud-origin stock op would never be delivered to anyone.
	c, err := buildCore(&cfg.Shared, infra, ModeOptions{
		Tenancy:      tenant.AnyOrganization(),
		SyncIdentity: dto.SyncIdentity{NodeID: cloudOriginNodeID},
		StatusHandler: func(*service.EdgeStatusService, *syncstatus.Tracker) *handler.EdgeHandler {
			return handler.NewCloudStatusHandler()
		},
		PendingInvoiceInitialStatus: dto.PendingInvoiceStatusPending,
		StockMovementEmitter:        stockmovement.NewNoopEmitter(),
		StockAuthoring:              true,
		DeliveryEnabled:             true,
		EInvoicingEnabled:           true,
		WrapOrganizationSettings: func(inner ports.OrganizationSettingsRepository) ports.OrganizationSettingsRepository {
			return cache.NewCachedOrganizationSettingsRepository(inner, settingsCache, logger)
		},
		OrderOptions: []service.OrderServiceOption{
			service.WithContactRecorder(contactService),
			service.WithMarketingEvents(marketingEvents),
			service.WithEventPublisher(domainEvents),
			service.WithPaymentLinks(paymentLinks),
			service.WithDeliveryTracking(deliveryRepo),
		},
	})
	if err != nil {
		return nil, err
	}
	if err = applyTrustedProxies(c.router, cfg.TrustedProxies); err != nil {
		return nil, err
	}

	// Entity appliers turn a received op's payload snapshot into a local upsert (or a
	// soft-delete for a tombstone). The sync service dispatches on op.EntityType and
	// fails closed for any entity type without a registered applier.
	syncAppliers := map[dto.SyncEntityType]ports.SyncApplier{
		// A payment applied from the box is where the cloud first learns a takeaway was paid.
		dto.SyncEntityOpenBill: service.NewMarketingPaymentSyncApplier(
			repository.NewOpenBillSyncApplier(db), repository.NewOrderOriginReader(db), marketingEvents),
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

	c.app.background = append(c.app.background,
		newBackgroundLoop(func(ctx context.Context) { settingsCache.Sweep(ctx, time.Minute) }),
		newBackgroundLoop(func(ctx context.Context) {
			logPaymentLinks(ctx, logger, cloudOrganizations, integrations, cfg.PaymentLinks)
		}))
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

	paymentWebhooks := service.NewPaymentWebhookService(cloudOrganizations, c.organizationRunner, integrations,
		paymentLinkRepo, paymentTransactionRepo, c.orderService, c.unitOfWork, logger)
	paymentExpiry := service.NewPaymentExpiryService(c.organizationRunner, paymentLinkRepo, paymentTransactionRepo,
		paymentLinks, c.orderService, logger, nil)
	c.app.schedulers = append(c.app.schedulers, namedScheduler{
		name: "payment expiry scheduler",
		build: func() (scheduler, error) {
			return cron.NewPaymentExpiryScheduler(paymentExpiry, logger)
		},
	})

	reservationHub := sse.NewReservationHub(logger)
	c.app.sseHubs = append(c.app.sseHubs, reservationHub)
	reservationService := service.NewReservationService(
		reservationRepo, contactService, c.unitOfWork, c.orgSettingsRepo, marketingEvents, reservationHub, cfg.PhoneHomeRegion, logger, nil,
		service.WithReservationEventPublisher(domainEvents),
	)
	feedbackService := service.NewFeedbackService(
		feedbackRepo, reservationRepo, c.unitOfWork, c.orgSettingsRepo, marketingEvents, cfg.PhoneHomeRegion, nil,
	)
	crmJobs := cron.CRMJobs{Reservations: reservationService, Feedback: feedbackService}
	if cfg.MarketingEvents.Enabled() {
		crmJobs.Delivery = service.NewMarketingEventDeliveryService(
			marketingOutbox, httpclient.NewMarketingEventsClient(infra.HTTPClient, cfg.MarketingEvents.URL),
			marketingSecrets, logger, nil,
		)
		logger.Info("Marketing events enabled", slog.String("url", cfg.MarketingEvents.URL))
	}
	c.app.schedulers = append(c.app.schedulers, namedScheduler{
		name: "CRM scheduler",
		build: func() (scheduler, error) {
			return cron.NewCRMScheduler(
				c.organizationRunner, cloudOrganizations, marketingSecrets, crmJobs,
				cfg.ReservationVisitedCron, cfg.FeedbackRequestCron, logger,
			)
		},
	})

	var notificationChannelService *service.NotificationChannelService
	var notificationRuleService *service.NotificationRuleService
	var staffDelivery *service.StaffNotificationDeliveryService
	if staffNotifications {
		cipher, cipherErr := crypto.NewAESGCMCipher(cfg.StaffNotifications.SecretsKey)
		if cipherErr != nil {
			return nil, cipherErr
		}
		senders := httpclient.NewStaffNotificationSenderRegistry(map[dto.NotificationChannelType]ports.StaffNotificationSender{
			dto.NotificationChannelDiscord: httpclient.NewDiscordWebhookClient(infra.HTTPClient),
		})
		notificationChannelService = service.NewNotificationChannelService(notificationChannelRepo, cipher, senders)
		notificationRuleService = service.NewNotificationRuleService(notificationRuleRepo, notificationChannelRepo)
		staffDelivery = service.NewStaffNotificationDeliveryService(staffNotificationOutbox, senders, cipher, logger, nil)
	}
	eventRelayService := service.NewEventRelayService(domainEvents, eventRegistry, c.unitOfWork, logger, nil)
	c.app.schedulers = append(c.app.schedulers, namedScheduler{
		name: "event relay scheduler",
		build: func() (scheduler, error) {
			relayScheduler, err := cron.NewEventRelayScheduler(c.organizationRunner, eventRelayService, logger)
			if err != nil {
				return nil, err
			}
			if staffDelivery != nil {
				relayScheduler.WithStaffNotifications(staffDelivery)
			}
			return relayScheduler, nil
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
	registerCloudBackOfficeRoutes(c.router, cfg.AdminAPIKey, c.jwtService, backOfficeHandlers{
		product:           c.handlers.product,
		productIngredient: c.handlers.productIngredient,
		plateOptionSlot:   c.handlers.plateOptionSlot,
		user:              c.handlers.user,
		supplier:          handler.NewSupplierHandler(service.NewSupplierService(c.supplierRepo, c.supplierCatalogRepo, c.productRepo)),
		purchaseEntry:     handler.NewPurchaseEntryHandler(purchaseEntryService),
		expense:           handler.NewExpenseHandler(expenseService),
		financial:         handler.NewFinancialHandler(service.NewFinancialService(c.billRepo, expenseRepo, purchaseEntryRepo)),
	})

	// API keys are a cloud credential: the edge's JWT service never accepts them.
	apiKeyService := service.NewAPIKeyService(
		repository.NewAPIKeyRepository(db), repository.NewOrganizationRepository(db),
		repository.NewUserRepository(db), repository.NewUserRoleRepository(db), logger,
	)
	c.jwtService.AcceptAPIKeys(apiKeyService)

	leadRepo := repository.NewLeadRepository(db)
	leadService := service.NewLeadService(leadRepo, cfg.LeadAlerts.Enabled(), logger, nil)
	if cfg.LeadAlerts.Enabled() {
		leadAlerts := service.NewLeadAlertService(
			leadRepo, httpclient.NewDiscordWebhookClient(infra.HTTPClient), cfg.LeadAlerts.DiscordWebhookURL, logger, nil)
		c.app.schedulers = append(c.app.schedulers, namedScheduler{
			name: "lead alert scheduler",
			build: func() (scheduler, error) {
				return cron.NewLeadAlertScheduler(leadAlerts, logger)
			},
		})
	} else {
		logger.Info("Lead alerts disabled: set LEAD_ALERT_DISCORD_WEBHOOK_URL to be told about new leads")
	}

	registerCloudRoutes(c.router, cfg.AdminAPIKey, cfg.NodeSyncKey, c.jwtService, cloudHandlers{
		invoice:         handler.NewInvoiceHandler(invoiceService, documentURLMaintenance),
		supportDocument: handler.NewSupportDocumentHandler(supportDocService, documentURLMaintenance),
		sync:            handler.NewSyncHandler(syncService, syncReferenceService),
		telemetry:       telemetryHandler,
		organization:    handler.NewOrganizationHandler(c.organizationService),
		lead:            handler.NewLeadHandler(leadService),
		apiKey:          handler.NewAPIKeyHandler(apiKeyService),
		integration:     handler.NewOrganizationIntegrationHandler(integrations),
		deliveryZone:    handler.NewDeliveryZoneHandler(service.NewDeliveryZoneService(c.deliveryZoneRepo)),
		orgSettings:     handler.NewOrganizationSettingsHandler(service.NewOrganizationSettingsService(c.orgSettingsRepo)),
		reservation:     handler.NewReservationHandler(reservationService),
		reservationLive: handler.NewReservationStreamHandler(reservationHub, logger),
		contact:         handler.NewContactHandler(contactService),
		feedback:        handler.NewFeedbackHandler(feedbackService),
		notifChannel:    handler.NewNotificationChannelHandler(notificationChannelService),
		notifRule:       handler.NewNotificationRuleHandler(notificationRuleService),
		paymentWebhook:  handler.NewPaymentWebhookHandler(paymentWebhooks, logger),
		delivery:        handler.NewDeliveryHandler(service.NewDeliveryService(deliveryRepo, c.unitOfWork, marketingEvents, nil)),
		nodes:           service.NewNodeService(repository.NewNodeRepository(db)),
	})

	return c.app, nil
}

func logIgnoredMarketingEventsEnv(logger *slog.Logger, settings config.MarketingEvents) {
	for _, name := range settings.IgnoredEnv {
		logger.Warn("Ignored: marketing event signing secrets are read from each organization's marketing_app integration, not the environment; unset it",
			slog.String("variable", name))
	}
}

// logPaymentLinks says at startup which organizations take agent orders: one without a usable
// Wompi integration refuses them. Informational only, so a failure to read is logged and startup goes on.
func logPaymentLinks(ctx context.Context, logger *slog.Logger, orgs ports.OrganizationRepository, links ports.PaymentLinkConfig, settings config.PaymentLinks) {
	for _, name := range settings.IgnoredEnv {
		logger.Warn("Ignored: Wompi keys are read from each organization's Wompi integration, not the environment; unset it",
			slog.String("variable", name))
	}
	list, err := orgs.List(ctx)
	if err != nil {
		logger.Warn("Could not list organizations to report payment links", slog.Any("error", err))
		return
	}
	for _, org := range list {
		if org.Status != dto.OrganizationStatusActive {
			continue
		}
		_, configured, err := links.For(ctx, org.ID)
		if err != nil {
			logger.Warn("Could not read the Wompi integration to report payment links", slog.String("organization", org.Slug), slog.Any("error", err))
			continue
		}
		if configured {
			logger.Info("Payment links configured", slog.String("organization", org.Slug), slog.String("wompi", settings.APIURL))
			continue
		}
		logger.Info("Payment links not configured: agent orders are refused; put the keys with PUT /api/platform/organizations/:id/integrations/wompi",
			slog.String("organization", org.Slug))
	}
}

// applyTrustedProxies limits who may set X-Forwarded-For. With none configured the router keeps
// its default of trusting every peer, which is what the per-address limit is weakest under.
func applyTrustedProxies(router *gin.Engine, proxies []string) error {
	if len(proxies) == 0 {
		return nil
	}
	if err := router.SetTrustedProxies(proxies); err != nil {
		return fmt.Errorf("set trusted proxies: %w", err)
	}
	return nil
}
