package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDiscordWebhook = "https://discord.com/api/webhooks/123456/abcdef"

func TestLoadCloud_LeadAlertsOffWithoutWebhook(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("LEAD_ALERT_DISCORD_WEBHOOK_URL", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.LeadAlerts.Enabled())
}

func TestLoadCloud_LeadAlertsValidWebhook(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("LEAD_ALERT_DISCORD_WEBHOOK_URL", " "+testDiscordWebhook+" ")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.True(t, cfg.LeadAlerts.Enabled())
	assert.Equal(t, testDiscordWebhook, cfg.LeadAlerts.DiscordWebhookURL)
}

func TestLoadCloud_LeadAlertsMalformedWebhookStopsStartup(t *testing.T) {
	for _, value := range []string{"not a url", "http://discord.com/api/webhooks/1/x", "https://example.com/api/webhooks/1/x"} {
		t.Run(value, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv("LEAD_ALERT_DISCORD_WEBHOOK_URL", value)

			_, err := LoadCloud()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "LEAD_ALERT_DISCORD_WEBHOOK_URL")
		})
	}
}

func TestLoadCloud_TrustedProxiesUnsetKeepsDefault(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("TRUSTED_PROXIES", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Nil(t, cfg.TrustedProxies)
}

func TestLoadCloud_TrustedProxiesValidList(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/16, 172.16.1.5 ,")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.0/16", "172.16.1.5"}, cfg.TrustedProxies)
}

func TestLoadCloud_TrustedProxiesMalformedEntryStopsStartup(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/16,not-an-ip")

	_, err := LoadCloud()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "TRUSTED_PROXIES")
}
