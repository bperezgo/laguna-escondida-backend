package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEdge_BootsWithoutCloudOnlyVariables(t *testing.T) {
	setEdgeEnv(t)
	for _, name := range []string{"STOCK_RECONCILE_CRON", "TELEMETRY_FORWARD_URL", "TELEMETRY_TENANT_ID"} {
		t.Setenv(name, "")
	}

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, testOrganizationID, cfg.OrganizationID)
}

func TestLoadEdge_InvalidCloudOnlyValuesDoNotStopIt(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("STOCK_RECONCILE_CRON", "nightly")
	t.Setenv("TELEMETRY_FORWARD_URL", "http://127.0.0.1:4319")
	t.Setenv("TELEMETRY_TENANT_ID", "")

	_, err := LoadEdge()

	require.NoError(t, err)
}

// Wrong-host protection: the cloud has no organization or node identity.
func TestLoadEdge_OnTheCloudEnvironmentRefusesToBoot(t *testing.T) {
	setCloudEnv(t)

	cfg, err := LoadEdge()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "ORGANIZATION_ID")
}

func TestLoadEdge_Identity(t *testing.T) {
	cases := []struct {
		name    string
		orgID   string
		nodeID  string
		wantErr string
	}{
		{name: "both set", orgID: testOrganizationID, nodeID: testNodeID},
		{name: "without organization", nodeID: testNodeID, wantErr: "ORGANIZATION_ID"},
		{name: "malformed organization", orgID: "test-org", nodeID: testNodeID, wantErr: "is not an organization id"},
		{name: "without node id", orgID: testOrganizationID, wantErr: "NODE_ID"},
		{name: "malformed node id", orgID: testOrganizationID, nodeID: "box-1", wantErr: "is not a node id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEdgeEnv(t)
			t.Setenv("ORGANIZATION_ID", tc.orgID)
			t.Setenv("NODE_ID", tc.nodeID)

			cfg, err := LoadEdge()

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.orgID, cfg.OrganizationID)
			assert.Equal(t, tc.nodeID, cfg.NodeID, "the configured node id is used as-is")
		})
	}
}

func TestLoadEdge_CloudNodeIDEnvHasNoEffect(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("CLOUD_NODE_ID", "11111111-1111-1111-1111-111111111111")

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, CloudNodeIDFor(testOrganizationID), cfg.CloudNodeID)
}

// Laguna's box keeps its sync_state cursors under this key; it must never change.
func TestLoadEdge_LagunaCloudPeerKeyIsUnchanged(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("ORGANIZATION_ID", "bb0cad3e-be13-4bad-9a37-bba9525bfece")

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, "45598059-e12e-530c-86b6-758477c4423d", cfg.CloudNodeID)
}

func TestLoadEdge_CloudSyncURL(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("CLOUD_SYNC_URL", "http://cloud.local:8081")

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, "http://cloud.local:8081", cfg.CloudSyncURL)
}

func TestLoadEdge_CronDefaults(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("SYNC_PUSH_CRON", "")
	t.Setenv("SYNC_PULL_CRON", "")
	t.Setenv("STOCK_PULL_CRON", "")

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, "* * * * *", cfg.SyncPushCron)
	assert.Equal(t, "* * * * *", cfg.SyncPullCron)
	assert.Equal(t, "0 5 * * *", cfg.StockPullCron, "the edge adopts the cloud's numbers once a day, before service")
}

func TestLoadEdge_CronOverrides(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("SYNC_PUSH_CRON", "*/5 * * * *")
	t.Setenv("SYNC_PULL_CRON", "*/2 * * * *")
	t.Setenv("STOCK_PULL_CRON", "30 4 * * *")

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, "*/5 * * * *", cfg.SyncPushCron)
	assert.Equal(t, "*/2 * * * *", cfg.SyncPullCron)
	assert.Equal(t, "30 4 * * *", cfg.StockPullCron)
}

func TestLoadEdge_StockPullCron_InvalidExpressionReturnsError(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("STOCK_PULL_CRON", "every day at five")

	cfg, err := LoadEdge()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "STOCK_PULL_CRON")
}

func TestLoadEdge_PrinterDefaults(t *testing.T) {
	setEdgeEnv(t)
	for _, name := range []string{"PRINTER_TRANSPORT", "PRINTER_TARGET", "PRINTER_WIDTH_MM", "PRINTER_CODEPAGE", "PRINTER_CUT"} {
		t.Setenv(name, "")
	}

	cfg, err := LoadEdge()

	require.NoError(t, err)
	assert.Equal(t, Printer{Transport: "file", WidthMM: 80, Codepage: "CP850", Cut: "partial"}, cfg.Printer)
}

func TestLoadEdge_PrinterWidth_InvalidReturnsError(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("PRINTER_WIDTH_MM", "wide")

	cfg, err := LoadEdge()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "PRINTER_WIDTH_MM")
}
