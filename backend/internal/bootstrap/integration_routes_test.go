package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/crypto"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	staffIntegrationsPath = "/api/organization/integrations"
	staffPrivateKey       = "prv_test_s3cretPrivateKeyWxyz" //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow
	staffEventsSecret     = "test_events_s3cretEvents9876"  //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow
	agentAPIKey           = "lgk_tttttttttttttttttttttttttttttttttttttttttttt"
	staffWompiBody        = `{"private_key":"` + staffPrivateKey + `","events_secret":"` + staffEventsSecret + `"}`
)

type staffIntegrations struct {
	router *gin.Engine
	jwt    *service.JWTService
	repo   *mocks.MockOrganizationIntegrationRepository
	stored *dto.OrganizationIntegrationRecord
	// orgs records the organization each repository call ran under.
	orgs []string
}

func newStaffIntegrations(t *testing.T, withKey bool) *staffIntegrations {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := &staffIntegrations{repo: mocks.NewMockOrganizationIntegrationRepository(t)}
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().Get(mock.Anything, testOrgID).Return(&dto.Organization{ID: testOrgID}, nil).Maybe()
	track := func(ctx context.Context) {
		org, _ := tenant.OrgFrom(ctx)
		s.orgs = append(s.orgs, org)
	}
	s.repo.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, rec *dto.OrganizationIntegrationRecord) (*dto.OrganizationIntegrationRecord, error) {
			track(ctx)
			stored := *rec
			stored.OrganizationID = testOrgID
			s.stored = &stored
			return &stored, nil
		}).Maybe()
	s.repo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, _ dto.IntegrationProvider) (*dto.OrganizationIntegrationRecord, bool, error) {
			track(ctx)
			return s.stored, s.stored != nil, nil
		}).Maybe()
	s.repo.EXPECT().List(mock.Anything).RunAndReturn(func(ctx context.Context) ([]*dto.OrganizationIntegrationRecord, error) {
		track(ctx)
		if s.stored == nil {
			return nil, nil
		}
		return []*dto.OrganizationIntegrationRecord{s.stored}, nil
	}).Maybe()
	s.repo.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ dto.IntegrationProvider) (bool, error) {
		track(ctx)
		deleted := s.stored != nil
		s.stored = nil
		return deleted, nil
	}).Maybe()

	var cipher ports.SecretCipher
	if withKey {
		var err error
		cipher, err = crypto.NewAESGCMCipher(make([]byte, 32))
		require.NoError(t, err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	integrations := service.NewOrganizationIntegrationService(s.repo, orgs, cipher, "https://sandbox.wompi.co/v1", logger, nil)

	keys := mocks.NewMockAPIKeyRepository(t)
	keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(&dto.APIKeyRecord{
		KeyID: "019a0000-0000-7000-8000-0000000000d1", KeyOrganizationID: testOrgID,
		UserID: "019a0000-0000-7000-8000-0000000000c1", Username: "agente", UserOrganizationID: testOrgID, UserActive: true,
		OrganizationStatus: dto.OrganizationStatusActive, RoleIDs: []int{permissions.RoleAgent},
	}, nil).Maybe()
	keys.EXPECT().TouchLastUsed(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(context.Context, string, time.Time) error { return nil }).Maybe()
	s.jwt = service.NewJWTService("test-secret", tenant.AnyOrganization())
	s.jwt.AcceptAPIKeys(service.NewAPIKeyService(keys, mocks.NewMockOrganizationRepository(t), mocks.NewMockUserRepository(t),
		mocks.NewMockUserRoleRepository(t), logger))

	s.router = gin.New()
	RegisterOrganizationIntegrationRoutes(s.router, handler.NewOrganizationIntegrationHandler(integrations), s.jwt)
	return s
}

func (s *staffIntegrations) token(t *testing.T, role int) string {
	t.Helper()
	token, err := s.jwt.GenerateToken("019a0000-0000-7000-8000-0000000000aa", testOrgID, "staff", []int{role})
	require.NoError(t, err)
	return token
}

func (s *staffIntegrations) serve(method, path, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

func assertNoStaffSecret(t *testing.T, body string) {
	t.Helper()
	assert.NotContains(t, body, "s3cret")
	assert.NotContains(t, body, "secrets")
}

func TestOrganizationIntegrationRoutes_AdminManagesItsOwnOrganization(t *testing.T) {
	s := newStaffIntegrations(t, true)
	admin := s.token(t, permissions.RoleAdmin)

	put := s.serve(http.MethodPut, staffIntegrationsPath+"/wompi", admin, staffWompiBody)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.Contains(t, put.Body.String(), `"environment":"sandbox"`)
	assert.Contains(t, put.Body.String(), `"private_key_hint":"Wxyz"`)
	assert.Contains(t, put.Body.String(), `"webhook_path":"/api/webhooks/wompi/`+testOrgID+`"`)

	list := s.serve(http.MethodGet, staffIntegrationsPath, admin, "")
	require.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, list.Body.String(), `"provider":"wompi"`)
	get := s.serve(http.MethodGet, staffIntegrationsPath+"/wompi", admin, "")
	require.Equal(t, http.StatusOK, get.Code)
	refused := s.serve(http.MethodPut, staffIntegrationsPath+"/wompi", admin, `{"private_key":"pub_test_s3cretPub","events_secret":"`+staffEventsSecret+`"}`)
	require.Equal(t, http.StatusBadRequest, refused.Code)
	assert.Contains(t, refused.Body.String(), `"field":"private_key"`)
	del := s.serve(http.MethodDelete, staffIntegrationsPath+"/wompi", admin, "")
	assert.Equal(t, http.StatusNoContent, del.Code)
	assert.Equal(t, http.StatusNotFound, s.serve(http.MethodGet, staffIntegrationsPath+"/wompi", admin, "").Code)

	for _, rec := range []*httptest.ResponseRecorder{put, list, get, refused, del} {
		assertNoStaffSecret(t, rec.Body.String())
	}
	for _, org := range s.orgs {
		assert.Equal(t, testOrgID, org, "every read and write runs under the token's organization")
	}
}

func TestOrganizationIntegrationRoutes_AdminGeneratesTheMarketingAppSecret(t *testing.T) {
	s := newStaffIntegrations(t, true)
	admin := s.token(t, permissions.RoleAdmin)

	generated := s.serve(http.MethodPost, staffIntegrationsPath+"/marketing_app/secret", admin, "")
	require.Equal(t, http.StatusOK, generated.Code, generated.Body.String())
	var body struct {
		Provider      string `json:"provider"`
		SigningSecret string `json:"signing_secret"`
	}
	require.NoError(t, json.Unmarshal(generated.Body.Bytes(), &body))
	assert.Equal(t, "marketing_app", body.Provider)
	assert.Len(t, body.SigningSecret, 43)

	get := s.serve(http.MethodGet, staffIntegrationsPath+"/marketing_app", admin, "")
	require.Equal(t, http.StatusOK, get.Code)
	assert.NotContains(t, get.Body.String(), body.SigningSecret)
	assert.Equal(t, http.StatusNotFound, s.serve(http.MethodPost, staffIntegrationsPath+"/wompi/secret", admin, "").Code)
	for _, org := range s.orgs {
		assert.Equal(t, testOrgID, org)
	}
}

func TestOrganizationIntegrationRoutes_OnlyAdminGetsIn(t *testing.T) {
	s := newStaffIntegrations(t, true)
	callers := map[string]string{
		"manager":    s.token(t, permissions.RoleManager),
		"accountant": s.token(t, permissions.RoleAccountant),
		"server":     s.token(t, permissions.RoleServer),
		"agent key":  agentAPIKey,
	}
	for name, bearer := range callers {
		for _, req := range [][3]string{
			{http.MethodGet, staffIntegrationsPath, ""},
			{http.MethodGet, staffIntegrationsPath + "/wompi", ""},
			{http.MethodPut, staffIntegrationsPath + "/wompi", staffWompiBody},
			{http.MethodDelete, staffIntegrationsPath + "/wompi", ""},
			{http.MethodPost, staffIntegrationsPath + "/marketing_app/secret", ""},
			{http.MethodPut, staffIntegrationsPath + "/marketing_app", `{"signing_secret":"s3cret-marketing-signing-secret-48-chars-long-zz9"}`}, // gitleaks:allow (test fixture)
		} {
			rec := s.serve(req[0], req[1], bearer, req[2])
			assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s %s", name, req[0], req[1])
			assertNoStaffSecret(t, rec.Body.String())
		}
	}
	assert.Nil(t, s.stored, "nothing changed")
	assert.Empty(t, s.orgs, "no repository call was made")
}

func TestOrganizationIntegrationRoutes_NoEncryptionKey(t *testing.T) {
	s := newStaffIntegrations(t, false)
	admin := s.token(t, permissions.RoleAdmin)

	for _, req := range [][3]string{
		{http.MethodGet, staffIntegrationsPath, ""},
		{http.MethodGet, staffIntegrationsPath + "/wompi", ""},
		{http.MethodPut, staffIntegrationsPath + "/wompi", staffWompiBody},
		{http.MethodDelete, staffIntegrationsPath + "/wompi", ""},
		{http.MethodPost, staffIntegrationsPath + "/marketing_app/secret", ""},
	} {
		rec := s.serve(req[0], req[1], admin, req[2])
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, req[0]+" "+req[1])
		assert.Contains(t, rec.Body.String(), `"code":"INTEGRATIONS_UNAVAILABLE"`)
	}
}

func TestOrganizationIntegrationRoutes_NeedASignedInUser(t *testing.T) {
	s := newStaffIntegrations(t, true)
	req := httptest.NewRequest(http.MethodGet, staffIntegrationsPath, nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
