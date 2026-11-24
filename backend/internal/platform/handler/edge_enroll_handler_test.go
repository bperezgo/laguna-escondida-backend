package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

const enrollOrgID = "019a0000-0000-7000-8000-0000000000a1"

type enrollRig struct {
	router *gin.Engine
	nodes  *mocks.MockNodeRepository
	jwt    *service.JWTService
}

func newEnrollRig(t *testing.T) *enrollRig {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &enrollRig{
		nodes: mocks.NewMockNodeRepository(t),
		jwt:   service.NewJWTService("test-secret", tenant.AnyOrganization()),
	}
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.On("Get", mock.Anything, enrollOrgID).Return(&dto.Organization{ID: enrollOrgID, Name: "Laguna"}, nil).Maybe()
	nodes := service.NewNodeService(r.nodes, orgs, immediateUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://api.example.com")
	r.router = gin.New()
	r.router.POST("/api/edge/enroll", JWTAuthMiddleware(r.jwt), RequirePermission(permissions.EdgeEnroll), NewEdgeEnrollHandler(nodes).EnrollHandler)
	return r
}

func (r *enrollRig) post(t *testing.T, roleID int, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/edge/enroll", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if roleID != 0 {
		token, err := r.jwt.GenerateToken("019a0000-0000-7000-8000-0000000000aa", enrollOrgID, "someone", []int{roleID})
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec
}

func TestEdgeEnrollHandler_Admin_Returns201WithTheKeyOnce(t *testing.T) {
	r := newEnrollRig(t)
	r.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	r.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	rec := r.post(t, permissions.RoleAdmin, `{"name":"Caja principal"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var got dto.EnrolledEdgeBox
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, strings.HasPrefix(got.NodeKey, "lgn_"))
	assert.Equal(t, enrollOrgID, got.OrganizationID)
	assert.Equal(t, "Laguna", got.OrganizationName)
	assert.Equal(t, "https://api.example.com", got.CloudSyncURL)
	assert.NotEmpty(t, got.NodeID)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestEdgeEnrollHandler_EmptyName_Returns400(t *testing.T) {
	r := newEnrollRig(t)

	rec := r.post(t, permissions.RoleAdmin, `{"name":"  "}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEdgeEnrollHandler_WithoutToken_Returns401(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, newEnrollRig(t).post(t, 0, `{"name":"x"}`).Code)
}

func TestEdgeEnrollHandler_Manager_Returns403(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, newEnrollRig(t).post(t, permissions.RoleManager, `{"name":"x"}`).Code)
}

func TestEdgeEnrollHandler_ActiveBox_Returns409WithIt(t *testing.T) {
	r := newEnrollRig(t)
	r.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(&dto.Node{ID: "n1", Name: "Caja principal", HasKey: true}, nil)

	rec := r.post(t, permissions.RoleAdmin, `{"name":"PC nuevo"}`)

	require.Equal(t, http.StatusConflict, rec.Code)
	var body struct {
		Code      string `json:"code"`
		ActiveBox struct {
			Name       string  `json:"name"`
			LastSeenAt *string `json:"last_seen_at"`
		} `json:"active_box"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "box_already_enrolled", body.Code)
	assert.Equal(t, "Caja principal", body.ActiveBox.Name)
	assert.Nil(t, body.ActiveBox.LastSeenAt)
}

func TestEdgeEnrollHandler_LostRace_Returns409(t *testing.T) {
	r := newEnrollRig(t)
	r.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	r.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domainError.ErrBoxAlreadyEnrolled)

	assert.Equal(t, http.StatusConflict, r.post(t, permissions.RoleAdmin, `{"name":"x"}`).Code)
}

func TestEdgeEnrollHandler_ReplaceAndAdoption_ReturnTheSyncAddress(t *testing.T) {
	cases := []struct {
		name   string
		active *dto.Node
		body   string
		insert bool
		setKey bool
		revoke bool
	}{
		{"replace", &dto.Node{ID: "n1", HasKey: true}, `{"name":"PC nuevo","replace":true}`, true, false, true},
		{"adoption", &dto.Node{ID: "n1", HasKey: false}, `{"name":"Caja"}`, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newEnrollRig(t)
			r.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(tc.active, nil)
			if tc.revoke {
				r.nodes.On("Revoke", mock.Anything, "n1", mock.Anything).Return(nil)
			}
			if tc.insert {
				r.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}
			if tc.setKey {
				r.nodes.On("SetKey", mock.Anything, "n1", "Caja", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}

			rec := r.post(t, permissions.RoleAdmin, tc.body)

			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			var got dto.EnrolledEdgeBox
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.Equal(t, "https://api.example.com", got.CloudSyncURL)
		})
	}
}
