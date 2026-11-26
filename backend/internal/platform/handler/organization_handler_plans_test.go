package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrganizationHandler_Create_DefaultsToActiveFull(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations", `{"name":"Restaurante X S.A.S","slug":"restaurante-x"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	assert.JSONEq(t, `"active"`, string(body["status"]))
	assert.JSONEq(t, `"full"`, string(body["plan"]))
	assert.JSONEq(t, `null`, string(body["trial_ends_at"]))
}

func TestOrganizationHandler_Create_TrialWithoutAnEndReturns400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPost, "/api/platform/organizations", `{"name":"X","slug":"restaurante-x","status":"trial"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"trial_ends_at"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_Update_UnknownPlanReturns400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"plan":"premium"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"plan"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_Update_TrialWithoutAnEndReturns400(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Status: "active"}, nil)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"status":"trial"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"trial_ends_at"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_Update_ActivateAndExtend(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Update(mock.Anything, testOrganizationID, mock.MatchedBy(func(req *dto.UpdateOrganizationRequest) bool {
		return *req.Status == "active" && *req.Plan == "einvoicing"
	})).Return(&dto.Organization{ID: testOrganizationID, Status: "active", Plan: "einvoicing"}, nil)
	later := time.Date(2026, 12, 30, 0, 0, 0, 0, time.UTC)
	orgRepo.EXPECT().Update(mock.Anything, testOrganizationID, mock.MatchedBy(func(req *dto.UpdateOrganizationRequest) bool {
		return req.TrialEndsAt != nil && *req.TrialEndsAt == "2026-12-30T00:00:00Z"
	})).Return(&dto.Organization{ID: testOrganizationID, Status: "trial", Plan: "full", TrialEndsAt: &later}, nil)

	rec := serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"status":"active","plan":"einvoicing"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var org dto.Organization
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &org))
	assert.Equal(t, "active", org.Status)
	assert.Equal(t, "einvoicing", org.Plan)

	rec = serveOrganization(router, http.MethodPatch, "/api/platform/organizations/"+testOrganizationID, `{"trial_ends_at":"2026-12-30T00:00:00Z"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `"2026-12-30T00:00:00Z"`, string(decodeBody(t, rec)["trial_ends_at"]))
}
