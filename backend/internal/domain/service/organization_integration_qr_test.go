package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sandboxPublicKey = "pub_test_abcdefpubl"       //nolint:gosec // G101: a test fixture, not a credential
	sandboxIntegrity = "test_integrity_abcdef1234" //nolint:gosec // G101: a test fixture, not a credential
	prodPublicKey    = "pub_prod_abcdefpubl"       //nolint:gosec // G101: a test fixture, not a credential
	prodIntegrity    = "prod_integrity_abcdef1234" //nolint:gosec // G101: a test fixture, not a credential
)

func TestOrganizationIntegrationService_SetWompi_QRCredentials(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	got, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{
		PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents, PublicKey: " " + sandboxPublicKey, IntegritySecret: sandboxIntegrity,
	})

	require.NoError(t, err)
	assert.Equal(t, "wxyz", got.PrivateKeyHint)
	assert.Equal(t, "9876", got.EventsSecretHint)
	assert.Equal(t, "publ", got.PublicKeyHint)
	assert.Equal(t, "1234", got.IntegrityHint)
	require.NotNil(t, got.QREnabled)
	assert.True(t, *got.QREnabled)
	stored := string(f.stored[dto.IntegrationProviderWompi].Secrets)
	assert.NotContains(t, stored, sandboxPublicKey, "stored encrypted")
	assert.NotContains(t, stored, sandboxIntegrity, "stored encrypted")

	creds, configured, err := f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured)
	assert.Equal(t, sandboxPublicKey, creds.PublicKey)
	assert.Equal(t, sandboxIntegrity, creds.IntegritySecret)
	assert.True(t, creds.QREnabled())
}

func TestOrganizationIntegrationService_SetWompi_WithoutQRCredentials(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	got, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})

	require.NoError(t, err)
	require.NotNil(t, got.QREnabled)
	assert.False(t, *got.QREnabled)
	assert.Empty(t, got.PublicKeyHint)
	assert.Empty(t, got.IntegrityHint)
	creds, configured, err := f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured, "an integration saved without the QR keys still makes links")
	assert.False(t, creds.QREnabled())
}

func TestOrganizationIntegrationService_SetWompi_ReplacingDropsQRCredentials(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{
		PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents, PublicKey: sandboxPublicKey, IntegritySecret: sandboxIntegrity,
	})
	require.NoError(t, err)

	got, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})

	require.NoError(t, err)
	assert.False(t, *got.QREnabled)
	creds, _, err := f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.False(t, creds.QREnabled())
}

func TestOrganizationIntegrationService_SetWompi_QRRefused(t *testing.T) {
	base := dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents}
	with := func(pub, integrity string) dto.WompiCredentials {
		c := base
		c.PublicKey, c.IntegritySecret = pub, integrity
		return c
	}
	tests := []struct {
		name  string
		creds dto.WompiCredentials
		field string
	}{
		{"only the public key", with(sandboxPublicKey, ""), "integrity_secret"},
		{"only the integrity secret", with("", sandboxIntegrity), "public_key"},
		{"a private key as public key", with(sandboxPrivateKey, sandboxIntegrity), "public_key"},
		{"an events secret as integrity secret", with(sandboxPublicKey, sandboxEvents), "integrity_secret"},
		{"mixed environments: public key", with(prodPublicKey, sandboxIntegrity), "public_key"},
		{"mixed environments: integrity secret", with(sandboxPublicKey, prodIntegrity), "integrity_secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newIntegrationFixture(t, sandboxAPIURL)

			_, err := f.service.SetWompi(context.Background(), integrationOrgID, tt.creds)

			assert.Equal(t, tt.field, validationField(t, err))
			assert.NotContains(t, err.Error(), "abcdef", "the error never carries a value")
			assert.Empty(t, f.stored, "nothing is stored")
		})
	}
}
