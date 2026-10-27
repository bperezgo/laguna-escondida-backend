package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testOrganizationID = "019a0000-0000-7000-8000-000000000001"

type immediateUnitOfWork struct{}

func (immediateUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newOrganizationRouter(t *testing.T) (*gin.Engine, *mocks.MockOrganizationRepository, *mocks.MockEInvoicingSettingsRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	orgRepo := mocks.NewMockOrganizationRepository(t)
	einvoicingRepo := mocks.NewMockEInvoicingSettingsRepository(t)
	h := NewOrganizationHandler(service.NewOrganizationService(orgRepo, einvoicingRepo, immediateUnitOfWork{}))

	router := gin.New()
	router.POST("/api/platform/organizations", h.CreateHandler)
	router.GET("/api/platform/organizations", h.ListHandler)
	router.GET("/api/platform/organizations/:id", h.GetHandler)
	router.PATCH("/api/platform/organizations/:id", h.UpdateHandler)
	router.PUT("/api/platform/organizations/:id/einvoicing", h.SetEInvoicingHandler)
	return router, orgRepo, einvoicingRepo
}

func serveOrganization(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestOrganizationHandler_Create_Returns201WithDefaults(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations", `{"name":"Restaurante X S.A.S","slug":"restaurante-x"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var org dto.Organization
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &org))
	assert.NotEmpty(t, org.ID)
	assert.Equal(t, "active", org.Status)
	assert.Equal(t, "America/Bogota", org.Timezone)
	assert.Equal(t, "COP", org.Currency)
	assert.JSONEq(t, "null", string(decodeBody(t, rec)["einvoicing"]))
}

func TestOrganizationHandler_Create_InvalidFieldReturns400WithField(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations",
		`{"name":"Restaurante X","slug":"restaurante-x","timezone":"America/Bogata"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"timezone"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_Create_MalformedBodyReturns400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations", `{"name":`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOrganizationHandler_Create_SlugTakenReturns409(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(domainError.ErrOrganizationSlugTaken)

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations", `{"name":"Laguna","slug":"laguna-escondida"}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestOrganizationHandler_Create_UnexpectedErrorReturns500(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down"))

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations", `{"name":"Restaurante X","slug":"restaurante-x"}`)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "db down")
}

func TestOrganizationHandler_List_Returns200(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().List(mock.Anything).Return([]*dto.Organization{{ID: "a", Name: "A"}}, nil)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations", "")

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Organizations []dto.Organization `json:"organizations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Organizations, 1)
}

func TestOrganizationHandler_Get_Returns200(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{
		ID: testOrganizationID, EInvoicing: &dto.EInvoicingSettings{OrganizationID: testOrganizationID, InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"},
	}, nil)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations/"+testOrganizationID, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"invoice_prefix":"RX","support_document_prefix":"DSX"}`, string(decodeBody(t, rec)["einvoicing"]))
}

func TestOrganizationHandler_Get_UnknownReturns404(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations/"+testOrganizationID, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrganizationHandler_Update_Returns200(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	displayName, empty := "Laguna", ""
	orgRepo.EXPECT().Update(mock.Anything, testOrganizationID, &dto.UpdateOrganizationRequest{DisplayName: &displayName, TicketFooter: &empty}).
		Return(&dto.Organization{ID: testOrganizationID, DisplayName: &displayName}, nil)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"display_name":"Laguna","ticket_footer":""}`)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrganizationHandler_Update_SlugReturns400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"slug":"new-slug"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"slug"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_Update_ClearingRequiredFieldReturns400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"timezone":""}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"timezone"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_Update_UnknownReturns404(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Update(mock.Anything, testOrganizationID, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"display_name":"X"}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrganizationHandler_SetEInvoicing_Returns200(t *testing.T) {
	router, orgRepo, einvoicingRepo := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID}, nil)
	einvoicingRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil)

	rec := serveOrganization(router, http.MethodPut, "/api/platform/organizations/"+testOrganizationID+"/einvoicing",
		`{"invoice_prefix":"RX","support_document_prefix":"DSX"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"invoice_prefix":"RX","support_document_prefix":"DSX"}`, rec.Body.String())
}

func TestOrganizationHandler_SetEInvoicing_EqualPrefixesReturns400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPut, "/api/platform/organizations/"+testOrganizationID+"/einvoicing",
		`{"invoice_prefix":"RX","support_document_prefix":"RX"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOrganizationHandler_SetEInvoicing_UnknownReturns404(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)

	rec := serveOrganization(router, http.MethodPut, "/api/platform/organizations/"+testOrganizationID+"/einvoicing",
		`{"invoice_prefix":"RX","support_document_prefix":"DSX"}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrganizationHandler_Get_IncludesSettings(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	reason := "cocina saturada"
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{
		ID: testOrganizationID,
		Settings: dto.OrganizationSettings{Ordering: dto.OrderingSettings{
			Delivery: dto.OrderTypeSettings{Paused: true, PausedReason: &reason, Schedule: []dto.OpeningWindow{}},
			Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{{Day: "mon", Opens: "11:30", Closes: "22:30"}}},
		}},
	}, nil)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations/"+testOrganizationID, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ordering":{
		"delivery":{"paused":true,"paused_reason":"cocina saturada","schedule":[]},
		"takeaway":{"paused":false,"paused_reason":null,"schedule":[{"day":"mon","opens":"11:30","closes":"22:30"}]}
	}}`, string(decodeBody(t, rec)["settings"]))
}
