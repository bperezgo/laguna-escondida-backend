package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newLeadRouter(t *testing.T) (*gin.Engine, *mocks.MockLeadRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockLeadRepository(t)
	svc := service.NewLeadService(repo, true, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Now)
	h := NewLeadHandler(svc)

	router := gin.New()
	router.POST("/api/public/leads", h.SubmitHandler)
	router.GET("/api/platform/leads", h.ListHandler)
	router.GET("/api/platform/leads/:id", h.GetHandler)
	router.PATCH("/api/platform/leads/:id", h.UpdateHandler)
	router.DELETE("/api/platform/leads/:id", h.DeleteHandler)
	return router, repo
}

const validLeadBody = `{"name":"Ana Ruiz","restaurant_name":"El Fogón","phone":"300 123 4567","consent":true,"utm_source":"facebook"}`

func TestLeadHandler_Submit_Returns201WithoutAnyField(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().FindRecentByPhone(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.EXPECT().CountSince(mock.Anything, mock.Anything).Return(0, nil)
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads", validLeadBody)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"received"}`, rec.Body.String())
}

func TestLeadHandler_Submit_NamesTheInvalidField(t *testing.T) {
	router, _ := newLeadRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads",
		`{"name":"Ana","restaurant_name":"El Fogón","phone":"12345","consent":true}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"phone"`, string(decodeBody(t, rec)["field"]))
}

func TestLeadHandler_Submit_NoConsentNamesConsent(t *testing.T) {
	router, _ := newLeadRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads",
		`{"name":"Ana","restaurant_name":"El Fogón","phone":"3001234567","consent":false}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"consent"`, string(decodeBody(t, rec)["field"]))
}

func TestLeadHandler_Submit_NotJSONIs400(t *testing.T) {
	router, _ := newLeadRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads", `{not json`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLeadHandler_Submit_BodyOver16KBIs413(t *testing.T) {
	router, _ := newLeadRouter(t)
	body := `{"name":"` + strings.Repeat("a", 17<<10) + `"}`

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads", body)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestLeadHandler_Submit_GlobalCapIs429WithRetryAfter(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().FindRecentByPhone(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.EXPECT().CountSince(mock.Anything, mock.Anything).Return(300, nil)

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads", validLeadBody)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "300", rec.Header().Get("Retry-After"))
}

func TestLeadHandler_Submit_HoneypotStillAnswers201(t *testing.T) {
	router, _ := newLeadRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/public/leads",
		`{"name":"Ana","restaurant_name":"El Fogón","phone":"3001234567","consent":true,"website":"http://spam.example"}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"status":"received"}`, rec.Body.String())
}

func TestLeadHandler_List_PassesFilterAndReturnsEveryField(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().List(mock.Anything, dto.ListLeadsFilter{Status: "new", Page: 2, PageSize: 10}).
		Return([]*dto.Lead{{ID: "l1", Name: "Ana", Status: dto.LeadStatusNew, LeadAttribution: dto.LeadAttribution{UTMSource: "facebook"}}}, int64(11), nil)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/leads?status=new&page=2&page_size=10", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var page struct {
		Leads []map[string]any `json:"leads"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	assert.Equal(t, 11, page.Total)
	require.Len(t, page.Leads, 1)
	assert.Equal(t, "facebook", page.Leads[0]["utm_source"])
	assert.NotContains(t, page.Leads[0], "alert_status")
}

func TestLeadHandler_List_BadPageIs400(t *testing.T) {
	router, _ := newLeadRouter(t)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/leads?page=abc", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLeadHandler_List_UnknownStatusIs400(t *testing.T) {
	router, _ := newLeadRouter(t)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/leads?status=maybe", "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"status"`, string(decodeBody(t, rec)["field"]))
}

func TestLeadHandler_Get_UnknownIs404(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().Get(mock.Anything, "nope").Return(nil, domainError.ErrLeadNotFound)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/leads/nope", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestLeadHandler_Update_ChangesStatusAndNotes(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().Get(mock.Anything, "l1").Return(&dto.Lead{ID: "l1", Name: "Ana", Status: dto.LeadStatusNew}, nil)
	repo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/leads/l1", `{"status":"contacted","notes":"Demo el jueves"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got dto.Lead
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, dto.LeadStatusContacted, got.Status)
	assert.Equal(t, "Demo el jueves", got.Notes)
	assert.Equal(t, "Ana", got.Name)
}

func TestLeadHandler_Update_UnknownStatusIs400(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().Get(mock.Anything, "l1").Return(&dto.Lead{ID: "l1"}, nil)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/leads/l1", `{"status":"archived"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"status"`, string(decodeBody(t, rec)["field"]))
}

func TestLeadHandler_Delete(t *testing.T) {
	router, repo := newLeadRouter(t)
	repo.EXPECT().Delete(mock.Anything, "l1").Return(nil)
	repo.EXPECT().Delete(mock.Anything, "nope").Return(domainError.ErrLeadNotFound)

	assert.Equal(t, http.StatusNoContent, serveOrganization(router, http.MethodDelete, "/api/platform/leads/l1", "").Code)
	assert.Equal(t, http.StatusNotFound, serveOrganization(router, http.MethodDelete, "/api/platform/leads/nope", "").Code)
}
