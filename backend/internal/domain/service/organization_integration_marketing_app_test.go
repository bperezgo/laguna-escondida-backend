package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lagunaCurrentSecret = "existing-marketing-secret-48-characters-long-abcd" //nolint:gosec // G101: a test fixture, not a credential. gitleaks:allow

var _ ports.MarketingEventSecrets = MarketingAppSecrets{}

func TestOrganizationIntegrationService_GenerateMarketingAppSecret(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	got, err := f.service.GenerateMarketingAppSecret(context.Background(), integrationOrgID, "marketing_app")

	require.NoError(t, err)
	assert.Equal(t, dto.IntegrationProviderMarketingApp, got.Provider)
	assert.Len(t, got.SigningSecret, 43)
	assert.NotContains(t, got.SigningSecret, "=")
	assert.NotContains(t, got.SigningSecret, "+")
	assert.NotContains(t, got.SigningSecret, "/")
	assert.Equal(t, got.SigningSecret[39:], got.SigningSecretHint)
	assert.Empty(t, got.Environment)
	assert.Empty(t, got.WebhookPath)
	assert.NotContains(t, string(f.stored[dto.IntegrationProviderMarketingApp].Secrets), got.SigningSecret, "stored encrypted")

	read, err := f.service.Get(context.Background(), integrationOrgID, "marketing_app")
	require.NoError(t, err)
	assert.Equal(t, got.SigningSecretHint, read.SigningSecretHint)

	secret, configured, err := MarketingAppSecrets{f.service}.SecretFor(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured)
	assert.Equal(t, got.SigningSecret, secret)
}

func TestOrganizationIntegrationService_GenerateMarketingAppSecret_TwiceRotates(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	first, err := f.service.GenerateMarketingAppSecret(context.Background(), integrationOrgID, "marketing_app")
	require.NoError(t, err)
	second, err := f.service.GenerateMarketingAppSecret(context.Background(), integrationOrgID, "marketing_app")
	require.NoError(t, err)

	assert.NotEqual(t, first.SigningSecret, second.SigningSecret)
	secret, _, err := f.service.MarketingAppSecret(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.Equal(t, second.SigningSecret, secret)
}

func TestOrganizationIntegrationService_GenerateMarketingAppSecret_OtherProviders(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	require.NoError(t, err)
	wompi := *f.stored[dto.IntegrationProviderWompi]

	for _, provider := range []string{"wompi", "stripe"} {
		_, err = f.service.GenerateMarketingAppSecret(context.Background(), integrationOrgID, provider)
		assert.ErrorIs(t, err, domainError.ErrIntegrationProviderUnknown, provider)
	}
	assert.Equal(t, wompi, *f.stored[dto.IntegrationProviderWompi], "Wompi integration unchanged")
	assert.NotContains(t, f.stored, dto.IntegrationProviderMarketingApp)
}

func TestOrganizationIntegrationService_SetMarketingApp_ExistingSecret(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	got, err := f.service.SetMarketingApp(context.Background(), integrationOrgID, dto.MarketingAppCredentials{SigningSecret: " " + lagunaCurrentSecret + "\n"})

	require.NoError(t, err)
	assert.Equal(t, "abcd", got.SigningSecretHint)
	secret, configured, err := f.service.MarketingAppSecret(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured)
	assert.Equal(t, lagunaCurrentSecret, secret, "trimmed, and the same value signs")
}

func TestOrganizationIntegrationService_SetMarketingApp_Refused(t *testing.T) {
	cases := map[string]string{
		"missing":    "",
		"too short":  strings.Repeat("a", 20),
		"whitespace": strings.Repeat("a", 20) + " " + strings.Repeat("b", 20),
	}
	for name, secret := range cases {
		t.Run(name, func(t *testing.T) {
			f := newIntegrationFixture(t, sandboxAPIURL)

			_, err := f.service.SetMarketingApp(context.Background(), integrationOrgID, dto.MarketingAppCredentials{SigningSecret: secret})

			assert.Equal(t, "signing_secret", validationField(t, err))
			if secret != "" {
				assert.NotContains(t, err.Error(), secret, "the error never carries the value")
			}
			assert.Empty(t, f.stored, "nothing is stored")
		})
	}
}

func TestOrganizationIntegrationService_SetMarketingApp_UnknownOrganization(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	_, err := f.service.SetMarketingApp(context.Background(), "not-a-uuid", dto.MarketingAppCredentials{SigningSecret: lagunaCurrentSecret})

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
	assert.Empty(t, f.stored)
}

func TestOrganizationIntegrationService_BothProvidersSideBySide(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	require.NoError(t, err)

	_, err = f.service.GenerateMarketingAppSecret(context.Background(), integrationOrgID, "marketing_app")
	require.NoError(t, err)

	assert.Len(t, f.stored, 2)
	creds, configured, err := f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured)
	assert.Equal(t, sandboxPrivateKey, creds.PrivateKey, "Wompi keys unchanged")
}

func TestOrganizationIntegrationService_MarketingApp_NoEncryptionKey(t *testing.T) {
	service := NewOrganizationIntegrationService(mocks.NewMockOrganizationIntegrationRepository(t), mocks.NewMockOrganizationRepository(t),
		nil, sandboxAPIURL, slog.New(slog.DiscardHandler), nil)
	ctx := context.Background()

	_, err := service.SetMarketingApp(ctx, integrationOrgID, dto.MarketingAppCredentials{SigningSecret: lagunaCurrentSecret})
	assert.ErrorIs(t, err, domainError.ErrIntegrationsUnavailable)
	_, err = service.GenerateMarketingAppSecret(ctx, integrationOrgID, "marketing_app")
	assert.ErrorIs(t, err, domainError.ErrIntegrationsUnavailable)

	_, configured, err := service.MarketingAppSecret(ctx, integrationOrgID)
	require.NoError(t, err)
	assert.False(t, configured, "no organization records marketing events")
}

func TestOrganizationIntegrationService_MarketingAppSecret_NoIntegration(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	_, configured, err := f.service.MarketingAppSecret(context.Background(), integrationOrgID)

	require.NoError(t, err)
	assert.False(t, configured)
}

func TestOrganizationIntegrationService_MarketingAppSecret_KeyReplaced(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetMarketingApp(context.Background(), integrationOrgID, dto.MarketingAppCredentials{SigningSecret: lagunaCurrentSecret})
	require.NoError(t, err)
	f.cipher.key = 0x33

	_, configured, err := f.service.MarketingAppSecret(context.Background(), integrationOrgID)

	require.NoError(t, err)
	assert.False(t, configured)
	assert.Contains(t, f.logs.String(), integrationOrgID)
	assert.Contains(t, f.logs.String(), "provider=marketing_app")
	assert.NotContains(t, f.logs.String(), "existing-marketing", "the log never carries a value")

	_, err = f.service.GenerateMarketingAppSecret(context.Background(), integrationOrgID, "marketing_app")
	require.NoError(t, err)
	_, configured, err = f.service.MarketingAppSecret(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured, "generating again restores it")
}
