package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When observability is disabled, Init must return a usable no-op so callers never branch
// and local dev / docker-compose boots with no collector present.
func TestInit_Disabled_ReturnsNoopShutdown(t *testing.T) {
	providers, err := Init(context.Background(), InitConfig{Enabled: false, OTLPEndpoint: "localhost:4317"})

	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NotNil(t, providers.Shutdown)
	assert.NoError(t, providers.Shutdown(context.Background()))
}

// Even when enabled, an empty endpoint means the exporters can't start, so Init must fall
// back to the safe no-op path rather than erroring.
func TestInit_EnabledButNoEndpoint_ReturnsNoopShutdown(t *testing.T) {
	providers, err := Init(context.Background(), InitConfig{Enabled: true, OTLPEndpoint: ""})

	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NotNil(t, providers.Shutdown)
	assert.NoError(t, providers.Shutdown(context.Background()))
}

// The slog logger constructor must return a working logger on the disabled path without
// needing any global provider installed.
func TestNewSlogLogger_Disabled(t *testing.T) {
	slogLogger := NewSlogLogger(false)
	require.NotNil(t, slogLogger)
	slogLogger.Info("disabled slog logger works")
}
