package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const startTrialPath = "/api/platform/organizations/" + testOrganizationID + "/start-trial"

func TestOrganizationHandler_StartTrial_Pending(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Status: "pending", Plan: "einvoicing"}, nil)
	var endsAt time.Time
	orgRepo.EXPECT().StartTrial(mock.Anything, testOrganizationID, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, ends time.Time) (*dto.Organization, error) {
			endsAt = ends
			return &dto.Organization{ID: testOrganizationID, Status: "trial", Plan: "einvoicing", TrialEndsAt: &ends}, nil
		})
	before := time.Now()

	rec := serveOrganization(router, http.MethodPost, startTrialPath, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var org dto.Organization
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &org))
	assert.Equal(t, "trial", org.Status)
	assert.Equal(t, "einvoicing", org.Plan)
	assert.WithinDuration(t, before.Add(14*24*time.Hour), endsAt, time.Minute)
}

func TestOrganizationHandler_StartTrial_NotPendingIs409(t *testing.T) {
	ends := time.Now().Add(time.Hour)
	for _, current := range []*dto.Organization{
		{ID: testOrganizationID, Status: "trial", TrialEndsAt: &ends},
		{ID: testOrganizationID, Status: "active"},
	} {
		t.Run(current.Status, func(t *testing.T) {
			router, orgRepo, _ := newOrganizationRouter(t)
			orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(current, nil)

			rec := serveOrganization(router, http.MethodPost, startTrialPath, "")

			require.Equal(t, http.StatusConflict, rec.Code)
			assert.JSONEq(t, `"not_pending"`, string(decodeBody(t, rec)["code"]))
			orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestOrganizationHandler_StartTrial_UnknownIs404(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)

	rec := serveOrganization(router, http.MethodPost, startTrialPath, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrganizationHandler_List_StatusFilter(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	created := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	orgRepo.EXPECT().ListByStatus(mock.Anything, "pending").Return([]*dto.Organization{
		{ID: "a", Name: "A", Status: "pending", Plan: "starter", CreatedAt: created},
		{ID: "b", Name: "B", Status: "pending", Plan: "full", CreatedAt: created},
	}, nil)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations?status=pending", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct{ Organizations []dto.Organization }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Organizations, 2)
	for _, o := range body.Organizations {
		assert.Equal(t, "pending", o.Status)
		assert.NotEmpty(t, o.Plan)
		assert.Equal(t, created, o.CreatedAt.UTC())
	}
	orgRepo.AssertNotCalled(t, "List", mock.Anything)
}

func TestOrganizationHandler_List_UnknownStatusIs400(t *testing.T) {
	router, _, _ := newOrganizationRouter(t)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations?status=waiting", "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"status"`, string(decodeBody(t, rec)["field"]))
}

func TestOrganizationHandler_List_NoFilterReturnsEverything(t *testing.T) {
	router, orgRepo, _ := newOrganizationRouter(t)
	orgRepo.EXPECT().List(mock.Anything).Return([]*dto.Organization{{ID: "a", Status: "active"}, {ID: "b", Status: "pending"}}, nil)

	rec := serveOrganization(router, http.MethodGet, "/api/platform/organizations", "")

	require.Equal(t, http.StatusOK, rec.Code)
	orgRepo.AssertNotCalled(t, "ListByStatus", mock.Anything, mock.Anything)
}
