package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"laguna-escondida/backend/test/acceptance/testsupport"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Migration 000057 renames product_ingredients.quantity -> default_quantity and adds the
// side-dish columns. This suite proves, end to end against a real Postgres, that the up
// migration preserves existing recipe amounts and the down migration reverses it.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const (
	dbName             = "laguna_accept_migration"
	sideDishMigration  = 57
	preSideDishVersion = 56
)

func gateEnabled() bool { return os.Getenv("RUN_ACCEPTANCE_TESTS") == "true" }

type connParams struct {
	host, port, user, password, sslmode string
}

func (p connParams) gormDSN(name string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.host, p.port, p.user, p.password, name, p.sslmode)
}

func (p connParams) migrateURL(name string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.user, p.password, p.host, p.port, name, p.sslmode)
}

func TestMigration_SideDishRename_PreservesAmounts(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	recreateDatabase(t, base, dbName)
	migrationsDir := findMigrationsDir(t)

	m, err := migrate.New("file://"+migrationsDir, base.migrateURL(dbName))
	require.NoError(t, err, "open migrator")
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Migrate(preSideDishVersion), "migrate up to pre-side-dish schema")

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)

	compositeID := uuid.NewString()
	ingredientID := uuid.NewString()
	recipeID := uuid.NewString()
	seedProduct(t, db, compositeID, "Plate", "COMPOSITE")
	seedProduct(t, db, ingredientID, "Salad", "INGREDIENT")

	require.NoError(t, db.Exec(
		`INSERT INTO product_ingredients (id, composite_product_id, ingredient_product_id, quantity) VALUES (?, ?, ?, ?)`,
		recipeID, compositeID, ingredientID, decimal.RequireFromString("3.5"),
	).Error, "seed legacy recipe row")

	require.NoError(t, m.Migrate(sideDishMigration), "migrate up through side-dish migration")

	var after struct {
		DefaultQuantity decimal.Decimal `gorm:"column:default_quantity"`
		IsSideDish      bool            `gorm:"column:is_side_dish"`
		MinQuantity     int             `gorm:"column:min_quantity"`
		MaxQuantity     int             `gorm:"column:max_quantity"`
	}
	require.NoError(t, db.Raw(
		`SELECT default_quantity, is_side_dish, min_quantity, max_quantity FROM product_ingredients WHERE id = ?`,
		recipeID,
	).Scan(&after).Error, "read migrated row")

	require.True(t, after.DefaultQuantity.Equal(decimal.RequireFromString("3.5")),
		"existing amount must carry over into default_quantity, got %s", after.DefaultQuantity)
	require.False(t, after.IsSideDish, "existing rows default to fixed ingredients")
	require.Equal(t, 0, after.MinQuantity)
	require.Equal(t, 0, after.MaxQuantity)

	require.NoError(t, m.Steps(-1), "migrate down the side-dish migration")

	var restored decimal.Decimal
	require.NoError(t, db.Raw(
		`SELECT quantity FROM product_ingredients WHERE id = ?`, recipeID,
	).Scan(&restored).Error, "read rolled-back row")
	require.True(t, restored.Equal(decimal.RequireFromString("3.5")),
		"amount must survive the rollback under the restored quantity column, got %s", restored)
}

func resolveConn(t *testing.T, ctx context.Context) (connParams, *testsupport.PostgresContainer) {
	t.Helper()
	if os.Getenv("DB_HOST") != "" {
		return connParams{
			host:     os.Getenv("DB_HOST"),
			port:     envOr("DB_PORT", "5432"),
			user:     envOr("DB_USER", "postgres"),
			password: envOr("DB_PASSWORD", "postgres"),
			sslmode:  envOr("DB_SSLMODE", "disable"),
		}, nil
	}
	ctr, err := testsupport.StartPostgres(ctx)
	require.NoError(t, err, "boot postgres testcontainer (is Docker running?)")
	return connParams{
		host: ctr.Host, port: ctr.Port, user: ctr.User,
		password: ctr.Password, sslmode: ctr.SSLMode,
	}, ctr
}

func recreateDatabase(t *testing.T, base connParams, name string) {
	t.Helper()
	admin := openGorm(t, base.gormDSN("postgres"))
	defer closeGorm(admin)
	require.NoError(t, admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)).Error, "drop db")
	require.NoError(t, admin.Exec(fmt.Sprintf("CREATE DATABASE %s", name)).Error, "create db")
}

func seedProduct(t *testing.T, db *gorm.DB, id, name, productType string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO products (id, name, category, product_type, unit_of_measure, version, unit_price, vat, vat_amount, ico, ico_amount, sku, total_price_with_taxes)
		 VALUES (?, ?, 'acceptance', ?, 'g', 1, 1000, 0, 0, 0, 0, ?, 1000)`,
		id, name, productType, "SKU-"+id,
	).Error, "seed product")
}

func openGorm(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormpg.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err, "open gorm")
	return db
}

func closeGorm(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return filepath.Join(dir, "internal", "platform", "postgres", "migrations")
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found")
		dir = parent
	}
}
