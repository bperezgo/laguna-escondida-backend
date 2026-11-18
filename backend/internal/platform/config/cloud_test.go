package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_BootsWithoutEdgeOnlyVariables(t *testing.T) {
	setCloudEnv(t)
	for _, name := range []string{"ORGANIZATION_ID", "NODE_ID", "CLOUD_SYNC_URL", "STOCK_PULL_CRON", "PRINTER_TRANSPORT", "PRINTER_WIDTH_MM"} {
		t.Setenv(name, "")
	}

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, ElectronicInvoice{URL: "https://invoice.example.com", User: "user", Password: "password"}, cfg.ElectronicInvoice)
}

func TestLoadCloud_IgnoresNodeAndOrganization(t *testing.T) {
	setCloudEnv(t)
	want, err := LoadCloud()
	require.NoError(t, err)

	t.Setenv("ORGANIZATION_ID", testOrganizationID)
	t.Setenv("NODE_ID", "not-even-a-uuid")
	got, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadCloud_InvalidEdgeOnlyValuesDoNotStopIt(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("STOCK_PULL_CRON", "every day at five")
	t.Setenv("PRINTER_WIDTH_MM", "wide")

	_, err := LoadCloud()

	require.NoError(t, err)
}

// Wrong-host protection: a box has no e-invoicing credentials.
func TestLoadCloud_OnABoxEnvironmentRefusesToBoot(t *testing.T) {
	setEdgeEnv(t)

	cfg, err := LoadCloud()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "ELECTRONIC_INVOICE_")
}

func TestLoadCloud_ElectronicInvoiceRequired(t *testing.T) {
	for _, name := range []string{"ELECTRONIC_INVOICE_URL", "ELECTRONIC_INVOICE_USER", "ELECTRONIC_INVOICE_PASSWORD"} {
		t.Run(name, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv(name, "")

			cfg, err := LoadCloud()

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), name)
		})
	}
}

func TestLoadCloud_CronDefaults(t *testing.T) {
	setCloudEnv(t)
	for _, name := range []string{"INVOICE_URL_CRON", "SUPPORT_DOCUMENT_URL_CRON", "INVOICE_SUBMIT_CRON", "STOCK_RECONCILE_CRON"} {
		t.Setenv(name, "")
	}

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "0 * * * *", cfg.InvoiceURLCron)
	assert.Equal(t, "30 * * * *", cfg.SupportDocumentURLCron)
	assert.Equal(t, "* * * * *", cfg.InvoiceSubmitCron)
	assert.Equal(t, "15 3 * * *", cfg.StockReconcileCron)
}

func TestLoadCloud_StockReconcileCron_InvalidExpressionReturnsError(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("STOCK_RECONCILE_CRON", "nightly")

	cfg, err := LoadCloud()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "STOCK_RECONCILE_CRON")
}

func TestLoadCloud_TelemetryForward_ReadFromEnv(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("TELEMETRY_FORWARD_URL", "http://127.0.0.1:4319")
	t.Setenv("TELEMETRY_TENANT_ID", "tenant-abc")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:4319", cfg.TelemetryForwardURL)
	assert.Equal(t, "tenant-abc", cfg.TelemetryTenantID)
	assert.True(t, cfg.TelemetryIngestEnabled())
}

// The cloud has no organization of its own to fall back on.
func TestLoadCloud_TelemetryTenantID_RequiredWithForwardURL(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("TELEMETRY_FORWARD_URL", "http://127.0.0.1:4319")
	t.Setenv("TELEMETRY_TENANT_ID", "")

	cfg, err := LoadCloud()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "TELEMETRY_TENANT_ID")
}

// A cloud without an Alloy sidecar must still boot; it just serves no ingest.
func TestLoadCloud_TelemetryIngest_DisabledWhenForwardURLMissing(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("TELEMETRY_FORWARD_URL", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.TelemetryIngestEnabled())
}
