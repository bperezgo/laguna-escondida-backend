package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const integrationSigningSecret = "s3cret-marketing-signing-secret-48-chars-long-zz9" //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow

func signingSecretBody(secret string) string {
	return `{"signing_secret":"` + secret + `"}`
}

func TestOrganizationIntegrationHandler_GenerateMarketingAppSecret(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPost, integrationsPath+"/marketing_app/secret", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var body struct {
		Provider          string  `json:"provider"`
		Environment       *string `json:"environment"`
		SigningSecret     string  `json:"signing_secret"`
		SigningSecretHint string  `json:"signing_secret_hint"`
		WebhookPath       *string `json:"webhook_path"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "marketing_app", body.Provider)
	assert.Nil(t, body.Environment)
	assert.Nil(t, body.WebhookPath)
	assert.Len(t, body.SigningSecret, 43)
	assert.Equal(t, body.SigningSecret[39:], body.SigningSecretHint)

	for _, read := range []string{
		serveOrganization(r.router, http.MethodGet, integrationsPath+"/marketing_app", "").Body.String(),
		serveOrganization(r.router, http.MethodGet, integrationsPath, "").Body.String(),
	} {
		assert.Contains(t, read, `"signing_secret_hint":"`+body.SigningSecretHint+`"`)
		assert.NotContains(t, read, body.SigningSecret, "only the generate response carries the secret")
		assert.NotContains(t, read, `"signing_secret":`)
	}
}

func TestOrganizationIntegrationHandler_PutMarketingAppSecret(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPut, integrationsPath+"/marketing_app", signingSecretBody(integrationSigningSecret))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"signing_secret_hint":"-zz9"`)
	assert.NotContains(t, rec.Body.String(), `"signing_secret":`)
	assertNoSecret(t, rec.Body.String())
	assert.Equal(t, dto.IntegrationProviderMarketingApp, r.stored.Provider)
}

func TestOrganizationIntegrationHandler_PutMarketingAppSecret_Refused(t *testing.T) {
	for name, body := range map[string]string{
		"too short":       signingSecretBody("s3cret-short-twenty1"),
		"with whitespace": signingSecretBody("s3cret with spaces in it that is long enough"),
		"missing":         `{}`,
		"wompi's body":    wompiBody(integrationPrivateKey, integrationEventsSecret),
	} {
		t.Run(name, func(t *testing.T) {
			r := newIntegrationRouter(t, true)

			rec := serveOrganization(r.router, http.MethodPut, integrationsPath+"/marketing_app", body)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.JSONEq(t, `"signing_secret"`, string(decodeBody(t, rec)["field"]))
			assertNoSecret(t, rec.Body.String())
			assert.Nil(t, r.stored, "nothing is stored")
		})
	}
}

func TestOrganizationIntegrationHandler_GenerateSecret_OtherProviders(t *testing.T) {
	r := newIntegrationRouter(t, true)

	for _, provider := range []string{"wompi", "stripe"} {
		rec := serveOrganization(r.router, http.MethodPost, integrationsPath+"/"+provider+"/secret", "")
		assert.Equal(t, http.StatusNotFound, rec.Code, provider)
	}
	assert.Nil(t, r.stored)
}

func TestOrganizationIntegrationHandler_GenerateSecret_UnknownOrganization(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPost, "/api/platform/organizations/019a0000-0000-7000-8000-0000000000ff/integrations/marketing_app/secret", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Nil(t, r.stored)
}

func TestOrganizationIntegrationHandler_MarketingApp_NoEncryptionKey(t *testing.T) {
	r := newIntegrationRouter(t, false)

	for _, req := range [][3]string{
		{http.MethodPost, integrationsPath + "/marketing_app/secret", ""},
		{http.MethodPut, integrationsPath + "/marketing_app", signingSecretBody(integrationSigningSecret)},
		{http.MethodGet, integrationsPath + "/marketing_app", ""},
	} {
		rec := serveOrganization(r.router, req[0], req[1], req[2])

		require.Equal(t, http.StatusServiceUnavailable, rec.Code, req[0]+" "+req[1])
		assert.JSONEq(t, `"INTEGRATIONS_UNAVAILABLE"`, string(decodeBody(t, rec)["code"]))
	}
	assert.Nil(t, r.stored)
}

func TestOrganizationIntegrationHandler_MarketingApp_DeleteThenNone(t *testing.T) {
	r := newIntegrationRouter(t, true)
	r.repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderMarketingApp).Return(true, nil).Once()

	assert.Equal(t, http.StatusNoContent, serveOrganization(r.router, http.MethodDelete, integrationsPath+"/marketing_app", "").Code)
}

// The logger records method, path, status and timing only, so neither the PUT body nor the
// generated secret in the POST response reaches a log line.
func TestOrganizationIntegrationHandler_MarketingApp_RequestLogNeverCarriesASecret(t *testing.T) {
	r := newIntegrationRouter(t, true)
	var logs strings.Builder
	logged := gin.New()
	logged.Use(LoggerMiddleware(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))))
	logged.Any("/*path", func(c *gin.Context) { r.router.HandleContext(c) })

	serveOrganization(logged, http.MethodPut, integrationsPath+"/marketing_app", signingSecretBody(integrationSigningSecret))
	serveOrganization(logged, http.MethodPut, integrationsPath+"/marketing_app", signingSecretBody("s3cret-short-twenty1"))
	generated := serveOrganization(logged, http.MethodPost, integrationsPath+"/marketing_app/secret", "")
	require.Equal(t, http.StatusOK, generated.Code)
	var body struct {
		SigningSecret string `json:"signing_secret"`
	}
	require.NoError(t, json.Unmarshal(generated.Body.Bytes(), &body))

	require.NotEmpty(t, logs.String(), "the requests were logged")
	assertNoSecret(t, logs.String())
	assert.NotContains(t, logs.String(), body.SigningSecret)
}
