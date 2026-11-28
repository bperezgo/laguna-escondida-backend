package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type importHandlerMocks struct {
	products    *mocks.MockProductRepository
	ingredients *mocks.MockProductIngredientRepository
	slots       *mocks.MockPlateOptionSlotRepository
	stock       *mocks.MockStockRepository
}

func importRouter(t *testing.T) (*gin.Engine, importHandlerMocks) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	m := importHandlerMocks{
		products:    mocks.NewMockProductRepository(t),
		ingredients: mocks.NewMockProductIngredientRepository(t),
		slots:       mocks.NewMockPlateOptionSlotRepository(t),
		stock:       mocks.NewMockStockRepository(t),
	}
	svc := service.NewCatalogImportService(m.products, m.ingredients, m.slots, m.stock, immediateUnitOfWork{}, mocks.NewMockStockMovementEmitter(t))
	h := NewCatalogImportHandler(svc)
	router := gin.New()
	router.POST("/api/catalog/import/validate", h.ValidateHandler)
	router.POST("/api/catalog/import", h.ApplyHandler)
	return router, m
}

func postImport(router *gin.Engine, path string, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

const handlerImportDoc = `{
	"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0},
	"ingredients": [{"ref": "arroz", "name": "Arroz", "category": "insumos"}],
	"plates": [{"ref": "p", "name": "Arroz con pollo", "category": "platos", "total_price_with_taxes": 20000, "unit_of_measure": "unit",
		"recipe": [{"ingredient": "arroz", "quantity": 200}]}]
}`

func TestCatalogImportHandler_Validate_ValidDocument(t *testing.T) {
	router, m := importRouter(t)
	m.products.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(nil, nil)

	rec := postImport(router, "/api/catalog/import/validate", []byte(handlerImportDoc))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got dto.CatalogImportResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.Valid)
	assert.False(t, got.Applied)
	assert.Len(t, got.Digest, 64)
	assert.Equal(t, 2, got.Summary.Products)
	assert.Equal(t, `[]`, string(mustField(t, rec.Body.Bytes(), "problems")), "problems is an empty list, never null")
}

func TestCatalogImportHandler_Validate_InvalidDocumentIs200WithProblems(t *testing.T) {
	router, m := importRouter(t)
	m.products.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(nil, nil)

	rec := postImport(router, "/api/catalog/import/validate", []byte(`{"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}]}`))

	require.Equal(t, http.StatusOK, rec.Code)
	var got dto.CatalogImportResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.False(t, got.Valid)
	require.Len(t, got.Problems, 1)
	assert.Equal(t, service.ProblemMissingField, got.Problems[0].Code)
}

func TestCatalogImportHandler_UnknownField(t *testing.T) {
	body := `{"plates": [{"ref": "p", "name": "X", "category": "c", "recipe": [{"ingredient": "a", "product_id": "019a", "quantity": 1}]}]}`
	for _, path := range []string{"/api/catalog/import/validate", "/api/catalog/import"} {
		t.Run(path, func(t *testing.T) {
			router, _ := importRouter(t)

			rec := postImport(router, path, []byte(body))

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), "product_id")
		})
	}
}

func TestCatalogImportHandler_TrailingDataIsRefused(t *testing.T) {
	router, _ := importRouter(t)

	rec := postImport(router, "/api/catalog/import/validate", []byte(`{} {"ingredients": []}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCatalogImportHandler_OversizedBody(t *testing.T) {
	name := strings.Repeat("a", 3<<20)
	body := []byte(`{"ingredients": [{"ref": "a", "name": "` + name + `", "category": "c"}]}`)
	for _, path := range []string{"/api/catalog/import/validate", "/api/catalog/import"} {
		t.Run(path, func(t *testing.T) {
			router, _ := importRouter(t)

			rec := postImport(router, path, body)

			assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		})
	}
}

func TestCatalogImportHandler_Apply_Created(t *testing.T) {
	router, m := importRouter(t)
	m.products.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(nil, nil)
	validated := postImport(router, "/api/catalog/import/validate", []byte(handlerImportDoc))
	var v dto.CatalogImportResult
	require.NoError(t, json.Unmarshal(validated.Body.Bytes(), &v))

	m.products.EXPECT().CreateMany(mock.Anything, mock.Anything).Return(nil)
	m.ingredients.EXPECT().CreateMany(mock.Anything, mock.Anything).Return(nil)
	m.products.EXPECT().CreatePreparationResponsibilities(mock.Anything, mock.Anything).Return(nil)
	body := strings.Replace(handlerImportDoc, `{`, `{"expected_digest": "`+v.Digest+`",`, 1)

	rec := postImport(router, "/api/catalog/import", []byte(body))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var got dto.CatalogImportResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.Applied)
	assert.Len(t, got.Products, 2)
	assert.NotEmpty(t, got.Products["arroz"].ID)
	assert.NotEmpty(t, got.Products["arroz"].SKU)
}

func TestCatalogImportHandler_Apply_DigestMismatchIs409(t *testing.T) {
	router, _ := importRouter(t)
	body := strings.Replace(handlerImportDoc, `{`, `{"expected_digest": "stale",`, 1)

	rec := postImport(router, "/api/catalog/import", []byte(body))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), service.ProblemDigestMismatch)
}

func TestCatalogImportHandler_Apply_ProblemsAre422(t *testing.T) {
	router, m := importRouter(t)
	m.products.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(nil, nil)
	doc := `{"ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}]}`
	validated := postImport(router, "/api/catalog/import/validate", []byte(doc))
	var v dto.CatalogImportResult
	require.NoError(t, json.Unmarshal(validated.Body.Bytes(), &v))

	rec := postImport(router, "/api/catalog/import", []byte(`{"expected_digest": "`+v.Digest+`", "ingredients": [{"ref": "a", "name": "Arroz", "category": "insumos"}]}`))

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), service.ProblemMissingField)
}

func TestCatalogImportHandler_Apply_WriteFailureIs500(t *testing.T) {
	router, m := importRouter(t)
	m.products.EXPECT().FindAll(mock.Anything, dto.ListProductsRequest{}).Return(nil, nil)
	validated := postImport(router, "/api/catalog/import/validate", []byte(handlerImportDoc))
	var v dto.CatalogImportResult
	require.NoError(t, json.Unmarshal(validated.Body.Bytes(), &v))
	m.products.EXPECT().CreateMany(mock.Anything, mock.Anything).Return(errors.New("db down"))
	body := strings.Replace(handlerImportDoc, `{`, `{"expected_digest": "`+v.Digest+`",`, 1)

	rec := postImport(router, "/api/catalog/import", []byte(body))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "db down")
}

func mustField(t *testing.T, body []byte, field string) json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	return fields[field]
}
