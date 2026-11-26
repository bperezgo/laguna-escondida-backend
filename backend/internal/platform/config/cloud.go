package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"laguna-escondida/backend/internal/platform/buildinfo"
)

// Cloud is the cloud binary's configuration. It has no organization or node identity: each
// request's organization comes from its caller.
type Cloud struct {
	Shared

	AdminAPIKey string
	Storage     Storage

	ElectronicInvoice ElectronicInvoice

	InvoiceURLCron         string
	SupportDocumentURLCron string
	// InvoiceSubmitCron drains the pending_invoices queue. Per-row backoff (next_attempt_at),
	// not this cron, throttles retries of a failing invoice.
	InvoiceSubmitCron string
	// StockReconcileCron is the on-hand invariant check. It corrects nothing, so daily is
	// enough to catch a silent corruption while the ledger stays intact.
	StockReconcileCron string

	// Relayed edge telemetry goes to the cloud Alloy sidecar, which owns the Grafana Cloud
	// credentials. Without the forward URL the ingest routes are not wired.
	TelemetryForwardURL string // e.g. "http://127.0.0.1:4319"
	TelemetryTenantID   string // tenant.id stamped on relayed telemetry; required with TelemetryForwardURL

	// MarketingEvents sends agent order and reservation events to marketing-app; off without its URL.
	MarketingEvents MarketingEvents
	// StaffNotifications is the key for encrypting channel secrets; notifications are off without it.
	StaffNotifications StaffNotifications
	// LeadAlerts announces new leads from the marketing site; off without a webhook.
	LeadAlerts LeadAlerts
	// Integrations is the key for encrypting organizations' provider credentials; off without it.
	Integrations Integrations
	// PaymentLinks is the Wompi API and link lifetime shared by every organization.
	PaymentLinks PaymentLinks
	// TrustedProxies are the peers allowed to set X-Forwarded-For; nil keeps the router's trust-all default.
	TrustedProxies []string
	// PublicAPIURL is the address boxes sync against; enrollment hands it to the box as CLOUD_SYNC_URL.
	// A domain the owner controls, since a box keeps the value it was given.
	PublicAPIURL string
	// MinEdgeVersion is the oldest edge release allowed to sync ("vX.Y.Z"); empty accepts any.
	MinEdgeVersion string
	// NodeRefusalRetryAfter is the Retry-After on every node refusal; within the edge's [1m, 24h] clamp.
	NodeRefusalRetryAfter time.Duration
	// FeedbackRequestCron asks for feedback the morning after visits; ReservationVisitedCron marks
	// confirmed reservations visited two hours after they start.
	FeedbackRequestCron    string
	ReservationVisitedCron string
	// EdgeReleasesRepo ("owner/name") is the public GitHub repository the installer download reads.
	EdgeReleasesRepo string
	Signup           Signup
}

type ElectronicInvoice struct {
	URL      string
	User     string
	Password string
}

// TelemetryIngestEnabled reports whether the cloud has somewhere to relay edge telemetry to.
func (c *Cloud) TelemetryIngestEnabled() bool {
	return c.TelemetryForwardURL != ""
}

func LoadCloud() (*Cloud, error) {
	shared, err := loadShared()
	if err != nil {
		return nil, err
	}

	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIKey == "" {
		return nil, errors.New("ADMIN_API_KEY is not set")
	}
	if shared.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is not set")
	}
	storage, err := loadStorage()
	if err != nil {
		return nil, err
	}

	invoice := ElectronicInvoice{
		URL:      os.Getenv("ELECTRONIC_INVOICE_URL"),
		User:     os.Getenv("ELECTRONIC_INVOICE_USER"),
		Password: os.Getenv("ELECTRONIC_INVOICE_PASSWORD"),
	}
	if invoice.URL == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_URL is not set")
	}
	if invoice.User == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_USER is not set")
	}
	if invoice.Password == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_PASSWORD is not set")
	}

	stockReconcileCron := envOr("STOCK_RECONCILE_CRON", "15 3 * * *")
	if err = validateCronExpression("STOCK_RECONCILE_CRON", stockReconcileCron); err != nil {
		return nil, err
	}

	telemetryForwardURL := os.Getenv("TELEMETRY_FORWARD_URL")
	telemetryTenantID := os.Getenv("TELEMETRY_TENANT_ID")
	if telemetryForwardURL != "" && telemetryTenantID == "" {
		return nil, errors.New("TELEMETRY_TENANT_ID is required when TELEMETRY_FORWARD_URL is set")
	}

	marketingEvents, err := loadMarketingEvents()
	if err != nil {
		return nil, err
	}
	staffNotifications, err := loadStaffNotifications()
	if err != nil {
		return nil, err
	}
	leadAlerts, err := loadLeadAlerts()
	if err != nil {
		return nil, err
	}
	integrations, err := loadIntegrations()
	if err != nil {
		return nil, err
	}
	paymentLinks, err := loadPaymentLinks()
	if err != nil {
		return nil, err
	}
	trustedProxies, err := loadTrustedProxies()
	if err != nil {
		return nil, err
	}
	feedbackRequestCron := envOr("FEEDBACK_REQUEST_CRON", "5 * * * *")
	if err = validateCronExpression("FEEDBACK_REQUEST_CRON", feedbackRequestCron); err != nil {
		return nil, err
	}
	reservationVisitedCron := envOr("RESERVATION_VISITED_CRON", "*/15 * * * *")
	if err = validateCronExpression("RESERVATION_VISITED_CRON", reservationVisitedCron); err != nil {
		return nil, err
	}

	minEdgeVersion := os.Getenv("MIN_EDGE_VERSION")
	if _, ok := buildinfo.ParseVersion(minEdgeVersion); minEdgeVersion != "" && !ok {
		return nil, fmt.Errorf("invalid MIN_EDGE_VERSION %q: want vMAJOR.MINOR.PATCH", minEdgeVersion)
	}

	publicAPIURL, err := loadPublicAPIURL()
	if err != nil {
		return nil, err
	}
	nodeRefusalRetryAfter, err := loadNodeRefusalRetryAfter()
	if err != nil {
		return nil, err
	}
	edgeReleasesRepo, err := loadEdgeReleasesRepo()
	if err != nil {
		return nil, err
	}
	signup, err := loadSignup()
	if err != nil {
		return nil, err
	}

	return &Cloud{
		NodeRefusalRetryAfter:  nodeRefusalRetryAfter,
		PublicAPIURL:           publicAPIURL,
		MinEdgeVersion:         minEdgeVersion,
		Shared:                 shared,
		AdminAPIKey:            adminAPIKey,
		Storage:                storage,
		ElectronicInvoice:      invoice,
		InvoiceURLCron:         envOr("INVOICE_URL_CRON", "0 * * * *"),
		SupportDocumentURLCron: envOr("SUPPORT_DOCUMENT_URL_CRON", "30 * * * *"),
		InvoiceSubmitCron:      envOr("INVOICE_SUBMIT_CRON", "* * * * *"),
		StockReconcileCron:     stockReconcileCron,
		TelemetryForwardURL:    telemetryForwardURL,
		TelemetryTenantID:      telemetryTenantID,
		MarketingEvents:        marketingEvents,
		StaffNotifications:     staffNotifications,
		LeadAlerts:             leadAlerts,
		Integrations:           integrations,
		PaymentLinks:           paymentLinks,
		TrustedProxies:         trustedProxies,
		FeedbackRequestCron:    feedbackRequestCron,
		ReservationVisitedCron: reservationVisitedCron,
		EdgeReleasesRepo:       edgeReleasesRepo,
		Signup:                 signup,
	}, nil
}

// The sync clients append /api/sync/..., so anything past the host would break them.
func loadPublicAPIURL() (string, error) {
	raw := os.Getenv("PUBLIC_API_URL")
	if raw == "" {
		return "", errors.New("PUBLIC_API_URL is not set")
	}
	parsed, err := url.Parse(raw)
	invalid := err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		strings.Trim(parsed.Path, "/") != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil
	if invalid {
		return "", fmt.Errorf("invalid PUBLIC_API_URL %q: want an absolute http(s) URL with a host and no path, query or fragment", raw)
	}
	return strings.TrimSuffix(raw, "/"), nil
}

const (
	defaultNodeRefusalRetryAfter = 6 * time.Hour
	minNodeRefusalRetryAfter     = time.Minute
	maxNodeRefusalRetryAfter     = 24 * time.Hour
)

func loadNodeRefusalRetryAfter() (time.Duration, error) {
	raw := os.Getenv("NODE_REFUSAL_RETRY_AFTER")
	if raw == "" {
		return defaultNodeRefusalRetryAfter, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < minNodeRefusalRetryAfter || d > maxNodeRefusalRetryAfter {
		return 0, fmt.Errorf("invalid NODE_REFUSAL_RETRY_AFTER %q: want a duration between 1m and 24h", raw)
	}
	return d, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
