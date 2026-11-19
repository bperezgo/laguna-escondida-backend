package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_PaymentLinksBothKeys(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("WOMPI_API_URL", "https://sandbox.wompi.co/v1/")
	t.Setenv("WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA", "prv_test_x")
	t.Setenv("WOMPI_EVENTS_SECRET_LAGUNA_ESCONDIDA", "test_events_x")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "https://sandbox.wompi.co/v1", cfg.PaymentLinks.APIURL)
	assert.True(t, cfg.PaymentLinks.ConfiguredFor("laguna-escondida"))
	key, ok := cfg.PaymentLinks.PrivateKeyFor("laguna-escondida")
	assert.True(t, ok)
	assert.Equal(t, "prv_test_x", key)
	secret, ok := cfg.PaymentLinks.EventsSecretFor("laguna-escondida")
	assert.True(t, ok)
	assert.Equal(t, "test_events_x", secret)
	assert.Equal(t, 30*time.Minute, cfg.PaymentLinks.TTL)
}

func TestLoadCloud_PaymentLinksOneKeyIsNotConfigured(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("WOMPI_PRIVATE_KEY_RESTAURANTE_X", "prv_test_x")
	t.Setenv("WOMPI_EVENTS_SECRET_RESTAURANTE_X", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.PaymentLinks.ConfiguredFor("restaurante-x"))
}

func TestLoadCloud_PaymentLinksNone(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("WOMPI_API_URL", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "https://production.wompi.co/v1", cfg.PaymentLinks.APIURL)
	assert.False(t, cfg.PaymentLinks.ConfiguredFor("laguna-escondida"))
}

func TestLoadCloud_PaymentLinkTTL(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("PAYMENT_LINK_TTL", "2m")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, 2*time.Minute, cfg.PaymentLinks.TTL)
}

func TestLoadCloud_InvalidPaymentLinkSettings(t *testing.T) {
	for name, env := range map[string][2]string{
		"ttl not a duration": {"PAYMENT_LINK_TTL", "half an hour"},
		"ttl not positive":   {"PAYMENT_LINK_TTL", "0s"},
		"relative api url":   {"WOMPI_API_URL", "sandbox.wompi.co"},
	} {
		t.Run(name, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv(env[0], env[1])

			_, err := LoadCloud()

			require.ErrorContains(t, err, env[0])
		})
	}
}
