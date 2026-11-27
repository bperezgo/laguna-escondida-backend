// Package signup drives self-serve signup, plans, the trial, invitations and enrollment codes
// end to end through the real cloud assembly, with Google's token check stubbed.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/signup/...
package signup

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
	"strings"
	"testing"

	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/pkg/eventbus"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/stretchr/testify/require"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	dbName      = "laguna_accept_signup"
	adminAPIKey = "accept-admin-key"
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

// fakeGoogle accepts a token "google:<email>:<name>", as if Google had signed it with a verified email.
type fakeGoogle struct{}

func (fakeGoogle) Verify(_ context.Context, idToken string) (*dto.GoogleIdentity, error) {
	parts := strings.SplitN(idToken, ":", 3)
	if len(parts) != 3 || parts[0] != "google" {
		return nil, domainError.ErrGoogleTokenInvalid
	}
	return &dto.GoogleIdentity{Email: parts[1], Name: parts[2], Subject: parts[1]}, nil
}

func googleToken(email, name string) string { return "google:" + email + ":" + name }

type cloud struct {
	t      *testing.T
	server *httptest.Server
}

func newCloud(t *testing.T) *cloud {
	t.Helper()
	return newCloudWith(t, testsupport.PaymentLinks(t), config.Billing{})
}

// newCloudWithBilling is a cloud with the platform's billing keys, so signup asks for a card.
func newCloudWithBilling(t *testing.T) (*cloud, *testsupport.FakeBillingWompi) {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
	wompi := testsupport.BillingWompi(t)
	return newCloudWith(t, wompi.PaymentLinks(), wompi.Billing()), wompi
}

func newCloudWith(t *testing.T, paymentLinks config.PaymentLinks, billing config.Billing) *cloud {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	t.Cleanup(func() { _ = bus.Close() })
	cfg := &config.Cloud{AdminAPIKey: adminAPIKey, PublicAPIURL: "http://localhost", Shared: config.Shared{
		JWTSecret: "accept-secret", PhoneHomeRegion: "CO",
		Observability: config.Observability{ServiceName: "laguna-backend", MetricsPort: "0"},
	}, PaymentLinks: paymentLinks, Integrations: testsupport.Integrations(), Billing: billing,
		Signup: config.Signup{Enabled: true, GoogleClientID: "accept.apps.googleusercontent.com", PublicAppURL: "https://app.example.com"}}
	app, err := bootstrap.NewCloud(cfg, bootstrap.Infra{
		DB: cloudDB, Logger: logger, HTTPClient: httpclient.NewClient(logger),
		Storage: mocks.NewMockStorageClient(t), ElectronicInvoice: mocks.NewMockElectronicInvoiceClient(t),
		EventBus: bus, GoogleVerifier: fakeGoogle{},
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

func (r response) code(t *testing.T) string {
	t.Helper()
	var b struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.body, &b)
	return b.Code
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

func (c *cloud) as(token, method, path string, body any) response {
	c.t.Helper()
	return c.call(method, path, map[string]string{"Authorization": "Bearer " + token}, body)
}

func (c *cloud) operator(method, path string, body any) response {
	c.t.Helper()
	return c.call(method, path, map[string]string{"X-API-Key": adminAPIKey}, body)
}

func (c *cloud) node(nodeID, key, method, path string, body any) response {
	c.t.Helper()
	return c.call(method, path, map[string]string{"X-Node-ID": nodeID, "X-Node-Key": key}, body)
}
