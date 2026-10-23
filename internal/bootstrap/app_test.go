package bootstrap

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const testOrganizationID = "019a0000-0000-7000-8000-000000000123"

func testCloudConfig() *config.Config {
	return &config.Config{
		AppMode: config.ModeCloud, NodeID: "cloud-node", NodeSyncKey: "node-key", AdminAPIKey: "admin-key",
		JWTSecret: "test-secret", ServiceName: "laguna-backend", MetricsPort: "0", PhoneHomeRegion: "CO",
		TelemetryForwardURL: "http://127.0.0.1:4319", TelemetryTenantID: "tenant",
	}
}

func testEdgeConfig() *config.Config {
	return &config.Config{
		AppMode: config.ModeEdge, NodeID: "edge-node", NodeSyncKey: "node-key", AdminAPIKey: "admin-key",
		JWTSecret: "test-secret", ServiceName: "laguna-backend", MetricsPort: "0", PhoneHomeRegion: "CO",
		OrganizationID: testOrganizationID, CloudSyncURL: "http://127.0.0.1:1", CloudNodeID: "cloud-node",
	}
}

// testInfra points at a database nobody listens on: assembling an app must not query it.
func testInfra(t *testing.T) Infra {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 port=1 user=test dbname=test sslmode=disable"),
		&gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	t.Cleanup(func() { _ = bus.Close() })
	return Infra{
		DB:                db,
		Logger:            logger,
		HTTPClient:        httpclient.NewClient(logger),
		Storage:           mocks.NewMockStorageClient(t),
		ElectronicInvoice: mocks.NewMockElectronicInvoiceClient(t),
		EventBus:          bus,
	}
}

func routeTable(router *gin.Engine) string {
	routes := router.Routes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	var b strings.Builder
	for _, r := range routes {
		fmt.Fprintf(&b, "%s %s %s\n", r.Method, r.Path, r.Handler)
	}
	return b.String()
}

// The golden files are the single binary's route tables from before the split: a diff here is
// a change to what a binary serves, and must be intended.
func TestRoutes_MatchGolden(t *testing.T) {
	cases := []struct {
		mode     config.Mode
		assemble func(*config.Config, Infra) (*App, error)
		cfg      *config.Config
	}{
		{config.ModeCloud, NewCloud, testCloudConfig()},
		{config.ModeEdge, NewEdge, testEdgeConfig()},
	}
	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			app, err := tc.assemble(tc.cfg, testInfra(t))
			require.NoError(t, err)
			want, err := os.ReadFile(filepath.Join("testdata", "routes_"+string(tc.mode)+".golden"))
			require.NoError(t, err)

			assert.Equal(t, string(want), routeTable(app.Router))
		})
	}
}

func TestNewCloudAndNewEdge_StartNothingBeforeRun(t *testing.T) {
	cases := []struct {
		name     string
		assemble func(*config.Config, Infra) (*App, error)
		cfg      *config.Config
	}{
		{"cloud", NewCloud, testCloudConfig()},
		{"edge", NewEdge, testEdgeConfig()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			infra := testInfra(t)
			before := runtime.NumGoroutine()

			app, err := tc.assemble(tc.cfg, infra)
			require.NoError(t, err)

			time.Sleep(50 * time.Millisecond)
			assert.LessOrEqual(t, runtime.NumGoroutine(), before, "no scheduler or subscriber goroutine before Run")
			assert.Len(t, app.subscribers, 2)
			assert.Len(t, app.schedulers, 1)
		})
	}
}

func TestNewCloudAndNewEdge_RefuseTheOtherModesConfig(t *testing.T) {
	_, err := NewCloud(testEdgeConfig(), testInfra(t))
	require.Error(t, err)

	_, err = NewEdge(testCloudConfig(), testInfra(t))
	require.Error(t, err)
}

func TestNewEdge_RequiresTheCloudSyncSettings(t *testing.T) {
	cfg := testEdgeConfig()
	cfg.CloudSyncURL = ""

	_, err := NewEdge(cfg, testInfra(t))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLOUD_SYNC_URL")
}
