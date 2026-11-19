package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_PaymentLinksSharedSettings(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("WOMPI_API_URL", "https://sandbox.wompi.co/v1/")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "https://sandbox.wompi.co/v1", cfg.PaymentLinks.APIURL)
	assert.Equal(t, 30*time.Minute, cfg.PaymentLinks.TTL)
}

func TestLoadCloud_PaymentLinksDefaultAPI(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("WOMPI_API_URL", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "https://production.wompi.co/v1", cfg.PaymentLinks.APIURL)
}

func TestLoadCloud_PaymentLinksListsLeftoverKeysByNameOnly(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA", "prv_test_leftover")
	t.Setenv("WOMPI_EVENTS_SECRET_LAGUNA_ESCONDIDA", "test_events_leftover")
	t.Setenv("WOMPI_PRIVATE_KEY_RESTAURANTE_X", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, []string{"WOMPI_EVENTS_SECRET_LAGUNA_ESCONDIDA", "WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA"}, cfg.PaymentLinks.IgnoredEnv)
	assert.NotContains(t, strings.Join(cfg.PaymentLinks.IgnoredEnv, " "), "leftover")
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
