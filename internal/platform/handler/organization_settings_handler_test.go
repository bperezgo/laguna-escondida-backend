package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func settingsRouter(t *testing.T, repo *mocks.MockOrganizationSettingsRepository, roleIDs ...int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewOrganizationSettingsHandler(service.NewOrganizationSettingsService(repo))
	router := gin.New()
	setRoles := func(c *gin.Context) { c.Set("role_ids", roleIDs) }
	router.GET("/api/organization/settings", setRoles, h.GetSettingsHandler)
	router.PATCH("/api/organization/settings", setRoles, h.PatchSettingsHandler)
	return router
}

func serve(router *gin.Engine, method, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = &bytes.Buffer{}
	}
	router.ServeHTTP(rec, httptest.NewRequest(method, "/api/organization/settings", reader))
	return rec
}

// storeMerges applies the patch to empty settings the way the real repository does.
func storeMerges(repo *mocks.MockOrganizationSettingsRepository) {
	repo.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, merge ports.OrderingMerge) (*dto.OrganizationSettingsView, error) {
			next, err := merge(dto.OrderingSettings{})
			if err != nil {
				return nil, err
			}
			return &dto.OrganizationSettingsView{Ordering: next, Timezone: "America/Bogota"}, nil
		}).Once()
}

func TestOrganizationSettingsHandler_Get_200(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: "America/Bogota"}, nil).Once()

	rec := serve(settingsRouter(t, repo, permissions.RoleAgent), http.MethodGet, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"ordering": {
			"delivery": {"paused": false, "paused_reason": null, "schedule": []},
			"takeaway": {"paused": false, "paused_reason": null, "schedule": []}
		},
		"availability": {
			"delivery": {"available": true, "reason": null, "paused_reason": null, "closes_at": null, "next_opens_at": null},
			"takeaway": {"available": true, "reason": null, "paused_reason": null, "closes_at": null, "next_opens_at": null}
		},
		"timezone": "America/Bogota"
	}`, rec.Body.String())
}

func TestOrganizationSettingsHandler_Get_500(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, errors.New("db down")).Once()

	rec := serve(settingsRouter(t, repo, permissions.RoleManager), http.MethodGet, "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOrganizationSettingsHandler_Patch_PauseByManager_200(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	storeMerges(repo)

	rec := serve(settingsRouter(t, repo, permissions.RoleManager), http.MethodPatch,
		`{"ordering":{"delivery":{"paused":true,"paused_reason":"cocina saturada"}}}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"reason":"paused"`)
	assert.Contains(t, rec.Body.String(), `"paused_reason":"cocina saturada"`)
}

func TestOrganizationSettingsHandler_Patch_400(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"malformed JSON", `{"ordering":`, `{"error":"Invalid request body"}`},
		{"wrong type", `{"ordering":{"delivery":{"paused":"yes"}}}`, `{"error":"Invalid request body"}`},
		{
			"overlapping windows",
			`{"ordering":{"takeaway":{"schedule":[{"day":"mon","opens":"11:00","closes":"15:00"},{"day":"mon","opens":"14:00","closes":"18:00"}]}}}`,
			`{"error":"invalid ordering.takeaway.schedule[1]: overlaps ordering.takeaway.schedule[0]",
				"code":"INVALID_ORDERING_SETTINGS","field":"ordering.takeaway.schedule[1]"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockOrganizationSettingsRepository(t)
			repo.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, merge ports.OrderingMerge) (*dto.OrganizationSettingsView, error) {
					_, err := merge(dto.OrderingSettings{})
					return nil, err
				}).Maybe()

			rec := serve(settingsRouter(t, repo, permissions.RoleManager), http.MethodPatch, tt.body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tt.want, rec.Body.String())
		})
	}
}

func TestOrganizationSettingsHandler_Patch_403(t *testing.T) {
	// A floor role with the pause switch but not the hours; no role has that split yet.
	pauseOnly := 990
	manageOnly := 991
	permissions.RolePermissions[pauseOnly] = []permissions.Permission{permissions.OrderingRead, permissions.OrderingPause}
	permissions.RolePermissions[manageOnly] = []permissions.Permission{permissions.OrderingRead, permissions.OrderingManage}
	t.Cleanup(func() {
		delete(permissions.RolePermissions, pauseOnly)
		delete(permissions.RolePermissions, manageOnly)
	})

	tests := []struct {
		name     string
		role     int
		body     string
		required string
	}{
		{"pause permission alone cannot change hours", pauseOnly,
			`{"ordering":{"takeaway":{"schedule":[{"day":"mon","opens":"11:30","closes":"22:30"}]}}}`, "ordering:manage"},
		{"mixed body without the pause permission", manageOnly,
			`{"ordering":{"delivery":{"schedule":[]},"takeaway":{"paused":true}}}`, "ordering:pause"},
		{"the agent cannot pause", permissions.RoleAgent, `{"ordering":{"delivery":{"paused":true}}}`, "ordering:pause"},
		{"the accountant cannot change hours", permissions.RoleAccountant, `{"ordering":{"delivery":{"schedule":[]}}}`, "ordering:manage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockOrganizationSettingsRepository(t)

			rec := serve(settingsRouter(t, repo, tt.role), http.MethodPatch, tt.body)

			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.JSONEq(t, `{"error":"Insufficient permissions","required":"`+tt.required+`"}`, rec.Body.String())
			repo.AssertNotCalled(t, "UpdateOrdering", mock.Anything, mock.Anything)
		})
	}
}

func TestOrganizationSettingsHandler_Patch_PauseOnlyRoleCanPause(t *testing.T) {
	pauseOnly := 992
	permissions.RolePermissions[pauseOnly] = []permissions.Permission{permissions.OrderingRead, permissions.OrderingPause}
	t.Cleanup(func() { delete(permissions.RolePermissions, pauseOnly) })
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	storeMerges(repo)

	rec := serve(settingsRouter(t, repo, pauseOnly), http.MethodPatch, `{"ordering":{"takeaway":{"paused":true}}}`)

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
