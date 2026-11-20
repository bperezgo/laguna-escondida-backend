package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_MarketingEventsOffWithoutURL(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("MARKETING_APP_EVENTS_URL", "")
	t.Setenv("MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA", "s3cret")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.MarketingEvents.Enabled())
}

func TestLoadCloud_MarketingEventsURL(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("MARKETING_APP_EVENTS_URL", "https://marketing.example.com/")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.True(t, cfg.MarketingEvents.Enabled())
	assert.Equal(t, "https://marketing.example.com", cfg.MarketingEvents.URL)
}

func TestLoadCloud_MarketingEventsLeftoverSecretsNamedNotRead(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("MARKETING_APP_EVENTS_URL", "https://marketing.example.com")
	t.Setenv("MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA", "leftover-value")
	t.Setenv("MARKETING_APP_EVENTS_SECRET_EMPTY_ONE", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, []string{"MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA"}, cfg.MarketingEvents.IgnoredEnv)
	assert.NotContains(t, strings.Join(cfg.MarketingEvents.IgnoredEnv, " "), "leftover-value")
}

func TestLoadCloud_MarketingEventsURLMustBeAbsolute(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("MARKETING_APP_EVENTS_URL", "marketing.example.com")

	_, err := LoadCloud()

	require.ErrorContains(t, err, "MARKETING_APP_EVENTS_URL")
}

func TestLoadCloud_CRMCronDefaults(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("FEEDBACK_REQUEST_CRON", "")
	t.Setenv("RESERVATION_VISITED_CRON", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "5 * * * *", cfg.FeedbackRequestCron)
	assert.Equal(t, "*/15 * * * *", cfg.ReservationVisitedCron)
}

func TestLoadCloud_InvalidCRMCron(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("RESERVATION_VISITED_CRON", "every quarter hour")

	_, err := LoadCloud()

	require.ErrorContains(t, err, "RESERVATION_VISITED_CRON")
}
