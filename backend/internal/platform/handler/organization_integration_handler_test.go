package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/crypto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	integrationPrivateKey   = "prv_test_s3cretPrivateKeyWxyz" //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow
	integrationEventsSecret = "test_events_s3cretEvents9876"  //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow
	integrationsPath        = "/api/platform/organizations/" + testOrganizationID + "/integrations"
)

type integrationRouter struct {
	router *gin.Engine
	repo   *mocks.MockOrganizationIntegrationRepository
	orgs   *mocks.MockOrganizationRepository
	stored *dto.OrganizationIntegrationRecord
}

func newIntegrationRouter(t *testing.T, withKey bool) *integrationRouter {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &integrationRouter{repo: mocks.NewMockOrganizationIntegrationRepository(t), orgs: mocks.NewMockOrganizationRepository(t)}
	r.orgs.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID}, nil).Maybe()
	r.orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound).Maybe()
	r.repo.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec *dto.OrganizationIntegrationRecord) (*dto.OrganizationIntegrationRecord, error) {
			stored := *rec
			stored.OrganizationID = testOrganizationID
			r.stored = &stored
			return &stored, nil
		}).Maybe()
	r.repo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, dto.IntegrationProvider) (*dto.OrganizationIntegrationRecord, bool, error) {
			return r.stored, r.stored != nil, nil
		}).Maybe()
	r.repo.EXPECT().List(mock.Anything).RunAndReturn(func(context.Context) ([]*dto.OrganizationIntegrationRecord, error) {
		if r.stored == nil {
			return nil, nil
		}
		return []*dto.OrganizationIntegrationRecord{r.stored}, nil
	}).Maybe()
	var cipher ports.SecretCipher
	if withKey {
		var err error
		cipher, err = crypto.NewAESGCMCipher(make([]byte, 32))
		require.NoError(t, err)
	}
	h := NewOrganizationIntegrationHandler(service.NewOrganizationIntegrationService(
		r.repo, r.orgs, cipher, "https://sandbox.wompi.co/v1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil))
	r.router = gin.New()
	r.router.GET("/api/platform/organizations/:id/integrations", h.ListHandler)
	r.router.GET("/api/platform/organizations/:id/integrations/:provider", h.GetHandler)
	r.router.PUT("/api/platform/organizations/:id/integrations/:provider", h.PutHandler)
	r.router.DELETE("/api/platform/organizations/:id/integrations/:provider", h.DeleteHandler)
	return r
}

func wompiBody(privateKey, eventsSecret string) string {
	return `{"private_key":"` + privateKey + `","events_secret":"` + eventsSecret + `"}`
}

func assertNoSecret(t *testing.T, body string) {
	t.Helper()
	for _, secret := range []string{integrationPrivateKey, integrationEventsSecret, "s3cret"} {
		assert.NotContains(t, body, secret)
	}
}

func TestOrganizationIntegrationHandler_Put_SandboxKeys(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiBody(integrationPrivateKey, integrationEventsSecret))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	assert.JSONEq(t, `"wompi"`, string(body["provider"]))
	assert.JSONEq(t, `"sandbox"`, string(body["environment"]))
	assert.JSONEq(t, `"Wxyz"`, string(body["private_key_hint"]))
	assert.JSONEq(t, `"9876"`, string(body["events_secret_hint"]))
	assert.JSONEq(t, `"/api/webhooks/wompi/`+testOrganizationID+`"`, string(body["webhook_path"]))
	assert.Contains(t, body, "created_at")
	assert.Contains(t, body, "updated_at")
	assertNoSecret(t, rec.Body.String())
}

func TestOrganizationIntegrationHandler_NoResponseCarriesASecret(t *testing.T) {
	r := newIntegrationRouter(t, true)
	r.repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderWompi).Return(true, nil).Once()
	put := serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiBody(integrationPrivateKey, integrationEventsSecret))
	require.Equal(t, http.StatusOK, put.Code)

	for _, rec := range []string{
		put.Body.String(),
		serveOrganization(r.router, http.MethodGet, integrationsPath, "").Body.String(),
		serveOrganization(r.router, http.MethodGet, integrationsPath+"/wompi", "").Body.String(),
		serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiBody("pub_test_s3cretPublic", integrationEventsSecret)).Body.String(),
		serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiBody("prv_prod_s3cretProd", "prod_events_s3cretProd")).Body.String(),
		serveOrganization(r.router, http.MethodDelete, integrationsPath+"/wompi", "").Body.String(),
	} {
		assertNoSecret(t, rec)
		assert.NotContains(t, rec, "secrets")
	}
}

func TestOrganizationIntegrationHandler_Put_Refused(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   string
		status int
		field  string
	}{
		{"production keys on the sandbox", "/wompi", wompiBody("prv_prod_abc1234", "prod_events_abc1234"), http.StatusBadRequest, ""},
		{"mixed environments", "/wompi", wompiBody("prv_test_abc1234", "prod_events_abc1234"), http.StatusBadRequest, ""},
		{"a public key by mistake", "/wompi", wompiBody("pub_test_abc1234", "test_events_abc1234"), http.StatusBadRequest, "private_key"},
		{"empty events secret", "/wompi", wompiBody("prv_test_abc1234", " "), http.StatusBadRequest, "events_secret"},
		{"not json", "/wompi", `{`, http.StatusBadRequest, ""},
		{"unknown provider", "/stripe", wompiBody("prv_test_abc1234", "test_events_abc1234"), http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newIntegrationRouter(t, true)

			rec := serveOrganization(r.router, http.MethodPut, integrationsPath+tt.path, tt.body)

			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			if tt.field != "" {
				assert.JSONEq(t, `"`+tt.field+`"`, string(decodeBody(t, rec)["field"]))
			}
			assert.Nil(t, r.stored, "nothing is stored")
		})
	}
}

func TestOrganizationIntegrationHandler_Put_UnknownOrganization(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPut, "/api/platform/organizations/019a0000-0000-7000-8000-0000000000ff/integrations/wompi",
		wompiBody(integrationPrivateKey, integrationEventsSecret))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Nil(t, r.stored)
}

func TestOrganizationIntegrationHandler_NoEncryptionKey(t *testing.T) {
	r := newIntegrationRouter(t, false)

	for _, req := range [][3]string{
		{http.MethodPut, integrationsPath + "/wompi", wompiBody(integrationPrivateKey, integrationEventsSecret)},
		{http.MethodGet, integrationsPath, ""},
		{http.MethodGet, integrationsPath + "/wompi", ""},
		{http.MethodDelete, integrationsPath + "/wompi", ""},
	} {
		rec := serveOrganization(r.router, req[0], req[1], req[2])

		require.Equal(t, http.StatusServiceUnavailable, rec.Code, req[0]+" "+req[1])
		assert.JSONEq(t, `"INTEGRATIONS_UNAVAILABLE"`, string(decodeBody(t, rec)["code"]))
	}
	assert.Nil(t, r.stored)
}

func TestOrganizationIntegrationHandler_NoneYet(t *testing.T) {
	r := newIntegrationRouter(t, true)

	list := serveOrganization(r.router, http.MethodGet, integrationsPath, "")
	require.Equal(t, http.StatusOK, list.Code)
	assert.JSONEq(t, `{"integrations":[]}`, list.Body.String())

	assert.Equal(t, http.StatusNotFound, serveOrganization(r.router, http.MethodGet, integrationsPath+"/wompi", "").Code)
	assert.Equal(t, http.StatusNotFound, serveOrganization(r.router, http.MethodGet, integrationsPath+"/stripe", "").Code)
}

func TestOrganizationIntegrationHandler_Delete(t *testing.T) {
	r := newIntegrationRouter(t, true)
	r.repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderWompi).Return(true, nil).Once()
	r.repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderWompi).Return(false, nil).Once()

	assert.Equal(t, http.StatusNoContent, serveOrganization(r.router, http.MethodDelete, integrationsPath+"/wompi", "").Code)
	rec := serveOrganization(r.router, http.MethodDelete, integrationsPath+"/wompi", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "Integration not found"))
}

func TestOrganizationIntegrationHandler_RequestLogNeverCarriesASecret(t *testing.T) {
	r := newIntegrationRouter(t, true)
	var logs strings.Builder
	logged := gin.New()
	logged.Use(LoggerMiddleware(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))))
	logged.Any("/*path", func(c *gin.Context) { r.router.HandleContext(c) })

	serveOrganization(logged, http.MethodPut, integrationsPath+"/wompi", wompiBody(integrationPrivateKey, integrationEventsSecret))
	serveOrganization(logged, http.MethodPut, integrationsPath+"/wompi", wompiBody("pub_test_s3cretPublic", integrationEventsSecret))

	require.NotEmpty(t, logs.String(), "the requests were logged")
	assertNoSecret(t, logs.String())
}
