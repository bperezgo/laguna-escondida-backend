package composite

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/pkg/eventbus"
	"laguna-escondida/backend/test/acceptance/testsupport"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// BSP-18 composite-stock acceptance tests (AC1 and AC2, the [A] rows). They seed real
// products, recipe rows, and stock into a fresh edge Postgres, then drive the real
// StockEventHandler and assert leaf stock moves through multiple recipe levels — exactly
// what the unit tests assert, but end to end over SQL. Opt-in like the sync suite:
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/composite/...
//
// AC2 is expected RED until decreaseStockForProduct recurses into composite ingredients.

const (
	edgeDBName = "laguna_accept_composite"
	testNodeID = "11111111-1111-1111-1111-111111111111"
	gramUnit   = dto.UnitOfMeasureGram
)

var (
	edgeGDB       *gorm.DB
	migrationsDir string
	pgContainer   *testsupport.PostgresContainer
)

func gateEnabled() bool { return os.Getenv("RUN_ACCEPTANCE_TESTS") == "true" }

func TestMain(m *testing.M) {
	if !gateEnabled() {
		os.Exit(m.Run())
	}
	if err := provision(); err != nil {
		fmt.Fprintln(os.Stderr, "acceptance DB provisioning failed:", err)
		fmt.Fprintln(os.Stderr, "start Docker (testcontainers boots Postgres automatically), or set DB_HOST to point at an external Postgres.")
		os.Exit(1)
	}
	code := m.Run()
	teardown()
	os.Exit(code)
}

type connParams struct {
	host, port, user, password, sslmode string
}

func baseConnParams() connParams {
	return connParams{
		host:     envOr("DB_HOST", "localhost"),
		port:     envOr("DB_PORT", "5432"),
		user:     envOr("DB_USER", "postgres"),
		password: envOr("DB_PASSWORD", "postgres"),
		sslmode:  envOr("DB_SSLMODE", "disable"),
	}
}

// resolveBaseConnParams decides which Postgres the acceptance database runs on. When DB_HOST
// is set it uses that external server (e.g. a CI service container); otherwise it boots a
// throwaway postgres:16-alpine via testcontainers, so the suite needs no manually-started DB.
func resolveBaseConnParams() (connParams, error) {
	if os.Getenv("DB_HOST") != "" {
		return baseConnParams(), nil
	}
	ctr, err := testsupport.StartPostgres(context.Background())
	if err != nil {
		return connParams{}, fmt.Errorf("boot postgres testcontainer (is Docker running? set DB_HOST to use an external DB instead): %w", err)
	}
	pgContainer = ctr
	return connParams{
		host:     ctr.Host,
		port:     ctr.Port,
		user:     ctr.User,
		password: ctr.Password,
		sslmode:  ctr.SSLMode,
	}, nil
}

// teardown stops the throwaway Postgres container, if one was started.
func teardown() {
	if pgContainer != nil {
		_ = pgContainer.Terminate(context.Background())
	}
}

func (p connParams) gormDSN(dbName string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.host, p.port, p.user, p.password, dbName, p.sslmode)
}

func (p connParams) migrateURL(dbName string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.user, p.password, p.host, p.port, dbName, p.sslmode)
}

func provision() error {
	loadDotEnv()
	dir, err := findMigrationsDir()
	if err != nil {
		return err
	}
	migrationsDir = dir

	base, err := resolveBaseConnParams()
	if err != nil {
		return err
	}
	if err = recreateDatabase(base, edgeDBName); err != nil {
		return fmt.Errorf("recreate %s: %w", edgeDBName, err)
	}
	if err = runMigrations(base, edgeDBName); err != nil {
		return fmt.Errorf("migrate %s: %w", edgeDBName, err)
	}
	if edgeGDB, err = openGorm(base.gormDSN(edgeDBName)); err != nil {
		return fmt.Errorf("open edge db: %w", err)
	}
	return nil
}

func recreateDatabase(base connParams, name string) error {
	admin, err := openGorm(base.gormDSN("postgres"))
	if err != nil {
		return err
	}
	defer closeGorm(admin)

	if err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)).Error; err != nil {
		if err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)).Error; err != nil {
			return fmt.Errorf("drop: %w", err)
		}
	}
	if err := admin.Exec(fmt.Sprintf("CREATE DATABASE %s", name)).Error; err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

func runMigrations(base connParams, name string) error {
	m, err := migrate.New("file://"+migrationsDir, base.migrateURL(name))
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func openGorm(dsn string) (*gorm.DB, error) {
	return gorm.Open(gormpg.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
}

func closeGorm(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func truncateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	const stmt = `DO $$
DECLARE tables text;
BEGIN
  SELECT string_agg(quote_ident(tablename), ', ') INTO tables
  FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations';
  IF tables IS NOT NULL THEN
    EXECUTE 'TRUNCATE TABLE ' || tables || ' RESTART IDENTITY CASCADE';
  END IF;
END $$;`
	require.NoError(t, db.Exec(stmt).Error, "truncate all tables")
}

// edgeRig is one edge deployment: the real repositories and the real StockEventHandler
// wired against a fresh Postgres, so a seeded recipe decrements exactly as production would.
type edgeRig struct {
	t   *testing.T
	ctx context.Context

	handler        *service.StockEventHandler
	productRepo    ports.ProductRepository
	stockRepo      ports.StockRepository
	ingredientRepo ports.ProductIngredientRepository
}

func newEdge(t *testing.T) *edgeRig {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs a local Postgres).")
	}

	truncateAll(t, edgeGDB)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	productRepo := repository.NewProductRepository(edgeGDB)
	stockRepo := repository.NewStockRepository(edgeGDB)
	ingredientRepo := repository.NewProductIngredientRepository(edgeGDB)
	outbox := repository.NewSyncOutboxRepository(edgeGDB)
	uow := postgres.NewUnitOfWork(edgeGDB)
	lockManager := eventbus.NewProductLockManager()

	handler := service.NewStockEventHandler(
		stockRepo,
		productRepo,
		ingredientRepo,
		lockManager,
		uow,
		outbox,
		dto.SyncIdentity{NodeID: testNodeID},
		logger,
	)

	return &edgeRig{
		t:              t,
		ctx:            context.Background(),
		handler:        handler,
		productRepo:    productRepo,
		stockRepo:      stockRepo,
		ingredientRepo: ingredientRepo,
	}
}

// seedProduct inserts a product row of the given type, in grams (small units).
func (r *edgeRig) seedProduct(id, name string, productType dto.ProductType) {
	r.t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	agg, err := product.NewAggregateFromDTO(&dto.Product{
		ID:                  id,
		Name:                name,
		Category:            "acceptance",
		ProductType:         productType,
		UnitOfMeasure:       gramUnit,
		Version:             1,
		UnitPrice:           decimal.NewFromInt(1000),
		VAT:                 decimal.Zero,
		VATAmount:           decimal.Zero,
		ICO:                 decimal.Zero,
		ICOAmount:           decimal.Zero,
		SKU:                 "SKU-" + id,
		TotalPriceWithTaxes: decimal.NewFromInt(1000),
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	require.NoError(r.t, err, "build product aggregate")
	require.NoError(r.t, r.productRepo.Create(r.ctx, agg), "seed product")
}

// seedStock sets a product's initial on-hand amount.
func (r *edgeRig) seedStock(productID string, amount int) {
	r.t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(r.t, r.stockRepo.Create(r.ctx, &dto.Stock{
		ProductID:     productID,
		Version:       1,
		Amount:        amount,
		UnitOfMeasure: gramUnit,
		CreatedAt:     now,
		UpdatedAt:     now,
	}), "seed stock")
}

// seedRecipe adds one recipe row (compositeID consumes qty of ingredientID). It writes the
// row directly so the test does not depend on AddIngredient's guards.
func (r *edgeRig) seedRecipe(compositeID, ingredientID string, qty int) {
	r.t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(r.t, r.ingredientRepo.Create(r.ctx, &dto.ProductIngredient{
		ID:                  uuid.NewString(),
		CompositeProductID:  compositeID,
		IngredientProductID: ingredientID,
		DefaultQuantity:     decimal.NewFromInt(int64(qty)),
		CreatedAt:           now,
		UpdatedAt:           now,
	}), "seed recipe row")
}

// stockAmount returns the on-hand amount and whether a live stock row exists.
func (r *edgeRig) stockAmount(productID string) (int, bool) {
	r.t.Helper()
	s, err := r.stockRepo.FindByProductID(r.ctx, productID)
	if err != nil {
		return 0, false
	}
	return s.Amount, true
}

func (r *edgeRig) sell(compositeID string, quantity int) {
	r.t.Helper()
	require.NoError(r.t, r.handler.HandleOrderCreated(r.ctx, dto.OrderCreatedEvent{
		OpenBillID: uuid.NewString(),
		Products:   []dto.OrderCreatedEventProduct{{ProductID: compositeID, Quantity: quantity}},
	}), "handle order created")
}

// AC1 [A]: Single-level decrement (regression), end to end.
func TestAcceptance_SingleLevelComposite_AC1(t *testing.T) {
	r := newEdge(t)

	cID := uuid.NewString()
	iID := uuid.NewString()
	r.seedProduct(cID, "Blackberry Juice", dto.ProductTypeComposite)
	r.seedProduct(iID, "Blackberry", dto.ProductTypeIngredient)
	r.seedStock(iID, 100)
	r.seedRecipe(cID, iID, 3)

	r.sell(cID, 2)

	amt, ok := r.stockAmount(iID)
	require.True(t, ok, "ingredient must have a stock row")
	require.Equal(t, 94, amt, "ingredient drops by 3×2") // 100 - 6

	_, hasComposite := r.stockAmount(cID)
	require.False(t, hasComposite, "composite must not carry its own stock row")
}

// AC2 [A]: Two-level decrement, end to end. Expected RED until multi-level recursion lands.
func TestAcceptance_TwoLevelComposite_AC2(t *testing.T) {
	r := newEdge(t)

	cID := uuid.NewString()
	bID := uuid.NewString()
	lID := uuid.NewString()
	r.seedProduct(cID, "Combo", dto.ProductTypeComposite)
	r.seedProduct(bID, "Sub Combo", dto.ProductTypeComposite)
	r.seedProduct(lID, "Rice", dto.ProductTypeIngredient)
	r.seedStock(lID, 100)
	r.seedRecipe(cID, bID, 2)
	r.seedRecipe(bID, lID, 5)

	r.sell(cID, 4)

	amt, ok := r.stockAmount(lID)
	require.True(t, ok, "leaf ingredient must have a stock row")
	require.Equal(t, 60, amt, "leaf drops by 2×5×4") // 100 - 40

	_, hasB := r.stockAmount(bID)
	require.False(t, hasB, "intermediate composite must not carry its own stock row")
	_, hasC := r.stockAmount(cID)
	require.False(t, hasC, "top composite must not carry its own stock row")
}

// SideDish round-trip: the real repository must persist and read back the side-dish fields
// (is_side_dish, min/max, default_quantity) added by migration 000057.
func TestAcceptance_SideDishOption_RoundTrip(t *testing.T) {
	r := newEdge(t)

	cID := uuid.NewString()
	saladID := uuid.NewString()
	proteinID := uuid.NewString()
	r.seedProduct(cID, "Plate", dto.ProductTypeComposite)
	r.seedProduct(saladID, "Salad", dto.ProductTypeIngredient)
	r.seedProduct(proteinID, "Protein", dto.ProductTypeIngredient)

	now := time.Now().UTC().Truncate(time.Microsecond)
	sideDishID := uuid.NewString()
	require.NoError(t, r.ingredientRepo.Create(r.ctx, &dto.ProductIngredient{
		ID:                  sideDishID,
		CompositeProductID:  cID,
		IngredientProductID: saladID,
		DefaultQuantity:     decimal.NewFromInt(1),
		IsSideDish:          true,
		MinQuantity:         0,
		MaxQuantity:         2,
		CreatedAt:           now,
		UpdatedAt:           now,
	}), "create side-dish row")
	require.NoError(t, r.ingredientRepo.Create(r.ctx, &dto.ProductIngredient{
		ID:                  uuid.NewString(),
		CompositeProductID:  cID,
		IngredientProductID: proteinID,
		DefaultQuantity:     decimal.NewFromInt(1),
		CreatedAt:           now,
		UpdatedAt:           now,
	}), "create fixed row")

	got, err := r.ingredientRepo.FindByID(r.ctx, sideDishID)
	require.NoError(t, err)
	require.True(t, got.IsSideDish)
	require.Equal(t, 0, got.MinQuantity)
	require.Equal(t, 2, got.MaxQuantity)
	require.True(t, got.DefaultQuantity.Equal(decimal.NewFromInt(1)))

	sideDishes, err := r.ingredientRepo.FindSideDishesByCompositeProductID(r.ctx, cID)
	require.NoError(t, err)
	require.Len(t, sideDishes, 1, "only the side-dish row is returned, not the fixed protein")
	require.Equal(t, saladID, sideDishes[0].IngredientProductID)
}

// Order-line side dishes must survive a persist → read round trip through the real
// open_bill repository (both the DTO reader and the aggregate reader).
func TestAcceptance_OpenBillSideDishes_RoundTrip(t *testing.T) {
	r := newEdge(t)
	openBillRepo := repository.NewOpenBillRepository(edgeGDB)

	userID := uuid.NewString()
	require.NoError(t, edgeGDB.Exec(
		`INSERT INTO users (id, username, password, name) VALUES (?, ?, 'x', 'Server')`,
		userID, "server-"+userID[:8],
	).Error, "seed user")

	plateID := uuid.NewString()
	saladID := uuid.NewString()
	canastaID := uuid.NewString()
	r.seedProduct(plateID, "Plate", dto.ProductTypeComposite)
	r.seedProduct(saladID, "Salad", dto.ProductTypeIngredient)
	r.seedProduct(canastaID, "Canasta", dto.ProductTypeIngredient)

	lineID := uuid.NewString()
	line, err := openBill.NewOpenBillProduct(lineID, plateID, 1, nil, nil, 0, userID)
	require.NoError(t, err)
	line.SetSideDishes([]dto.SideDishSelection{
		{IngredientProductID: saladID, Quantity: 0},
		{IngredientProductID: canastaID, Quantity: 3},
	})

	billID := uuid.NewString()
	aggregate, err := openBill.NewAggregate(billID, uuid.NewString(), nil, decimal.NewFromInt(1000), []*openBill.OpenBillProduct{line}, userID)
	require.NoError(t, err)
	require.NoError(t, openBillRepo.Create(r.ctx, aggregate), "persist open bill with side dishes")

	withProducts, err := openBillRepo.FindByIDWithProducts(r.ctx, billID)
	require.NoError(t, err)
	require.Len(t, withProducts.Products, 1)
	require.Len(t, withProducts.Products[0].SideDishes, 2)
	byIngredient := map[string]int{}
	for _, sd := range withProducts.Products[0].SideDishes {
		byIngredient[sd.IngredientProductID] = sd.Quantity
	}
	require.Equal(t, 0, byIngredient[saladID])
	require.Equal(t, 3, byIngredient[canastaID])

	readAggregate, err := openBillRepo.FindAggregateByID(r.ctx, billID)
	require.NoError(t, err)
	require.Len(t, readAggregate.Products(), 1)
	require.Len(t, readAggregate.Products()[0].SideDishes(), 2, "aggregate reader also carries side dishes")
}

// envOr returns the env var or a fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv() {
	if root, err := findRepoRoot(); err == nil {
		_ = godotenv.Load(filepath.Join(root, ".env"))
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func findMigrationsDir() (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "internal", "platform", "postgres", "migrations")
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("migrations dir not found at %s: %w", dir, err)
	}
	return dir, nil
}
