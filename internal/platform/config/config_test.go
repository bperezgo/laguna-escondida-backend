package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setRequiredEnv sets every env var NewConfig needs to succeed, except APP_MODE
// (left to each test to control). t.Setenv restores the previous values on cleanup.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ELECTRONIC_INVOICE_URL", "https://invoice.example.com")
	t.Setenv("ELECTRONIC_INVOICE_USER", "user")
	t.Setenv("ELECTRONIC_INVOICE_PASSWORD", "password")
	t.Setenv("ADMIN_API_KEY", "admin-key")
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("SPACES_REGION", "us-east-1")
	t.Setenv("SPACES_KEY", "spaces-key")
	t.Setenv("SPACES_SECRET", "spaces-secret")
	t.Setenv("SPACES_BUCKET", "spaces-bucket")
	t.Setenv("ORGANIZATION_ID", "org-123")
}

func TestNewConfig_AppMode_DefaultsToCloud(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_MODE", "")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, ModeCloud, cfg.AppMode)
}

func TestNewConfig_AppMode_Edge(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_MODE", "edge")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, ModeEdge, cfg.AppMode)
}

func TestNewConfig_AppMode_Cloud(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_MODE", "cloud")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, ModeCloud, cfg.AppMode)
}

func TestNewConfig_AppMode_InvalidReturnsError(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_MODE", "datacenter")

	cfg, err := NewConfig()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid APP_MODE")
}

func TestNewConfig_NodeID_DefaultsToStableValue(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("NODE_ID", "")

	first, err := NewConfig()
	require.NoError(t, err)
	second, err := NewConfig()
	require.NoError(t, err)

	assert.NotEmpty(t, first.NodeID)
	assert.Equal(t, first.NodeID, second.NodeID, "derived node id must be stable across restarts")
}

func TestNewConfig_NodeID_Override(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("NODE_ID", "22222222-2222-2222-2222-222222222222")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", cfg.NodeID)
}

func TestNewConfig_NodeSyncKey_FromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("NODE_SYNC_KEY", "node-secret")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "node-secret", cfg.NodeSyncKey)
}

func TestNewConfig_NodeSyncKey_DefaultsEmpty(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("NODE_SYNC_KEY", "")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Empty(t, cfg.NodeSyncKey)
}

func TestNewConfig_CloudSyncURL_FromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLOUD_SYNC_URL", "http://cloud.local:8081")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "http://cloud.local:8081", cfg.CloudSyncURL)
}

func TestNewConfig_CloudSyncURL_DefaultsEmpty(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLOUD_SYNC_URL", "")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Empty(t, cfg.CloudSyncURL)
}

func TestNewConfig_CloudNodeID_FromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLOUD_NODE_ID", "11111111-1111-1111-1111-111111111111")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", cfg.CloudNodeID)
}

func TestNewConfig_CloudNodeID_DerivedDefault_IsStable(t *testing.T) {
	setRequiredEnv(t)
	// Run as an edge node so this node's id (mode "edge") differs from the derived
	// cloud peer id (mode "cloud"); let both ids derive from the org default.
	t.Setenv("APP_MODE", "edge")
	t.Setenv("NODE_ID", "")
	t.Setenv("CLOUD_NODE_ID", "")

	first, err := NewConfig()
	require.NoError(t, err)
	second, err := NewConfig()
	require.NoError(t, err)

	assert.NotEmpty(t, first.CloudNodeID)
	assert.Equal(t, first.CloudNodeID, second.CloudNodeID, "derived cloud node id must be stable across boots")
	assert.NotEqual(t, first.NodeID, first.CloudNodeID, "cloud peer id differs from this edge node's id")
}

func TestNewConfig_SyncPushCron_DefaultsEveryMinute(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SYNC_PUSH_CRON", "")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "* * * * *", cfg.SyncPushCron)
}

func TestNewConfig_SyncPushCron_FromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SYNC_PUSH_CRON", "*/5 * * * *")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "*/5 * * * *", cfg.SyncPushCron)
}

func TestNewConfig_SyncPullCron_DefaultsEveryMinute(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SYNC_PULL_CRON", "")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "* * * * *", cfg.SyncPullCron)
}

func TestNewConfig_SyncPullCron_FromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SYNC_PULL_CRON", "*/2 * * * *")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, "*/2 * * * *", cfg.SyncPullCron)
}
