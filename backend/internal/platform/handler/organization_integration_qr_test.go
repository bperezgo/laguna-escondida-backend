package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	qrPublicKey       = "pub_test_s3cretPublicKeyPubl"       //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow
	qrIntegritySecret = "test_integrity_s3cretIntegrity1234" //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow
)

func wompiQRBody(publicKey, integritySecret string) string {
	return `{"private_key":"` + integrationPrivateKey + `","events_secret":"` + integrationEventsSecret +
		`","public_key":"` + publicKey + `","integrity_secret":"` + integritySecret + `"}`
}

func TestOrganizationIntegrationHandler_Put_WithQRCredentials(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiQRBody(qrPublicKey, qrIntegritySecret))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	assert.JSONEq(t, `"Publ"`, string(body["public_key_hint"]))
	assert.JSONEq(t, `"1234"`, string(body["integrity_secret_hint"]))
	assert.JSONEq(t, `true`, string(body["qr_enabled"]))
	for _, secret := range []string{qrPublicKey, qrIntegritySecret, "s3cret"} {
		assert.NotContains(t, rec.Body.String(), secret)
	}
	get := serveOrganization(r.router, http.MethodGet, integrationsPath+"/wompi", "")
	assert.Contains(t, get.Body.String(), `"qr_enabled":true`)
}

func TestOrganizationIntegrationHandler_Put_WithoutQRCredentialsReadsDisabled(t *testing.T) {
	r := newIntegrationRouter(t, true)

	rec := serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiBody(integrationPrivateKey, integrationEventsSecret))

	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeBody(t, rec)
	assert.JSONEq(t, `false`, string(body["qr_enabled"]))
	assert.NotContains(t, body, "public_key_hint")
}

func TestOrganizationIntegrationHandler_Put_QRRefused(t *testing.T) {
	r := newIntegrationRouter(t, true)

	only := serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiQRBody(qrPublicKey, ""))
	mixed := serveOrganization(r.router, http.MethodPut, integrationsPath+"/wompi", wompiQRBody("pub_prod_s3cretPublicKeyPubl", qrIntegritySecret))

	require.Equal(t, http.StatusBadRequest, only.Code)
	assert.Contains(t, only.Body.String(), `"field":"integrity_secret"`)
	require.Equal(t, http.StatusBadRequest, mixed.Code)
	assert.Contains(t, mixed.Body.String(), `"field":"public_key"`)
	assert.Nil(t, r.stored, "nothing is stored")
}
