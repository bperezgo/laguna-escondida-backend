package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/gin-gonic/gin"
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

// The two acceptance databases are created fresh and migrated once per package run, then
// truncated between tests (newRig) for isolation. They live on the same local Postgres
// the other integration tests use; only the database names differ.
const (
	cloudDBName = "laguna_accept_cloud"
	edgeDBName  = "laguna_accept_edge"
	testOrg     = "accept-test-org"
	testNodeKey = "accept-node-key"
)

var (
	cloudGDB      *gorm.DB
	edgeGDB       *gorm.DB
	migrationsDir string
)

// gateEnabled reports whether the acceptance suite should run. Like the repository
// integration tests, it is opt-in so a plain `go test ./...` stays fast and DB-free.
func gateEnabled() bool { return os.Getenv("RUN_ACCEPTANCE_TESTS") == "true" }

func TestMain(m *testing.M) {
	if !gateEnabled() {
		// Provisioning needs a database; skip it and let each test self-skip via newRig.
		os.Exit(m.Run())
	}
	if err := provision(); err != nil {
		fmt.Fprintln(os.Stderr, "acceptance DB provisioning failed:", err)
		fmt.Fprintln(os.Stderr, "ensure a local Postgres is reachable (DB_HOST/DB_PORT/DB_USER/DB_PASSWORD).")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// connParams is the local Postgres the acceptance DBs are created on.
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

	base := baseConnParams()
	for _, name := range []string{cloudDBName, edgeDBName} {
		if err = recreateDatabase(base, name); err != nil {
			return fmt.Errorf("recreate %s: %w", name, err)
		}
		if err = runMigrations(base, name); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
	}

	if cloudGDB, err = openGorm(base.gormDSN(cloudDBName)); err != nil {
		return fmt.Errorf("open cloud db: %w", err)
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

	// WITH (FORCE) terminates leftover connections from an aborted prior run (PG13+).
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

// truncateAll wipes every table except schema_migrations, restoring a clean slate
// without re-running migrations. CASCADE handles FK-linked child tables.
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

// rig is one in-process two-node deployment: a cloud sync API behind httptest and an
// edge whose real HTTP push/pull clients target it. Tests seed on one side, trigger a
// sync method, and assert on the other.
type rig struct {
	t   *testing.T
	ctx context.Context

	cloudDB *gorm.DB
	edgeDB  *gorm.DB

	cloudSync *service.SyncService                // cloud receive side (ApplyPush)
	cloudRef  *repository.SyncReferenceRepository // seed cloud reference data
	edgeRef   *repository.SyncReferenceRepository // read replicated edge reference data

	edgePull   *service.SyncPullService
	edgePush   *service.SyncPushService
	edgeOutbox ports.SyncOutboxRepository
	edgeUoW    ports.UnitOfWork

	identity dto.SyncIdentity
	server   *httptest.Server
}

// newRig skips when the gate is off, gives both DBs a clean slate, and wires a fresh
// two-node deployment for the test.
func newRig(t *testing.T) *rig {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs a local Postgres).")
	}

	truncateAll(t, cloudGDB)
	truncateAll(t, edgeGDB)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identity := dto.SyncIdentity{
		NodeID:      deriveNodeID(testOrg, "edge"),
		CloudNodeID: deriveNodeID(testOrg, "cloud"),
	}

	// --- cloud side: the sync aggregate served over a real HTTP boundary ---
	cloudUoW := postgres.NewUnitOfWork(cloudGDB)
	cloudInbox := repository.NewSyncInboxRepository(cloudGDB)
	cloudAppliers := map[dto.SyncEntityType]ports.SyncApplier{
		dto.SyncEntityOpenBill:      repository.NewOpenBillSyncApplier(cloudGDB),
		dto.SyncEntityPurchaseEntry: repository.NewPurchaseEntrySyncApplier(cloudGDB),
		dto.SyncEntityStock:         repository.NewStockSyncApplier(cloudGDB),
		dto.SyncEntityHistoricStock: repository.NewHistoricStockSyncApplier(cloudGDB),
	}
	cloudSync := service.NewSyncService(cloudUoW, cloudInbox, cloudAppliers, logger)
	cloudRef := repository.NewSyncReferenceRepository(cloudGDB)
	cloudRefService := service.NewSyncReferenceService(cloudRef, logger)
	syncHandler := handler.NewSyncHandler(cloudSync, cloudRefService)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	nodeAuth := handler.NodeAuthMiddleware(&config.Config{NodeSyncKey: testNodeKey})
	router.POST("/api/sync/push", nodeAuth, syncHandler.PushHandler)
	router.GET("/api/sync/pull", nodeAuth, syncHandler.PullHandler)
	server := httptest.NewServer(router)

	// --- edge side: real HTTP clients pointed at the cloud server ---
	httpClient := httpclient.NewClient(slog.New(slog.DiscardHandler))
	edgeUoW := postgres.NewUnitOfWork(edgeGDB)
	edgeOutbox := repository.NewSyncOutboxRepository(edgeGDB)
	edgeState := repository.NewSyncStateRepository(edgeGDB)
	edgeRef := repository.NewSyncReferenceRepository(edgeGDB)

	pullClient := httpclient.NewSyncPullClient(httpClient, server.URL, testNodeKey)
	edgePull := service.NewSyncPullService(edgeUoW, pullClient, edgeRef, edgeState, identity, logger)

	pushClient := httpclient.NewSyncPushClient(httpClient, server.URL, testNodeKey)
	edgePush := service.NewSyncPushService(edgeUoW, edgeOutbox, edgeState, pushClient, identity, 0, logger)

	t.Cleanup(server.Close)

	return &rig{
		t:          t,
		ctx:        context.Background(),
		cloudDB:    cloudGDB,
		edgeDB:     edgeGDB,
		cloudSync:  cloudSync,
		cloudRef:   cloudRef,
		edgeRef:    edgeRef,
		edgePull:   edgePull,
		edgePush:   edgePush,
		edgeOutbox: edgeOutbox,
		edgeUoW:    edgeUoW,
		identity:   identity,
		server:     server,
	}
}

// deriveNodeID mirrors config.NewConfig: a stable per-organization/per-mode UUID, so the
// values the rig uses are exactly the ones a real deployment would derive (SYNC-INV-01).
func deriveNodeID(org, mode string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("laguna-escondida/node/"+org+"/"+mode)).String()
}

// ---------------------------------------------------------------------------
// Sync triggers (synchronous — the cron jobs are thin wrappers around these).
// ---------------------------------------------------------------------------

func (r *rig) pull() *dto.SyncPullResult {
	r.t.Helper()
	res, err := r.edgePull.PullChanges(r.ctx)
	require.NoError(r.t, err, "edge pull")
	return res
}

func (r *rig) push() *dto.SyncPushResult {
	r.t.Helper()
	res, err := r.edgePush.PushPending(r.ctx)
	require.NoError(r.t, err, "edge push")
	return res
}

func (r *rig) cloudApply(req *dto.SyncPushRequest) *dto.SyncPushResponse {
	r.t.Helper()
	resp, err := r.cloudSync.ApplyPush(r.ctx, req)
	require.NoError(r.t, err, "cloud apply push")
	return resp
}

// ---------------------------------------------------------------------------
// Seeding cloud reference data (cloud → edge pull direction).
// ---------------------------------------------------------------------------

func (r *rig) seedCloudProducts(products ...dto.ProductSyncPayload) {
	r.t.Helper()
	require.NoError(r.t, r.cloudRef.UpsertProducts(r.ctx, products), "seed cloud products")
}

func (r *rig) seedCloudUsers(users ...dto.UserSyncPayload) {
	r.t.Helper()
	require.NoError(r.t, r.cloudRef.UpsertUsers(r.ctx, users), "seed cloud users")
}

func (r *rig) seedCloudProductResponsibilities(responsibilities ...dto.ProductResponsibilitySyncPayload) {
	r.t.Helper()
	require.NoError(r.t, r.cloudRef.UpsertProductResponsibilities(r.ctx, responsibilities), "seed cloud product responsibilities")
}

// ---------------------------------------------------------------------------
// Reading replicated edge reference data.
// ---------------------------------------------------------------------------

func (r *rig) edgeProductByID(id string) (dto.ProductSyncPayload, bool) {
	r.t.Helper()
	rows, err := r.edgeRef.FindChangedProducts(r.ctx, time.Time{})
	require.NoError(r.t, err, "read edge products")
	for _, p := range rows {
		if p.ID == id {
			return p, true
		}
	}
	return dto.ProductSyncPayload{}, false
}

func (r *rig) edgeUserByID(id string) (dto.UserSyncPayload, bool) {
	r.t.Helper()
	rows, err := r.edgeRef.FindChangedUsers(r.ctx, time.Time{})
	require.NoError(r.t, err, "read edge users")
	for _, u := range rows {
		if u.ID == id {
			return u, true
		}
	}
	return dto.UserSyncPayload{}, false
}

func (r *rig) edgeProductResponsibilityByID(id string) (dto.ProductResponsibilitySyncPayload, bool) {
	r.t.Helper()
	rows, err := r.edgeRef.FindChangedProductResponsibilities(r.ctx, time.Time{})
	require.NoError(r.t, err, "read edge product responsibilities")
	for _, resp := range rows {
		if resp.ID == id {
			return resp, true
		}
	}
	return dto.ProductResponsibilitySyncPayload{}, false
}

func (r *rig) edgePulledCursor() *time.Time {
	r.t.Helper()
	stateRepo := repository.NewSyncStateRepository(r.edgeDB)
	cursor, err := stateRepo.GetPulledCursor(r.ctx, r.identity.CloudNodeID)
	require.NoError(r.t, err, "read edge pulled cursor")
	return cursor
}

// ---------------------------------------------------------------------------
// Edge outbox seeding + push-side assertions (edge → cloud push direction).
// ---------------------------------------------------------------------------

// appendEdgeOutbox writes an outbox row inside a unit-of-work transaction, exactly as a
// business service would. Append assigns seq/created_at; the caller owns op_id.
func (r *rig) appendEdgeOutbox(entry *dto.SyncOutboxEntry) {
	r.t.Helper()
	require.NoError(r.t, r.edgeUoW.Do(r.ctx, func(ctx context.Context) error {
		return r.edgeOutbox.Append(ctx, entry)
	}), "append edge outbox")
}

func (r *rig) cloudCount(table, where string, args ...any) int64 {
	r.t.Helper()
	var n int64
	require.NoError(r.t, r.cloudDB.Table(table).Where(where, args...).Count(&n).Error, "count "+table)
	return n
}

func (r *rig) edgeCount(table, where string, args ...any) int64 {
	r.t.Helper()
	var n int64
	require.NoError(r.t, r.edgeDB.Table(table).Where(where, args...).Count(&n).Error, "count "+table)
	return n
}

func (r *rig) edgePushedSeq() int64 {
	r.t.Helper()
	var seq int64
	require.NoError(r.t, r.edgeDB.Table("sync_state").
		Where("peer_node_id = ?", r.identity.CloudNodeID).
		Select("COALESCE(last_pushed_seq, 0)").Scan(&seq).Error, "read last_pushed_seq")
	return seq
}

// ---------------------------------------------------------------------------
// Builders for payloads with valid, schema-tracking defaults.
// ---------------------------------------------------------------------------

func newProduct(sku, name string, changedAt time.Time) dto.ProductSyncPayload {
	return dto.ProductSyncPayload{
		ID:                  uuid.NewString(),
		Name:                name,
		Category:            "bebidas",
		ProductType:         "SELLABLE",
		UnitOfMeasure:       "unit",
		Version:             1,
		UnitPrice:           decimal.NewFromInt(5000),
		VAT:                 decimal.Zero,
		VATAmount:           decimal.Zero,
		ICO:                 decimal.Zero,
		ICOAmount:           decimal.Zero,
		SKU:                 sku,
		TotalPriceWithTaxes: decimal.NewFromInt(5000),
		CreatedAt:           changedAt,
		UpdatedAt:           changedAt,
	}
}

func newProductResponsibility(productID, area string, priority int, changedAt time.Time) dto.ProductResponsibilitySyncPayload {
	return dto.ProductResponsibilitySyncPayload{
		ID:        uuid.NewString(),
		ProductID: productID,
		Area:      area,
		Priority:  priority,
		CreatedAt: changedAt,
		UpdatedAt: changedAt,
	}
}

func newUser(username, passwordHash string, changedAt time.Time) dto.UserSyncPayload {
	return dto.UserSyncPayload{
		ID:        uuid.NewString(),
		Username:  username,
		Name:      "Test " + username,
		Password:  passwordHash,
		CreatedAt: changedAt,
		UpdatedAt: changedAt,
	}
}

// openBillOutboxEntry builds a create-op outbox row for an order whose payload references
// the given (cloud-resident) user and product, so the cloud applier's FKs resolve.
func (r *rig) openBillOutboxEntry(createdByID, productID string) (entry *dto.SyncOutboxEntry, orderID string) {
	r.t.Helper()
	orderID = uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	payload := dto.OpenBillSyncPayload{
		ID:                 orderID,
		TemporalIdentifier: uuid.NewString(),
		TotalAmount:        decimal.NewFromInt(5000),
		Status:             dto.CommandStatusCreated,
		CreatedByID:        createdByID,
		Products: []dto.OpenBillSyncProduct{
			{OpenBillProductID: uuid.NewString(), ProductID: productID, Quantity: 1, Status: dto.CommandStatusCreated},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	raw, err := json.Marshal(payload)
	require.NoError(r.t, err, "marshal open_bill payload")
	return &dto.SyncOutboxEntry{
		OpID:         uuid.NewString(),
		OriginNodeID: r.identity.NodeID,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     orderID,
		Operation:    dto.SyncOperationCreate,
		Payload:      raw,
	}, orderID
}

// stockOutboxEntry builds a stock outbox row for the given (cloud-resident) product. A
// create/update carries the amount snapshot; a delete carries a tombstone keyed by product_id.
func (r *rig) stockOutboxEntry(productID string, version, amount int, op dto.SyncOperation) *dto.SyncOutboxEntry {
	r.t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)

	var raw json.RawMessage
	if op == dto.SyncOperationDelete {
		b, err := json.Marshal(dto.SyncTombstone{ID: productID})
		require.NoError(r.t, err, "marshal stock tombstone")
		raw = b
	} else {
		b, err := json.Marshal(dto.StockSyncPayload{
			ProductID:     productID,
			Version:       version,
			Amount:        amount,
			UnitOfMeasure: "unit",
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		require.NoError(r.t, err, "marshal stock payload")
		raw = b
	}

	return &dto.SyncOutboxEntry{
		OpID:         uuid.NewString(),
		OriginNodeID: r.identity.NodeID,
		EntityType:   dto.SyncEntityStock,
		EntityID:     productID,
		Operation:    op,
		Payload:      raw,
	}
}

// historicStockOutboxEntry builds an append-only historic_stock create op for the given
// (cloud-resident) product. The row's op_id doubles as the sync op id (1:1), matching how the
// stock service produces it.
func (r *rig) historicStockOutboxEntry(productID string, change int) *dto.SyncOutboxEntry {
	r.t.Helper()
	opID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	b, err := json.Marshal(dto.HistoricStockSyncPayload{
		OpID:          opID,
		ProductID:     productID,
		UnitOfMeasure: "unit",
		Change:        change,
		CreatedAt:     now,
	})
	require.NoError(r.t, err, "marshal historic_stock payload")
	return &dto.SyncOutboxEntry{
		OpID:         opID,
		OriginNodeID: r.identity.NodeID,
		EntityType:   dto.SyncEntityHistoricStock,
		EntityID:     opID,
		Operation:    dto.SyncOperationCreate,
		Payload:      b,
	}
}

// cloudStockAmount reads the mirrored on-hand amount for a product from the cloud stock
// table (live rows only). Returns 0 when no live row exists.
func (r *rig) cloudStockAmount(productID string) int {
	r.t.Helper()
	var amount int
	require.NoError(r.t, r.cloudDB.Table("stock").
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Select("amount").Scan(&amount).Error, "read cloud stock amount")
	return amount
}

// envOr returns the env var or a fallback (kept local so the package needs no shared helper).
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv loads the repo root .env if present, so DB_* overrides are picked up.
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
