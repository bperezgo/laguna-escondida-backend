// Package apikeys drives organization API keys end to end through the real cloud assembly:
// issued over the platform routes, then used as a bearer against the tenant routes.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/apikeys/...
package apikeys

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/pkg/eventbus"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	dbName      = "laguna_accept_api_keys"
	adminAPIKey = "accept-admin-key"
	kitchenArea = "kitchen"
)

var (
	cloudDB     *gorm.DB
	pgContainer *testsupport.PostgresContainer
)

func gateEnabled() bool { return os.Getenv("RUN_ACCEPTANCE_TESTS") == "true" }

func TestMain(m *testing.M) {
	if !gateEnabled() {
		os.Exit(m.Run())
	}
	if err := provision(); err != nil {
		fmt.Fprintln(os.Stderr, "acceptance DB provisioning failed:", err)
		os.Exit(1)
	}
	code := m.Run()
	if pgContainer != nil {
		_ = pgContainer.Terminate(context.Background())
	}
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func provision() error {
	host, port, user, password, sslmode := envOr("DB_HOST", ""), envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"), envOr("DB_PASSWORD", "postgres"), envOr("DB_SSLMODE", "disable")
	if host == "" {
		ctr, err := testsupport.StartPostgres(context.Background())
		if err != nil {
			return fmt.Errorf("boot postgres testcontainer (is Docker running?): %w", err)
		}
		pgContainer = ctr
		host, port, user, password, sslmode = ctr.Host, ctr.Port, ctr.User, ctr.Password, ctr.SSLMode
	}
	dsn := func(name string) string {
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, name, sslmode)
	}

	admin, err := openGorm(dsn("postgres"))
	if err != nil {
		return err
	}
	if err = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName)).Error; err != nil {
		return err
	}
	if err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error; err != nil {
		return err
	}
	if sqlDB, dbErr := admin.DB(); dbErr == nil {
		_ = sqlDB.Close()
	}

	migrator, err := testsupport.NewMigrator(migrations.Cloud,
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslmode))
	if err != nil {
		return err
	}
	defer func() { _, _ = migrator.Close() }()
	if err = migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	cloudDB, err = openGorm(dsn(dbName))
	return err
}

func openGorm(dsn string) (*gorm.DB, error) {
	return gorm.Open(gormpg.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
}

// cloud is one running cloud binary behind httptest.
type cloud struct {
	t      *testing.T
	server *httptest.Server
}

func newCloud(t *testing.T) *cloud {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	t.Cleanup(func() { _ = bus.Close() })
	cfg := &config.Cloud{Shared: config.Shared{
		AdminAPIKey: adminAPIKey, JWTSecret: "accept-secret", NodeSyncKey: "accept-node-key", PhoneHomeRegion: "CO",
		Observability: config.Observability{ServiceName: "laguna-backend", MetricsPort: "0"},
	}}
	app, err := bootstrap.NewCloud(cfg, bootstrap.Infra{
		DB:                cloudDB,
		Logger:            logger,
		HTTPClient:        httpclient.NewClient(logger),
		Storage:           mocks.NewMockStorageClient(t),
		ElectronicInvoice: mocks.NewMockElectronicInvoiceClient(t),
		EventBus:          bus,
	})
	require.NoError(t, err)
	server := httptest.NewServer(app.Router)
	t.Cleanup(server.Close)
	return &cloud{t: t, server: server}
}

type response struct {
	status int
	body   []byte
}

func (r response) decode(t *testing.T, into any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(r.body, into), string(r.body))
}

func (c *cloud) call(method, path string, headers map[string]string, body any) response {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(c.t, err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.server.URL+path, reader)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return response{status: resp.StatusCode, body: raw}
}

func (c *cloud) withKey(key, method, path string, body any) response {
	c.t.Helper()
	return c.call(method, path, map[string]string{"Authorization": "Bearer " + key}, body)
}

func (c *cloud) admin(method, path string, body any) response {
	c.t.Helper()
	return c.call(method, path, map[string]string{"X-API-Key": adminAPIKey}, body)
}

// restaurant is an organization with an agent user, a product on its menu and an issued key.
type restaurant struct {
	orgID     string
	agentID   string
	productID string
	key       string
	keyID     string
}

func seedOrganization(t *testing.T) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, cloudDB.Exec("INSERT INTO organizations (id, name, slug, status) VALUES (?, ?, ?, 'active')",
		id, "Restaurant "+id[:8], "r-"+id[:8]).Error)
	return id
}

func seedUser(t *testing.T, orgID string, roleIDs ...int) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, cloudDB.Exec(`INSERT INTO users (id, organization_id, username, name, password, active, created_at, updated_at)
		VALUES (?, ?, ?, 'Agente', 'x', true, ?, ?)`, id, orgID, "agent-"+id[:8], now, now).Error)
	for _, roleID := range roleIDs {
		require.NoError(t, cloudDB.Exec(`INSERT INTO user_roles (user_id, role_id, organization_id, created_at) VALUES (?, ?, ?, ?)`,
			id, roleID, orgID, now).Error)
	}
	return id
}

func seedProduct(t *testing.T, orgID, name string) string {
	t.Helper()
	ctx := tenant.WithOrg(context.Background(), orgID)
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.NewString()
	refs := repository.NewSyncReferenceRepository(cloudDB)
	require.NoError(t, refs.UpsertProducts(ctx, []dto.ProductSyncPayload{{
		ID: id, Name: name, Category: "platos", ProductType: "SELLABLE", UnitOfMeasure: "unit", Version: 1,
		UnitPrice: decimal.NewFromInt(20000), VAT: decimal.Zero, VATAmount: decimal.Zero, ICO: decimal.Zero, ICOAmount: decimal.Zero,
		SKU: "SKU-" + id[:8], TotalPriceWithTaxes: decimal.NewFromInt(20000), CreatedAt: now, UpdatedAt: now,
	}}))
	require.NoError(t, refs.UpsertProductResponsibilities(ctx, []dto.ProductResponsibilitySyncPayload{{
		ID: uuid.NewString(), ProductID: id, Area: kitchenArea, Priority: 1, CreatedAt: now, UpdatedAt: now,
	}}))
	return id
}

// newRestaurant issues the agent's key through the platform route, as an operator would.
func (c *cloud) newRestaurant(menuItem string) *restaurant {
	c.t.Helper()
	r := &restaurant{orgID: seedOrganization(c.t)}
	r.agentID = seedUser(c.t, r.orgID, permissions.RoleAgent)
	r.productID = seedProduct(c.t, r.orgID, menuItem)

	resp := c.admin(http.MethodPost, "/api/platform/organizations/"+r.orgID+"/api-keys",
		map[string]string{"user_id": r.agentID, "name": "whatsapp-agent"})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var issued dto.IssuedAPIKey
	resp.decode(c.t, &issued)
	r.key, r.keyID = issued.Key, issued.ID
	return r
}

func (c *cloud) createTakeaway(r *restaurant) response {
	c.t.Helper()
	return c.withKey(r.key, http.MethodPost, "/api/orders", map[string]any{
		"open_bill_id":        uuid.NewString(),
		"temporal_identifier": uuid.NewString(),
		"order_type":          "takeaway",
		"customer_phone":      "3005551234",
		"products": []map[string]any{
			{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 1},
		},
	})
}

func orderID(t *testing.T, resp response) string {
	t.Helper()
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var order struct {
		ID string `json:"id"`
	}
	resp.decode(t, &order)
	require.NotEmpty(t, order.ID)
	return order.ID
}
