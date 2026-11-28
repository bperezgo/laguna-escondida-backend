package catalogimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/internal/platform/stockmovement"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/gin-gonic/gin"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Catalog import end to end against a migrated cloud database, over the same HTTP chain the
// cloud serves (JWT auth, catalog:import, DB session). Opt-in like the other suites:
//
//	DB_HOST= RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/catalogimport/...

const cloudDBName = "laguna_accept_catalog_import"

var (
	cloudGDB    *gorm.DB
	pgContainer *testsupport.PostgresContainer
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
	host, port := envOr("DB_HOST", ""), envOr("DB_PORT", "5432")
	user, password, sslmode := envOr("DB_USER", "postgres"), envOr("DB_PASSWORD", "postgres"), envOr("DB_SSLMODE", "disable")
	if host == "" {
		ctr, err := testsupport.StartPostgres(context.Background())
		if err != nil {
			return fmt.Errorf("boot postgres testcontainer: %w", err)
		}
		pgContainer = ctr
		host, port, user, password, sslmode = ctr.Host, ctr.Port, ctr.User, ctr.Password, ctr.SSLMode
	}
	dsn := func(db string) string {
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, db, sslmode)
	}
	admin, err := gorm.Open(gormpg.Open(dsn("postgres")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return err
	}
	if err = admin.Exec("DROP DATABASE IF EXISTS " + cloudDBName + " WITH (FORCE)").Error; err != nil {
		return err
	}
	if err = admin.Exec("CREATE DATABASE " + cloudDBName).Error; err != nil {
		return err
	}
	if sqlDB, dbErr := admin.DB(); dbErr == nil {
		_ = sqlDB.Close()
	}
	m, err := testsupport.NewMigrator(migrations.Cloud, fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, cloudDBName, sslmode))
	if err != nil {
		return err
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	_, _ = m.Close()
	cloudGDB, err = gorm.Open(gormpg.Open(dsn(cloudDBName)), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	return err
}

type rig struct {
	t      *testing.T
	router *gin.Engine
	jwt    *service.JWTService
	orgA   string
	orgB   string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
	r := &rig{t: t, jwt: service.NewJWTService("test-secret", tenant.AnyOrganization()), orgA: uuid.NewString(), orgB: uuid.NewString()}
	for _, org := range []string{r.orgA, r.orgB} {
		require.NoError(t, cloudGDB.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, ?, ?)`, org, "Org "+org[:8], "org-"+org[:8]).Error)
	}

	productRepo := repository.NewProductRepository(cloudGDB)
	imports := service.NewCatalogImportService(
		productRepo, repository.NewProductIngredientRepository(cloudGDB), repository.NewPlateOptionSlotRepository(cloudGDB),
		repository.NewStockRepository(cloudGDB), postgres.NewUnitOfWork(cloudGDB), stockmovement.NewNoopEmitter(),
	)
	gin.SetMode(gin.TestMode)
	r.router = gin.New()
	r.router.Use(handler.DBSessionMiddleware(cloudGDB))
	h := handler.NewCatalogImportHandler(imports)
	g := r.router.Group("/api/catalog/import", handler.JWTAuthMiddleware(r.jwt), handler.RequirePermission(permissions.CatalogImport))
	g.POST("/validate", h.ValidateHandler)
	g.POST("", h.ApplyHandler)
	return r
}

func (r *rig) post(orgID, path string, body []byte) (int, []byte) {
	r.t.Helper()
	token, err := r.jwt.GenerateToken(uuid.NewString(), orgID, "admin", []int{permissions.RoleAdmin})
	require.NoError(r.t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func (r *rig) validate(orgID string, doc []byte) dto.CatalogImportResult {
	r.t.Helper()
	status, body := r.post(orgID, "/api/catalog/import/validate", doc)
	require.Equal(r.t, http.StatusOK, status, string(body))
	var res dto.CatalogImportResult
	require.NoError(r.t, json.Unmarshal(body, &res))
	return res
}

func withDigest(doc []byte, digest string) []byte {
	return bytes.Replace(doc, []byte("{"), []byte(`{"expected_digest": "`+digest+`",`), 1)
}

func (r *rig) apply(orgID string, doc []byte, digest string) (int, dto.CatalogImportResult, []byte) {
	r.t.Helper()
	status, body := r.post(orgID, "/api/catalog/import", withDigest(doc, digest))
	var res dto.CatalogImportResult
	require.NoError(r.t, json.Unmarshal(body, &res), string(body))
	return status, res, body
}

func (r *rig) count(table, orgID, where string, args ...any) int64 {
	r.t.Helper()
	var n int64
	q := cloudGDB.Table(table).Where("organization_id = ?", orgID)
	if where != "" {
		q = q.Where(where, args...)
	}
	require.NoError(r.t, q.Count(&n).Error)
	return n
}

func (r *rig) ctx(orgID string) context.Context {
	return tenant.WithOrg(context.Background(), orgID)
}

const fullDoc = `{
	"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0},
	"plates": [
		{"ref": "tilapia", "name": "Tilapia frita", "category": "Platos", "unit_of_measure": "unit", "total_price_with_taxes": 32000,
		 "recipe": [{"ingredient": "tilapia-cruda", "quantity": 1}],
		 "slots": [{"name": "Arroz", "choices": [
			{"name": "Arroz blanco", "default": true, "items": [{"ingredient": "arroz-blanco", "quantity": 200}]},
			{"name": "Arroz de coco", "items": [{"ingredient": "arroz-coco", "quantity": 200}]},
			{"name": "Sin arroz"}]}]}
	],
	"ingredients": [
		{"ref": "arroz-blanco", "name": "Arroz blanco", "category": "Insumos"},
		{"ref": "arroz-coco", "name": "Arroz de coco", "category": "Insumos"},
		{"ref": "tilapia-cruda", "name": "Tilapia entera", "category": "Insumos", "unit_of_measure": "unit"}
	],
	"sellables": [
		{"ref": "coca", "name": "Coca-Cola 400ml", "category": "Bebidas", "unit_of_measure": "unit", "total_price_with_taxes": 5000, "prep": {"area": "bar"}},
		{"ref": "agua", "name": "Agua 600ml", "category": "Bebidas", "unit_of_measure": "unit", "total_price_with_taxes": 3000, "prep": {"area": "bar"}}
	],
	"opening_stock": [{"product": "arroz-blanco", "counted": 25000}]
}`

func TestAcceptance_CatalogImport_FullDocumentValidatedThenApplied(t *testing.T) {
	r := newRig(t)
	doc := []byte(fullDoc)

	validation := r.validate(r.orgA, doc)
	require.True(t, validation.Valid, "%+v", validation.Problems)
	assert.Zero(t, r.count("products", r.orgA, ""), "validation writes nothing")

	status, applied, body := r.apply(r.orgA, doc, validation.Digest)

	require.Equal(t, http.StatusCreated, status, string(body))
	require.Len(t, applied.Products, 6)
	assert.Equal(t, int64(6), r.count("products", r.orgA, "deleted_at IS NULL"))
	ids := func(ref string) string { return applied.Products[ref].ID }

	assert.Equal(t, int64(1), r.count("product_ingredients", r.orgA, "composite_product_id = ? AND ingredient_product_id = ?", ids("tilapia"), ids("tilapia-cruda")))

	slots, err := repository.NewPlateOptionSlotRepository(cloudGDB).FindByCompositeProductID(r.ctx(r.orgA), ids("tilapia"))
	require.NoError(t, err)
	require.Len(t, slots, 1)
	require.Len(t, slots[0].Choices, 3)
	assert.True(t, slots[0].Choices[0].IsDefault)
	assert.Equal(t, ids("arroz-blanco"), slots[0].Choices[0].Items[0].IngredientProductID)
	assert.Equal(t, 200, slots[0].Choices[0].Items[0].Quantity)
	assert.Empty(t, slots[0].Choices[2].Items)

	responsibilities, err := repository.NewProductRepository(cloudGDB).FindPreparationResponsibilitiesByProductIDs(r.ctx(r.orgA),
		[]string{ids("tilapia"), ids("coca"), ids("agua"), ids("arroz-blanco")})
	require.NoError(t, err)
	assert.Equal(t, "kitchen", responsibilities[ids("tilapia")].Area)
	assert.Equal(t, "bar", responsibilities[ids("coca")].Area)
	assert.NotContains(t, responsibilities, ids("arroz-blanco"), "an ingredient gets no preparation area")

	stock, err := repository.NewStockRepository(cloudGDB).FindByProductID(r.ctx(r.orgA), ids("arroz-blanco"))
	require.NoError(t, err)
	assert.Equal(t, 25000, stock.Amount)
	assert.Equal(t, int64(1), r.count("historic_stock", r.orgA, "product_id = ? AND kind = ? AND change = ?", ids("arroz-blanco"), string(dto.StockMovementKindCount), 25000))

	product, err := repository.NewProductRepository(cloudGDB).FindByID(r.ctx(r.orgA), ids("coca"))
	require.NoError(t, err)
	assert.Equal(t, applied.Products["coca"].SKU, product.SKU)
	assert.Equal(t, "BAR-COC-COL-400", product.SKU)
	assert.Equal(t, "0.08", product.ICO.String(), "default ICO 8 applied")
	assert.Equal(t, dto.ProductTypeComposite, mustFind(t, r, ids("tilapia")).ProductType)
}

func mustFind(t *testing.T, r *rig, id string) *dto.Product {
	t.Helper()
	p, err := repository.NewProductRepository(cloudGDB).FindByID(r.ctx(r.orgA), id)
	require.NoError(t, err)
	return p
}

func TestAcceptance_CatalogImport_SameDocumentAppliedTwice(t *testing.T) {
	r := newRig(t)
	doc := []byte(fullDoc)
	first := r.validate(r.orgA, doc)
	status, _, body := r.apply(r.orgA, doc, first.Digest)
	require.Equal(t, http.StatusCreated, status, string(body))

	status, second, _ := r.apply(r.orgA, doc, first.Digest)

	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.False(t, second.Applied)
	conflicts := 0
	for _, p := range second.Problems {
		if p.Code == service.ProblemNameConflict {
			conflicts++
		}
	}
	assert.Equal(t, 6, conflicts, "one name_conflict per product: %+v", second.Problems)
	assert.Equal(t, int64(6), r.count("products", r.orgA, ""), "nothing duplicated")
	assert.Equal(t, int64(1), r.count("historic_stock", r.orgA, ""), "no second count")
}

func TestAcceptance_CatalogImport_ReferencesAnExistingProductWithoutChangingIt(t *testing.T) {
	r := newRig(t)
	products := service.NewProductService(repository.NewProductRepository(cloudGDB), nil, nil, postgres.NewUnitOfWork(cloudGDB))
	ctx, release := postgres.WithSession(r.ctx(r.orgA), cloudGDB)
	limon, err := products.CreateProduct(ctx, &dto.CreateProductRequest{
		Name: "Limón", Category: "Insumos", ProductType: "INGREDIENT", UnitOfMeasure: "unit", SKU: "ING-LIM",
	})
	release()
	require.NoError(t, err)
	before := mustFind(t, r, limon.ID)
	doc := []byte(`{
		"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "unit", "prep_area": "kitchen", "prep_priority": 0},
		"existing": [{"ref": "limon", "name": "limón "}],
		"plates": [{"ref": "pescado", "name": "Pescado al limón", "category": "Platos", "total_price_with_taxes": 30000,
			"recipe": [{"ingredient": "limon", "quantity": 0.5}]}],
		"opening_stock": []
	}`)

	validation := r.validate(r.orgA, doc)
	require.True(t, validation.Valid, "%+v", validation.Problems)
	status, applied, body := r.apply(r.orgA, doc, validation.Digest)

	require.Equal(t, http.StatusCreated, status, string(body))
	assert.NotContains(t, applied.Products, "limon")
	assert.Equal(t, int64(1), r.count("product_ingredients", r.orgA, "composite_product_id = ? AND ingredient_product_id = ?", applied.Products["pescado"].ID, limon.ID))
	after := mustFind(t, r, limon.ID)
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt, "Limón itself is unchanged")
	assert.Equal(t, before.Name, after.Name)
}

func TestAcceptance_CatalogImport_AnotherOrganizationsProductCannotBeReferenced(t *testing.T) {
	r := newRig(t)
	secretID := uuid.NewString()
	require.NoError(t, cloudGDB.Exec(`INSERT INTO products (id, organization_id, name, category, product_type, unit_of_measure, version,
		unit_price, vat, vat_amount, ico, ico_amount, sku, total_price_with_taxes) VALUES (?, ?, 'Salsa secreta de B', 'Insumos', 'INGREDIENT', 'g', 1, 0, 0, 0, 0, 0, 'B-SECRETA', 0)`,
		secretID, r.orgB).Error)
	doc := []byte(`{
		"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "unit", "prep_area": "kitchen", "prep_priority": 0},
		"existing": [{"ref": "salsa", "sku": "B-SECRETA"}],
		"plates": [{"ref": "p", "name": "Plato con salsa", "category": "Platos", "total_price_with_taxes": 20000,
			"recipe": [{"ingredient": "salsa", "quantity": 1}]}]
	}`)

	validation := r.validate(r.orgA, doc)
	status, applied, body := r.apply(r.orgA, doc, validation.Digest)

	assert.False(t, validation.Valid)
	require.Len(t, validation.Problems, 1)
	assert.Equal(t, service.ProblemExistingNotFound, validation.Problems[0].Code)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.False(t, applied.Applied)
	assert.Zero(t, r.count("products", r.orgA, ""), "nothing written")
	assert.NotContains(t, string(body), secretID)
	assert.NotContains(t, string(body), "Salsa secreta")
	assert.Equal(t, int64(0), r.count("product_ingredients", r.orgB, ""), "B is untouched")
}

// thousandProducts is the largest document the caps allow, shaped like a big menu.
func thousandProducts() []byte {
	var ingredients, sellables, plates []string
	for i := 0; i < 600; i++ {
		ingredients = append(ingredients, fmt.Sprintf(`{"ref": "i%d", "name": "Insumo %d", "category": "Insumos"}`, i, i))
	}
	for i := 0; i < 250; i++ {
		sellables = append(sellables, fmt.Sprintf(`{"ref": "s%d", "name": "Bebida %d", "category": "Bebidas", "unit_of_measure": "unit", "total_price_with_taxes": 5000}`, i, i))
	}
	for i := 0; i < 150; i++ {
		var recipe []string
		for j := 0; j < 5; j++ {
			recipe = append(recipe, fmt.Sprintf(`{"ingredient": "i%d", "quantity": 100}`, (i*5+j)%600))
		}
		plates = append(plates, fmt.Sprintf(`{"ref": "p%d", "name": "Plato %d", "category": "Platos", "unit_of_measure": "unit", "total_price_with_taxes": 30000,
			"recipe": [%s],
			"slots": [{"name": "Arroz", "choices": [{"name": "Blanco", "default": true, "items": [{"ingredient": "i%d", "quantity": 200}]}, {"name": "Coco", "items": [{"ingredient": "i%d", "quantity": 200}]}, {"name": "Sin arroz"}]},
			          {"name": "Acompañante", "choices": [{"name": "Papas", "default": true, "items": [{"ingredient": "i%d", "quantity": 150}]}, {"name": "Patacón", "items": [{"ingredient": "i%d", "quantity": 2}]}, {"name": "Yuca", "items": [{"ingredient": "i%d", "quantity": 150}]}]}]}`,
			i, i, strings.Join(recipe, ","), i%600, (i+1)%600, (i+2)%600, (i+3)%600, (i+4)%600))
	}
	return []byte(`{"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0},
		"ingredients": [` + strings.Join(ingredients, ",") + `], "sellables": [` + strings.Join(sellables, ",") + `], "plates": [` + strings.Join(plates, ",") + `]}`)
}

func TestAcceptance_CatalogImport_ThousandProductApplyTime(t *testing.T) {
	r := newRig(t)
	doc := thousandProducts()
	require.Less(t, len(doc), handler.MaxCatalogImportBodyBytes)
	validation := r.validate(r.orgA, doc)
	require.True(t, validation.Valid, "%+v", validation.Problems)

	start := time.Now()
	status, _, body := r.apply(r.orgA, doc, validation.Digest)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusCreated, status, string(body[:min(len(body), 2000)]))
	t.Logf("1,000-product apply (%d KB document, 750 recipe lines, 300 slots) took %s", len(doc)/1024, elapsed)
	assert.Equal(t, int64(1000), r.count("products", r.orgA, ""))
	assert.Equal(t, int64(750), r.count("product_ingredients", r.orgA, ""))
	assert.Equal(t, int64(300), r.count("plate_option_slots", r.orgA, "deleted_at IS NULL"))
	assert.Less(t, elapsed, 60*time.Second)
}
