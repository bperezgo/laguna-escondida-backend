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

// otlpEndpointURL must produce a scheme-qualified URL for otlp*grpc.WithEndpointURL. The
// prod contract from aws-infra is the OTel-standard URL form ("http://127.0.0.1:4317"); a
// bare "host:port" (local/dev) is treated as plaintext. Passing the raw URL to the old
// WithEndpoint made gRPC append :443 -> "too many colons in address", dropping all
// traces/logs — this pins the regression.
func TestOTLPEndpointURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:4317":  "http://127.0.0.1:4317", // prod contract, unchanged
		"127.0.0.1:4317":         "http://127.0.0.1:4317", // bare host:port -> plaintext
		"localhost:4317":         "http://localhost:4317",
		"https://collector:4317": "https://collector:4317", // scheme preserved (future TLS)
	}
	for in, want := range cases {
		assert.Equal(t, want, otlpEndpointURL(in), "input %q", in)
	}
}

// The slog logger constructor must return a working logger on the disabled path without
// needing any global provider installed.
func TestNewSlogLogger_Disabled(t *testing.T) {
	slogLogger := NewSlogLogger(false)
	require.NotNil(t, slogLogger)
	slogLogger.Info("disabled slog logger works")
}
