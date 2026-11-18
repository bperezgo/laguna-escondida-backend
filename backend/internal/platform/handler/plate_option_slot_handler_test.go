package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const slotHandlerPlateID = "01900000-0000-7000-8000-000000000001"

type slotHandlerMocks struct {
	slots    *mocks.MockPlateOptionSlotRepository
	products *mocks.MockProductRepository
}

func slotRouter(t *testing.T) (*gin.Engine, slotHandlerMocks) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	m := slotHandlerMocks{
		slots:    mocks.NewMockPlateOptionSlotRepository(t),
		products: mocks.NewMockProductRepository(t),
	}
	h := NewPlateOptionSlotHandler(service.NewPlateOptionSlotService(m.slots, m.products))
	router := gin.New()
	router.GET("/api/products/:id/option-slots", h.GetHandler)
	router.PUT("/api/products/:id/option-slots", h.ReplaceHandler)
	return router, m
}

func serveSlots(router *gin.Engine, method, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, "/api/products/"+slotHandlerPlateID+"/option-slots", bytes.NewBufferString(body)))
	return rec
}

func composite() *dto.Product {
	return &dto.Product{ID: slotHandlerPlateID, Name: "Tilapia", ProductType: dto.ProductTypeComposite}
}

const oneSlotBody = `{"slots":[{"name":"Arroz","choices":[{"name":"Arroz blanco","is_default":true,"items":[]}]}]}`

func TestPlateOptionSlotHandler_Get_200(t *testing.T) {
	router, m := slotRouter(t)
	m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(composite(), nil)
	m.slots.EXPECT().FindByCompositeProductID(mock.Anything, slotHandlerPlateID).Return(nil, nil)

	rec := serveSlots(router, http.MethodGet, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"slots":[]}`, rec.Body.String())
}

func TestPlateOptionSlotHandler_Get_404(t *testing.T) {
	router, m := slotRouter(t)
	m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(nil, errors.New("record not found"))

	assert.Equal(t, http.StatusNotFound, serveSlots(router, http.MethodGet, "").Code)
}

func TestPlateOptionSlotHandler_Get_500(t *testing.T) {
	router, m := slotRouter(t)
	m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(composite(), nil)
	m.slots.EXPECT().FindByCompositeProductID(mock.Anything, slotHandlerPlateID).Return(nil, errors.New("db down"))

	assert.Equal(t, http.StatusInternalServerError, serveSlots(router, http.MethodGet, "").Code)
}

func TestPlateOptionSlotHandler_Replace_200(t *testing.T) {
	router, m := slotRouter(t)
	m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(composite(), nil)
	m.slots.EXPECT().FindByCompositeProductID(mock.Anything, slotHandlerPlateID).Return(nil, nil).Once()
	m.slots.EXPECT().ReplaceForCompositeProduct(mock.Anything, slotHandlerPlateID, mock.Anything).Return(nil)
	saved := []dto.PlateOptionSlot{{ID: "slot-1", Name: "Arroz", Choices: []dto.PlateOptionChoice{{ID: "c1", Name: "Arroz blanco", IsDefault: true}}}}
	m.slots.EXPECT().FindByCompositeProductID(mock.Anything, slotHandlerPlateID).Return(saved, nil).Once()

	rec := serveSlots(router, http.MethodPut, oneSlotBody)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dto.PlateOptionSlotListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "Arroz blanco", got.Slots[0].Choices[0].Name)
}

func TestPlateOptionSlotHandler_Replace_Errors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		setup  func(m slotHandlerMocks)
		status int
	}{
		{"malformed body", `{"slots":`, func(slotHandlerMocks) {}, http.StatusBadRequest},
		{"plate not found", oneSlotBody, func(m slotHandlerMocks) {
			m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(nil, errors.New("record not found"))
		}, http.StatusNotFound},
		{"bundle ingredient not found", `{"slots":[{"name":"Arroz","choices":[{"name":"A","is_default":true,"items":[{"ingredient_product_id":"01900000-0000-7000-8000-0000000000a9","quantity":1}]}]}]}`,
			func(m slotHandlerMocks) {
				m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(composite(), nil)
				m.products.EXPECT().FindByIDs(mock.Anything, mock.Anything).Return(nil, nil)
			}, http.StatusNotFound},
		{"not a composite", oneSlotBody, func(m slotHandlerMocks) {
			m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).
				Return(&dto.Product{ID: slotHandlerPlateID, ProductType: dto.ProductTypeSellable}, nil)
		}, http.StatusBadRequest},
		{"two defaults", `{"slots":[{"name":"Arroz","choices":[{"name":"A","is_default":true},{"name":"B","is_default":true}]}]}`,
			func(m slotHandlerMocks) {
				m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(composite(), nil)
			}, http.StatusBadRequest},
		{"repository failure", oneSlotBody, func(m slotHandlerMocks) {
			m.products.EXPECT().FindByID(mock.Anything, slotHandlerPlateID).Return(composite(), nil)
			m.slots.EXPECT().FindByCompositeProductID(mock.Anything, slotHandlerPlateID).Return(nil, errors.New("db down"))
		}, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, m := slotRouter(t)
			tt.setup(m)

			rec := serveSlots(router, http.MethodPut, tt.body)

			assert.Equal(t, tt.status, rec.Code, rec.Body.String())
		})
	}
}
