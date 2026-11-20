// Package deliveries drives delivery tracking on Postgres: the repository's conditional updates
// and reads, and the cloud's delivery routes end to end.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/deliveries/...
package deliveries

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/test/acceptance/testsupport"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const dbName = "laguna_accept_deliveries"

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

// restaurant is an organization with one product and the context that reaches its rows.
type restaurant struct {
	t         *testing.T
	orgID     string
	productID string
	ctx       context.Context
}

func seedRestaurant(t *testing.T) *restaurant {
	t.Helper()
	skipUnlessEnabled(t)
	id := uuid.NewString()
	require.NoError(t, cloudDB.Exec("INSERT INTO organizations (id, name, slug, status) VALUES (?, ?, ?, 'active')",
		id, "Restaurant "+id[:8], "r-"+id[:8]).Error)
	r := &restaurant{t: t, orgID: id, ctx: tenant.WithOrg(context.Background(), id)}
	r.productID = r.seedProduct("Mojarra frita")
	return r
}

func (r *restaurant) seedProduct(name string) string {
	r.t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.NewString()
	require.NoError(r.t, repository.NewSyncReferenceRepository(cloudDB).UpsertProducts(r.ctx, []dto.ProductSyncPayload{{
		ID: id, Name: name, Category: "platos", ProductType: "SELLABLE", UnitOfMeasure: "unit", Version: 1,
		UnitPrice: decimal.NewFromInt(45000), VAT: decimal.Zero, VATAmount: decimal.Zero, ICO: decimal.Zero, ICOAmount: decimal.Zero,
		SKU: "SKU-" + id[:8], TotalPriceWithTaxes: decimal.NewFromInt(45000), CreatedAt: now, UpdatedAt: now,
	}}))
	return id
}

func (r *restaurant) seedUser(name string, roleIDs ...int) string {
	r.t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(r.t, cloudDB.Exec(`INSERT INTO users (id, organization_id, username, name, email, password, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'x', true, ?, ?)`, id, r.orgID, "user-"+id[:8], name, "u-"+id[:8]+"@example.com", now, now).Error)
	for _, roleID := range roleIDs {
		require.NoError(r.t, cloudDB.Exec(`INSERT INTO user_roles (user_id, role_id, organization_id, created_at) VALUES (?, ?, ?, ?)`,
			id, roleID, r.orgID, now).Error)
	}
	return id
}

type seededOrder struct {
	orderType   dto.OrderType
	paidAt      *time.Time
	voided      bool
	lineStatus  []dto.CommandStatus
	channelID   *string
	createdByID string
}

// seedOrder writes an order straight to the tables, as the order service would leave it.
func (r *restaurant) seedOrder(o seededOrder) string {
	r.t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	createdBy := o.createdByID
	if createdBy == "" {
		createdBy = r.seedUser("Cajero")
	}
	var deletedAt *time.Time
	if o.paidAt != nil || o.voided {
		deletedAt = &now
	}
	var address *string
	var lat, lng *decimal.Decimal
	if o.orderType == dto.OrderTypeDelivery {
		a := "Calle 10 # 5-20"
		la, ln := decimal.RequireFromString("4.711000"), decimal.RequireFromString("-74.072100")
		address, lat, lng = &a, &la, &ln
	}
	require.NoError(r.t, cloudDB.Exec(`INSERT INTO open_bills (id, organization_id, temporal_identifier, total_amount, status,
		created_by, created_at, updated_at, deleted_at, paid_at, order_type, customer_name, customer_phone,
		delivery_address, delivery_lat, delivery_lng, channel_id)
		VALUES (?, ?, ?, 45000, 'created', ?, ?, ?, ?, ?, ?, 'Ana', '+573005551234', ?, ?, ?, ?)`,
		id, r.orgID, uuid.NewString(), createdBy, now, now, deletedAt, o.paidAt, string(o.orderType),
		address, lat, lng, o.channelID).Error)
	for _, status := range o.lineStatus {
		require.NoError(r.t, cloudDB.Exec(`INSERT INTO open_bills_products (id, organization_id, open_bill_id, product_id,
			quantity, status, priority, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, 2, ?, 1, ?, ?, ?)`,
			uuid.NewString(), r.orgID, id, r.productID, string(status), createdBy, now, now).Error)
	}
	return id
}

func at(t time.Time) *time.Time { return &t }

func (r *restaurant) historyCount(orderID string) int64 {
	r.t.Helper()
	var n int64
	require.NoError(r.t, cloudDB.Raw("SELECT count(*) FROM order_delivery_events WHERE open_bill_id = ?", orderID).Scan(&n).Error)
	return n
}
