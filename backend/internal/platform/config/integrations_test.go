package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_IntegrationsOffWithoutKey(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("INTEGRATION_SECRETS_KEY", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.Integrations.Enabled())
}

func TestLoadCloud_IntegrationsValidKey(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("INTEGRATION_SECRETS_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.True(t, cfg.Integrations.Enabled())
	assert.Len(t, cfg.Integrations.SecretsKey, 32)
}

func TestLoadCloud_IntegrationsMalformedKey(t *testing.T) {
	tests := map[string]string{
		"not base64":   "%%%not-base64%%%",
		"wrong length": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv("INTEGRATION_SECRETS_KEY", value)

			_, err := LoadCloud()

			require.ErrorContains(t, err, "INTEGRATION_SECRETS_KEY")
		})
	}
}
