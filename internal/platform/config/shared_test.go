package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrganizationID = "019a0000-0000-7000-8000-000000000123"
	testNodeID         = "22222222-2222-2222-2222-222222222222"
)

func setSharedEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ADMIN_API_KEY", "admin-key")
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("STORAGE_REGION", "us-east-1")
	t.Setenv("STORAGE_ACCESS_KEY", "storage-key")
	t.Setenv("STORAGE_SECRET", "storage-secret")
	t.Setenv("STORAGE_BUCKET", "storage-bucket")
}

// setCloudEnv is the cloud's documented env: no organization or node identity.
func setCloudEnv(t *testing.T) {
	t.Helper()
	setSharedEnv(t)
	t.Setenv("ELECTRONIC_INVOICE_URL", "https://invoice.example.com")
	t.Setenv("ELECTRONIC_INVOICE_USER", "user")
	t.Setenv("ELECTRONIC_INVOICE_PASSWORD", "password")
	t.Setenv("ORGANIZATION_ID", "")
	t.Setenv("NODE_ID", "")
}

// setEdgeEnv is a box's documented env: no e-invoicing credentials.
func setEdgeEnv(t *testing.T) {
	t.Helper()
	setSharedEnv(t)
	t.Setenv("ELECTRONIC_INVOICE_URL", "")
	t.Setenv("ELECTRONIC_INVOICE_USER", "")
	t.Setenv("ELECTRONIC_INVOICE_PASSWORD", "")
	t.Setenv("ORGANIZATION_ID", testOrganizationID)
	t.Setenv("NODE_ID", testNodeID)
}

// loaders runs a shared-config case through both binaries' loaders.
var loaders = []struct {
	name   string
	setEnv func(*testing.T)
	load   func() (*Shared, error)
}{
	{name: "cloud", setEnv: setCloudEnv, load: func() (*Shared, error) {
		cfg, err := LoadCloud()
		if err != nil {
			return nil, err
		}
		return &cfg.Shared, nil
	}},
	{name: "edge", setEnv: setEdgeEnv, load: func() (*Shared, error) {
		cfg, err := LoadEdge()
		if err != nil {
			return nil, err
		}
		return &cfg.Shared, nil
	}},
}

func TestLoadShared_RequiredVariables(t *testing.T) {
	for _, l := range loaders {
		for _, name := range []string{"ADMIN_API_KEY", "JWT_SECRET", "STORAGE_BUCKET"} {
			t.Run(l.name+"/"+name, func(t *testing.T) {
				l.setEnv(t)
				t.Setenv(name, "")

				cfg, err := l.load()

				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), name)
			})
		}
	}
}

func TestLoadShared_NodeSyncKey(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name+"/from env", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("NODE_SYNC_KEY", "node-secret")

			cfg, err := l.load()

			require.NoError(t, err)
			assert.Equal(t, "node-secret", cfg.NodeSyncKey)
		})
		t.Run(l.name+"/defaults empty", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("NODE_SYNC_KEY", "")

			cfg, err := l.load()

			require.NoError(t, err)
			assert.Empty(t, cfg.NodeSyncKey)
		})
	}
}

func TestLoadShared_StorageDefaults(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name, func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("STORAGE_REGION", "")
			t.Setenv("CDN_URL_TTL", "")

			cfg, err := l.load()

			require.NoError(t, err)
			assert.Equal(t, "us-east-1", cfg.Storage.Region)
			assert.Equal(t, "storage-bucket", cfg.Storage.Bucket)
			assert.Equal(t, 7*24*time.Hour, cfg.Storage.CDNURLTTL)
		})
	}
}

func TestLoadShared_CDNURLTTL_InvalidReturnsError(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name, func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("CDN_URL_TTL", "a week")

			cfg, err := l.load()

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "invalid CDN_URL_TTL")
		})
	}
}

func TestLoadShared_Observability_Defaults(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name, func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("OBSERVABILITY_ENABLED", "")
			t.Setenv("OTEL_SERVICE_NAME", "")
			t.Setenv("ENVIRONMENT", "")
			t.Setenv("SERVICE_VERSION", "")
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", "")
			t.Setenv("METRICS_PORT", "")

			cfg, err := l.load()

			require.NoError(t, err)
			obs := cfg.Observability
			assert.False(t, obs.Enabled)
			assert.Equal(t, "laguna-backend", obs.ServiceName)
			assert.Equal(t, "dev", obs.Environment)
			assert.Equal(t, "dev", obs.ServiceVersion)
			assert.Empty(t, obs.OTLPEndpoint)
			assert.Equal(t, 1.0, obs.TraceSampleRatio)
			assert.Equal(t, "9090", obs.MetricsPort)
		})
	}
}

func TestLoadShared_Observability_Enabled(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name, func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("OBSERVABILITY_ENABLED", "true")
			t.Setenv("OTEL_SERVICE_NAME", "laguna-cloud")
			t.Setenv("ENVIRONMENT", "prod")
			t.Setenv("SERVICE_VERSION", "v1.2.3")
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
			t.Setenv("METRICS_PORT", "9091")

			cfg, err := l.load()

			require.NoError(t, err)
			obs := cfg.Observability
			assert.True(t, obs.Enabled)
			assert.Equal(t, "laguna-cloud", obs.ServiceName)
			assert.Equal(t, "prod", obs.Environment)
			assert.Equal(t, "v1.2.3", obs.ServiceVersion)
			assert.Equal(t, "localhost:4317", obs.OTLPEndpoint)
			assert.Equal(t, "9091", obs.MetricsPort)
		})
	}
}

func TestLoadShared_TraceSampleRatio(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name+"/from env", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")

			cfg, err := l.load()

			require.NoError(t, err)
			assert.Equal(t, 0.25, cfg.Observability.TraceSampleRatio)
		})
		t.Run(l.name+"/invalid", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", "not-a-number")

			cfg, err := l.load()

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "invalid OTEL_TRACES_SAMPLER_ARG")
		})
	}
}

func TestLoadShared_PhoneHomeRegion(t *testing.T) {
	for _, l := range loaders {
		t.Run(l.name+"/defaults to CO", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("PHONE_HOME_REGION", "")

			cfg, err := l.load()

			require.NoError(t, err)
			assert.Equal(t, "CO", cfg.PhoneHomeRegion)
		})
		t.Run(l.name+"/upper-cased", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("PHONE_HOME_REGION", "mx")

			cfg, err := l.load()

			require.NoError(t, err)
			assert.Equal(t, "MX", cfg.PhoneHomeRegion)
		})
		t.Run(l.name+"/unsupported", func(t *testing.T) {
			l.setEnv(t)
			t.Setenv("PHONE_HOME_REGION", "ZZ")

			cfg, err := l.load()

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "PHONE_HOME_REGION")
		})
	}
}

// The mode comes from the binary alone.
func TestLoaders_AppModeHasNoEffect(t *testing.T) {
	for _, l := range loaders {
		for _, appMode := range []string{"cloud", "edge", "staging"} {
			t.Run(l.name+"/APP_MODE="+appMode, func(t *testing.T) {
				l.setEnv(t)
				t.Setenv("APP_MODE", "")
				want, err := l.load()
				require.NoError(t, err)

				t.Setenv("APP_MODE", appMode)
				got, err := l.load()

				require.NoError(t, err)
				assert.Equal(t, want, got)
			})
		}
	}
}
