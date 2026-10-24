package config

import (
	"errors"
	"os"
)

// Cloud is the cloud binary's configuration. It has no organization or node identity: each
// request's organization comes from its caller.
type Cloud struct {
	Shared

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

	return &Cloud{
		Shared:                 shared,
		ElectronicInvoice:      invoice,
		InvoiceURLCron:         envOr("INVOICE_URL_CRON", "0 * * * *"),
		SupportDocumentURLCron: envOr("SUPPORT_DOCUMENT_URL_CRON", "30 * * * *"),
		InvoiceSubmitCron:      envOr("INVOICE_SUBMIT_CRON", "* * * * *"),
		StockReconcileCron:     stockReconcileCron,
		TelemetryForwardURL:    telemetryForwardURL,
		TelemetryTenantID:      telemetryTenantID,
	}, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
