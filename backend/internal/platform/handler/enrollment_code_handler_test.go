package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type codeDeps struct {
	codes  *mocks.MockEnrollmentCodeRepository
	nodes  *mocks.MockNodeRepository
	orgs   *mocks.MockOrganizationRepository
	access *mocks.MockOrganizationAccessReader
	jwt    *service.JWTService
	router *gin.Engine
}

func newCodeDeps(t *testing.T) codeDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d := codeDeps{codes: mocks.NewMockEnrollmentCodeRepository(t), nodes: mocks.NewMockNodeRepository(t),
		orgs: mocks.NewMockOrganizationRepository(t), access: mocks.NewMockOrganizationAccessReader(t),
		jwt: service.NewJWTService("test-secret", tenant.AnyOrganization())}
	nodes := service.NewNodeService(d.nodes, d.orgs, immediateUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://api.example.com")
	h := NewEnrollmentCodeHandler(service.NewEnrollmentCodeService(d.codes, nodes, service.NewOrganizationAccessService(d.access), immediateUnitOfWork{}))
	d.router = gin.New()
	d.router.POST("/api/edge/enrollment-codes", JWTAuthMiddleware(d.jwt), RequirePermission(permissions.EdgeEnroll), h.IssueHandler)
	d.router.POST("/api/edge/enroll-with-code", h.EnrollHandler)
	return d
}

func (d codeDeps) post(t *testing.T, path, body string, roleID int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if roleID != 0 {
		token, err := d.jwt.GenerateToken("admin-id", testOrganizationID, "admin", []int{roleID})
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	d.router.ServeHTTP(rec, req)
	return rec
}

func (d codeDeps) validCode() {
	d.codes.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).Return(&dto.EnrollmentCode{
		ID: "code-1", OrganizationID: testOrganizationID, ExpiresAt: time.Now().Add(10 * time.Minute)}, nil)
}

func TestEnrollmentCodeHandler_Issue(t *testing.T) {
	d := newCodeDeps(t)
	d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "active", Plan: "full"}, nil)
	d.codes.EXPECT().InvalidateUnused(mock.Anything, mock.Anything).Return(nil)
	d.codes.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	rec := d.post(t, "/api/edge/enrollment-codes", "", permissions.RoleAdmin)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	assert.Regexp(t, `^"[2-9A-Z]{4}-[2-9A-Z]{4}"$`, string(body["code"]))
	assert.Contains(t, body, "expires_at")
}

func TestEnrollmentCodeHandler_Issue_ManagerIsForbidden(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, newCodeDeps(t).post(t, "/api/edge/enrollment-codes", "", permissions.RoleManager).Code)
}

func TestEnrollmentCodeHandler_Issue_InactiveOrganization(t *testing.T) {
	d := newCodeDeps(t)
	d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "suspended", Plan: "full"}, nil)

	rec := d.post(t, "/api/edge/enrollment-codes", "", permissions.RoleAdmin)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `"organization_inactive"`, string(decodeBody(t, rec)["code"]))
}

func TestEnrollmentCodeHandler_Enroll(t *testing.T) {
	d := newCodeDeps(t)
	d.validCode()
	d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "active", Plan: "full"}, nil)
	d.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Name: "Doña Rosa"}, nil)
	d.nodes.EXPECT().FindActiveEdgeForUpdate(mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	d.nodes.EXPECT().Insert(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	d.codes.EXPECT().MarkUsed(mock.Anything, "code-1", mock.Anything).Return(nil)

	rec := d.post(t, "/api/edge/enroll-with-code", `{"code":"k7f2 9qxm","name":"Caja principal"}`, 0)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	for _, field := range []string{"node_id", "organization_id", "organization_name", "node_key", "cloud_sync_url"} {
		assert.Contains(t, body, field)
	}
}

func TestEnrollmentCodeHandler_Enroll_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(d codeDeps)
		status int
		want   string
	}{
		{"unknown code", func(d codeDeps) {
			d.codes.EXPECT().FindByHashForUpdate(mock.Anything, mock.Anything).Return(nil, domainError.ErrEnrollmentCodeInvalid)
		}, http.StatusUnauthorized, `{"error":"invalid or expired code"}`},
		{"organization inactive", func(d codeDeps) {
			d.validCode()
			d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "suspended", Plan: "full"}, nil)
		}, http.StatusForbidden, `{"error":"Organization is not active","code":"organization_inactive"}`},
		{"active box", func(d codeDeps) {
			d.validCode()
			d.access.EXPECT().GetAccess(mock.Anything, testOrganizationID).Return(&dto.OrganizationAccess{Status: "active", Plan: "full"}, nil)
			d.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID}, nil)
			d.nodes.EXPECT().FindActiveEdgeForUpdate(mock.Anything).Return(&dto.Node{ID: "old", Name: "Caja vieja", HasKey: true}, nil)
		}, http.StatusConflict, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newCodeDeps(t)
			tc.setup(d)
			rec := d.post(t, "/api/edge/enroll-with-code", `{"code":"K7F2-9QXM","name":"Caja"}`, 0)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.want != "" {
				assert.JSONEq(t, tc.want, rec.Body.String())
				return
			}
			body := decodeBody(t, rec)
			assert.JSONEq(t, `"box_already_enrolled"`, string(body["code"]))
			assert.Contains(t, string(body["active_box"]), "Caja vieja")
		})
	}
}
