package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_StaffNotificationsOffWithoutKey(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("NOTIFICATION_SECRETS_KEY", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.StaffNotifications.Enabled())
}

func TestLoadCloud_StaffNotificationsValidKey(t *testing.T) {
	setCloudEnv(t)
	key := make([]byte, 32)
	t.Setenv("NOTIFICATION_SECRETS_KEY", base64.StdEncoding.EncodeToString(key))

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.True(t, cfg.StaffNotifications.Enabled())
	assert.Len(t, cfg.StaffNotifications.SecretsKey, 32)
}

func TestLoadCloud_StaffNotificationsMalformedKey(t *testing.T) {
	tests := map[string]string{
		"not base64":   "%%%not-base64%%%",
		"wrong length": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv("NOTIFICATION_SECRETS_KEY", value)

			_, err := LoadCloud()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "NOTIFICATION_SECRETS_KEY")
		})
	}
}
