// Package payments drives agent-order payment links end to end through the real cloud assembly:
// order routes, the Wompi webhook and the expiry job on Postgres, against a fake Wompi API.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/payments/...
package payments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
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
	dbName       = "laguna_accept_payments"
	adminAPIKey  = "accept-admin-key"
	jwtSecret    = "accept-secret"
	eventsSecret = "test_events_accept" //nolint:gosec // G101: a test fixture, not a credential
	privateKey   = "prv_test_accept"    //nolint:gosec // G101: a test fixture, not a credential
	customer     = "3005551234"
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

func skipUnlessEnabled(t *testing.T) {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
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

// fakeWompi is Wompi's payment links API: every created link gets a fresh id; deactivations are recorded.
type fakeWompi struct {
	mu          sync.Mutex
	created     []string
	deactivated []string
	fail        bool
	server      *httptest.Server
}

func newFakeWompi(t *testing.T) *fakeWompi {
	f := &fakeWompi{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+privateKey || f.fail {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodPost:
			id := "wl_" + uuid.NewString()[:8]
			f.created = append(f.created, id)
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"data":{"id":%q,"active":true}}`, id)
		case http.MethodPatch:
			f.deactivated = append(f.deactivated, r.URL.Path[len("/v1/payment_links/"):])
			_, _ = w.Write([]byte(`{"data":{"active":false}}`))
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeWompi) wasDeactivated(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.deactivated {
		if d == id {
			return true
		}
	}
	return false
}

// restaurant is an organization with a product, its agent's key and a manager's token.
type restaurant struct {
	orgID, slug, productID, agentKey, managerKey string
}

func seedRestaurant(t *testing.T) *restaurant {
	t.Helper()
	skipUnlessEnabled(t)
	id := uuid.NewString()
	r := &restaurant{orgID: id, slug: "r-" + id[:8]}
	require.NoError(t, cloudDB.Exec("INSERT INTO organizations (id, name, slug, status) VALUES (?, ?, ?, 'active')",
		id, "Restaurant "+id[:8], r.slug).Error)
	r.productID = seedProduct(t, id)
	return r
}

func seedUser(t *testing.T, orgID string, roleIDs ...int) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, cloudDB.Exec(`INSERT INTO users (id, organization_id, username, name, password, active, created_at, updated_at)
		VALUES (?, ?, ?, 'Usuario', 'x', true, ?, ?)`, id, orgID, "user-"+id[:8], now, now).Error)
	for _, roleID := range roleIDs {
		require.NoError(t, cloudDB.Exec(`INSERT INTO user_roles (user_id, role_id, organization_id, created_at) VALUES (?, ?, ?, ?)`,
			id, roleID, orgID, now).Error)
	}
	return id
}

func seedProduct(t *testing.T, orgID string) string {
	t.Helper()
	ctx := tenant.WithOrg(context.Background(), orgID)
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.NewString()
	refs := repository.NewSyncReferenceRepository(cloudDB)
	require.NoError(t, refs.UpsertProducts(ctx, []dto.ProductSyncPayload{{
		ID: id, Name: "Mojarra frita", Category: "platos", ProductType: "SELLABLE", UnitOfMeasure: "unit", Version: 1,
		UnitPrice: decimal.NewFromInt(45000), VAT: decimal.Zero, VATAmount: decimal.Zero, ICO: decimal.Zero, ICOAmount: decimal.Zero,
		SKU: "SKU-" + id[:8], TotalPriceWithTaxes: decimal.NewFromInt(45000), CreatedAt: now, UpdatedAt: now,
	}}))
	return id
}

// cloud is one cloud binary behind httptest, with payment links for the given organizations.
type cloud struct {
	t      *testing.T
	server *httptest.Server
	wompi  *fakeWompi
	links  config.PaymentLinks
	logger *slog.Logger
}

func newCloud(t *testing.T, ttl time.Duration, configured ...*restaurant) *cloud {
	t.Helper()
	skipUnlessEnabled(t)
	gin.SetMode(gin.TestMode)
	wompi := newFakeWompi(t)
	keys, secrets := map[string]string{}, map[string]string{}
	for _, r := range configured {
		keys[r.slug], secrets[r.slug] = privateKey, eventsSecret
	}
	links := config.NewPaymentLinks(wompi.server.URL+"/v1", ttl, keys, secrets)
	logger := slog.New(slog.NewTextHandler(logOutput(), &slog.HandlerOptions{Level: slog.LevelError}))
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	t.Cleanup(func() { _ = bus.Close() })
	cfg := &config.Cloud{
		Shared: config.Shared{
			AdminAPIKey: adminAPIKey, JWTSecret: jwtSecret, NodeSyncKey: "accept-node-key", PhoneHomeRegion: "CO",
			Observability: config.Observability{ServiceName: "laguna-backend", MetricsPort: "0"},
		},
		PaymentLinks: links,
	}
	app, err := bootstrap.NewCloud(cfg, bootstrap.Infra{
		DB: cloudDB, Logger: logger, HTTPClient: httpclient.NewClient(logger),
		Storage: mocks.NewMockStorageClient(t), ElectronicInvoice: mocks.NewMockElectronicInvoiceClient(t), EventBus: bus,
	})
	require.NoError(t, err)
	server := httptest.NewServer(app.Router)
	t.Cleanup(server.Close)
	return &cloud{t: t, server: server, wompi: wompi, links: links, logger: logger}
}

// join gives the restaurant its agent's key and a manager's token on this cloud.
func (c *cloud) join(r *restaurant) *restaurant {
	c.t.Helper()
	resp := c.call(http.MethodPost, "/api/platform/organizations/"+r.orgID+"/api-keys",
		map[string]string{"X-API-Key": adminAPIKey},
		map[string]string{"user_id": seedUser(c.t, r.orgID, permissions.RoleAgent), "name": "agent"})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var issued dto.IssuedAPIKey
	resp.decode(c.t, &issued)
	r.agentKey = issued.Key
	managerID := seedUser(c.t, r.orgID, permissions.RoleManager)
	token, err := service.NewJWTService(jwtSecret, tenant.AnyOrganization()).
		GenerateToken(managerID, r.orgID, "manager-"+managerID[:8], []int{permissions.RoleManager})
	require.NoError(c.t, err)
	r.managerKey = token
	return r
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
	switch b := body.(type) {
	case nil:
	case []byte:
		reader = bytes.NewReader(b)
	default:
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

func (c *cloud) as(key, method, path string, body any) response {
	c.t.Helper()
	return c.call(method, path, map[string]string{"Authorization": "Bearer " + key}, body)
}

func (c *cloud) createTakeaway(key string, r *restaurant, quantity int) response {
	c.t.Helper()
	return c.as(key, http.MethodPost, "/api/orders", map[string]any{
		"open_bill_id": uuid.NewString(), "temporal_identifier": uuid.NewString(),
		"order_type": "takeaway", "customer_phone": customer,
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": quantity}},
	})
}

// wompiEvent is a transaction.updated as Wompi signs it.
func wompiEvent(txID, status, linkID string, amountInCents int64, secret string) []byte {
	const timestamp = 1763485200
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d%d%s", txID, status, amountInCents, timestamp, secret)))
	return []byte(fmt.Sprintf(`{"event":"transaction.updated","data":{"transaction":{"id":%q,"amount_in_cents":%d,`+
		`"currency":"COP","payment_method_type":"CARD","status":%q,"payment_link_id":%q}},"environment":"test",`+
		`"signature":{"properties":["transaction.id","transaction.status","transaction.amount_in_cents"],"checksum":%q},"timestamp":%d}`,
		txID, amountInCents, status, linkID, hex.EncodeToString(sum[:]), timestamp))
}

func (c *cloud) deliver(r *restaurant, event []byte) int {
	c.t.Helper()
	return c.call(http.MethodPost, "/api/webhooks/wompi/"+r.orgID, nil, event).status
}

// expireDue runs one pass of the expiry job, built on the same repositories the cloud uses.
func (c *cloud) expireDue() {
	c.t.Helper()
	db := cloudDB
	orgs := repository.NewOrganizationRepository(db)
	linkRepo := repository.NewPaymentLinkRepository(db)
	txRepo := repository.NewPaymentTransactionRepository(db)
	links := service.NewPaymentLinkService(linkRepo, httpclient.NewWompiClient(httpclient.NewClient(c.logger), c.links.APIURL),
		c.links, orgs, c.links.TTL, c.logger, nil)
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	defer func() { _ = bus.Close() }()
	orders := service.NewOrderServiceWithSSE(c.logger, repository.NewOpenBillRepository(db), repository.NewProductRepository(db),
		repository.NewBillRepository(db), repository.NewPendingInvoiceRepository(db), dto.PendingInvoiceStatusPending,
		repository.NewBillOwnerRepository(db), postgres.NewUnitOfWork(db), bus, repository.NewUserRepository(db), nil,
		repository.NewSyncOutboxRepository(db), dto.SyncIdentity{NodeID: uuid.NewString()}, repository.NewProductIngredientRepository(db),
		repository.NewDeliveryZoneRepository(db), "CO", service.WithPaymentLinks(links))
	runner := service.NewOrganizationRunner(orgs, postgres.NewSessionOpener(db))
	require.NoError(c.t, service.NewPaymentExpiryService(runner, linkRepo, txRepo, links, orders, c.logger, nil).ExpireDue(context.Background()))
}

type order struct {
	ID         string     `json:"id"`
	PaymentURL *string    `json:"payment_url"`
	PaidAt     *time.Time `json:"paid_at"`
	VoidReason *string    `json:"void_reason"`
}

func linkID(t *testing.T, o order) string {
	t.Helper()
	require.NotNil(t, o.PaymentURL, "the order has a payment link")
	const prefix = "https://checkout.wompi.co/l/"
	require.Contains(t, *o.PaymentURL, prefix)
	return (*o.PaymentURL)[len(prefix):]
}

// logOutput is the cloud's error log: silent unless ACCEPTANCE_LOGS=true.
func logOutput() io.Writer {
	if os.Getenv("ACCEPTANCE_LOGS") == "true" {
		return os.Stderr
	}
	return io.Discard
}
