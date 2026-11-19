package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	sandboxAPIURL     = "https://sandbox.wompi.co/v1"
	productionAPIURL  = "https://production.wompi.co/v1"
	sandboxPrivateKey = "prv_test_abcdefwxyz"    //nolint:gosec // G101: a test fixture, not a credential
	sandboxEvents     = "test_events_abcdef9876" //nolint:gosec // G101: a test fixture, not a credential
	prodPrivateKey    = "prv_prod_abcdefwxyz"    //nolint:gosec // G101: a test fixture, not a credential
	prodEvents        = "prod_events_abcdef9876" //nolint:gosec // G101: a test fixture, not a credential
	integrationOrgID  = crmOrgID
)

// xorCipher is a reversible stand-in for AES-GCM whose key can be swapped to simulate a lost key.
type xorCipher struct{ key byte }

func (c *xorCipher) Encrypt(plaintext []byte) ([]byte, error) {
	out := make([]byte, len(plaintext)+1)
	out[0] = c.key
	for i, b := range plaintext {
		out[i+1] = b ^ c.key
	}
	return out, nil
}

func (c *xorCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 || ciphertext[0] != c.key {
		return nil, errors.New("message authentication failed")
	}
	out := make([]byte, len(ciphertext)-1)
	for i, b := range ciphertext[1:] {
		out[i] = b ^ c.key
	}
	return out, nil
}

type integrationFixture struct {
	repo    *mocks.MockOrganizationIntegrationRepository
	orgs    *mocks.MockOrganizationRepository
	cipher  *xorCipher
	logs    *bytes.Buffer
	stored  map[dto.IntegrationProvider]*dto.OrganizationIntegrationRecord
	service *OrganizationIntegrationService
}

func newIntegrationFixture(t *testing.T, wompiAPIURL string) *integrationFixture {
	t.Helper()
	f := &integrationFixture{
		repo:   mocks.NewMockOrganizationIntegrationRepository(t),
		orgs:   mocks.NewMockOrganizationRepository(t),
		cipher: &xorCipher{key: 0x5a},
		logs:   &bytes.Buffer{},
		stored: map[dto.IntegrationProvider]*dto.OrganizationIntegrationRecord{},
	}
	f.orgs.EXPECT().Get(mock.Anything, integrationOrgID).Return(&dto.Organization{ID: integrationOrgID, Slug: crmOrgSlug}, nil).Maybe()
	f.repo.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, rec *dto.OrganizationIntegrationRecord) (*dto.OrganizationIntegrationRecord, error) {
			orgID, err := tenant.RequireOrg(ctx)
			if err != nil {
				return nil, err
			}
			stored := *rec
			stored.OrganizationID = orgID
			if existing, ok := f.stored[rec.Provider]; ok {
				stored.ID, stored.CreatedAt = existing.ID, existing.CreatedAt
			}
			f.stored[rec.Provider] = &stored
			return &stored, nil
		}).Maybe()
	f.repo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p dto.IntegrationProvider) (*dto.OrganizationIntegrationRecord, bool, error) {
			rec, ok := f.stored[p]
			return rec, ok, nil
		}).Maybe()
	logger := slog.New(slog.NewTextHandler(f.logs, nil))
	f.service = NewOrganizationIntegrationService(f.repo, f.orgs, f.cipher, wompiAPIURL, logger,
		func() time.Time { return time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC) })
	return f
}

func validationField(t *testing.T, err error) string {
	t.Helper()
	var validation *domainError.IntegrationValidationError
	require.ErrorAs(t, err, &validation)
	return validation.Field
}

func TestOrganizationIntegrationService_SetWompi_SandboxKeys(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	got, err := f.service.SetWompi(context.Background(), integrationOrgID,
		dto.WompiCredentials{PrivateKey: "  " + sandboxPrivateKey + " ", EventsSecret: sandboxEvents})

	require.NoError(t, err)
	assert.Equal(t, dto.IntegrationProviderWompi, got.Provider)
	assert.Equal(t, dto.IntegrationEnvironmentSandbox, got.Environment)
	assert.Equal(t, "wxyz", got.PrivateKeyHint)
	assert.Equal(t, "9876", got.EventsSecretHint)
	assert.Equal(t, "/api/webhooks/wompi/"+integrationOrgID, got.WebhookPath)
	assert.NotContains(t, string(f.stored[dto.IntegrationProviderWompi].Secrets), sandboxPrivateKey, "stored encrypted")

	creds, configured, err := f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured, "the organization has payment links configured")
	assert.Equal(t, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents}, creds)
}

func TestOrganizationIntegrationService_SetWompi_TwiceReplaces(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	require.NoError(t, err)

	_, err = f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: "prv_test_second1111", EventsSecret: "test_events_second2222"})
	require.NoError(t, err)

	require.Len(t, f.stored, 1)
	creds, _, err := f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.Equal(t, "prv_test_second1111", creds.PrivateKey)
}

func TestOrganizationIntegrationService_SetWompi_Refused(t *testing.T) {
	tests := []struct {
		name      string
		apiURL    string
		creds     dto.WompiCredentials
		field     string
		reasonHas string
	}{
		{"production keys on the sandbox", sandboxAPIURL, dto.WompiCredentials{PrivateKey: prodPrivateKey, EventsSecret: prodEvents}, "", "production"},
		{"sandbox keys on production", productionAPIURL, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents}, "", "sandbox"},
		{"mixed environments", sandboxAPIURL, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: prodEvents}, "", "events secret"},
		{"a public key by mistake", sandboxAPIURL, dto.WompiCredentials{PrivateKey: "pub_test_abcdefwxyz", EventsSecret: sandboxEvents}, "private_key", "prv_test_"},
		{"no private key", sandboxAPIURL, dto.WompiCredentials{PrivateKey: "   ", EventsSecret: sandboxEvents}, "private_key", "required"},
		{"no events secret", sandboxAPIURL, dto.WompiCredentials{PrivateKey: sandboxPrivateKey}, "events_secret", "required"},
		{"an integrity secret as events secret", sandboxAPIURL, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: "test_integrity_abc"}, "events_secret", "test_events_"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newIntegrationFixture(t, tt.apiURL)

			_, err := f.service.SetWompi(context.Background(), integrationOrgID, tt.creds)

			assert.Equal(t, tt.field, validationField(t, err))
			assert.Contains(t, err.Error(), tt.reasonHas)
			assert.NotContains(t, err.Error(), "abcdef", "the error never carries a value")
			assert.Empty(t, f.stored, "nothing is stored")
		})
	}
}

func TestOrganizationIntegrationService_SetWompi_LocalWompiAcceptsEither(t *testing.T) {
	for _, creds := range []dto.WompiCredentials{
		{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents},
		{PrivateKey: prodPrivateKey, EventsSecret: prodEvents},
	} {
		f := newIntegrationFixture(t, "http://127.0.0.1:41234/v1")

		_, err := f.service.SetWompi(context.Background(), integrationOrgID, creds)

		require.NoError(t, err)
	}
}

func TestOrganizationIntegrationService_SetWompi_UnknownOrganization(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	other := "019a0000-0000-7000-8000-0000000000ff"
	f.orgs.EXPECT().Get(mock.Anything, other).Return(nil, domainError.ErrOrganizationNotFound).Once()

	_, err := f.service.SetWompi(context.Background(), other, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)

	_, err = f.service.SetWompi(context.Background(), "not-a-uuid", dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
	assert.Empty(t, f.stored)
}

func TestOrganizationIntegrationService_NoEncryptionKey(t *testing.T) {
	repo := mocks.NewMockOrganizationIntegrationRepository(t)
	service := NewOrganizationIntegrationService(repo, mocks.NewMockOrganizationRepository(t), nil, sandboxAPIURL, slog.New(slog.DiscardHandler), nil)
	ctx := context.Background()

	_, err := service.SetWompi(ctx, integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	assert.ErrorIs(t, err, domainError.ErrIntegrationsUnavailable)
	_, err = service.List(ctx, integrationOrgID)
	assert.ErrorIs(t, err, domainError.ErrIntegrationsUnavailable)
	_, err = service.Get(ctx, integrationOrgID, "wompi")
	assert.ErrorIs(t, err, domainError.ErrIntegrationsUnavailable)
	assert.ErrorIs(t, service.Delete(ctx, integrationOrgID, "wompi"), domainError.ErrIntegrationsUnavailable)

	_, configured, err := service.For(ctx, integrationOrgID)
	require.NoError(t, err)
	assert.False(t, configured, "no organization has payment links configured")
}

func TestOrganizationIntegrationService_For_KeyReplaced(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	require.NoError(t, err)
	f.cipher.key = 0x33

	_, configured, err := f.service.For(context.Background(), integrationOrgID)

	require.NoError(t, err)
	assert.False(t, configured)
	assert.Contains(t, f.logs.String(), integrationOrgID)
	assert.Contains(t, f.logs.String(), "provider=wompi")
	assert.NotContains(t, f.logs.String(), "abcdef", "the log never carries a value")

	_, err = f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	require.NoError(t, err)
	_, configured, err = f.service.For(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.True(t, configured, "putting the keys again restores it")
}

func TestOrganizationIntegrationService_For_NoIntegration(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	_, configured, err := f.service.For(context.Background(), integrationOrgID)

	require.NoError(t, err)
	assert.False(t, configured)
}

func TestOrganizationIntegrationService_For_ReadFailure(t *testing.T) {
	repo := mocks.NewMockOrganizationIntegrationRepository(t)
	repo.EXPECT().Get(mock.Anything, dto.IntegrationProviderWompi).Return(nil, false, errors.New("db down")).Once()
	service := NewOrganizationIntegrationService(repo, mocks.NewMockOrganizationRepository(t), &xorCipher{key: 1}, sandboxAPIURL, slog.New(slog.DiscardHandler), nil)

	_, _, err := service.For(context.Background(), integrationOrgID)

	require.Error(t, err)
}

func TestOrganizationIntegrationService_GetAndList(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
	require.NoError(t, err)
	f.repo.EXPECT().List(mock.Anything).Return([]*dto.OrganizationIntegrationRecord{f.stored[dto.IntegrationProviderWompi]}, nil).Once()

	list, err := f.service.List(context.Background(), integrationOrgID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "wxyz", list[0].PrivateKeyHint)

	got, err := f.service.Get(context.Background(), integrationOrgID, "wompi")
	require.NoError(t, err)
	assert.Equal(t, dto.IntegrationEnvironmentSandbox, got.Environment)
	assert.Equal(t, time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC), got.UpdatedAt)
}

func TestOrganizationIntegrationService_Get_NoneYet(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	f.repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()

	list, err := f.service.List(context.Background(), integrationOrgID)
	require.NoError(t, err)
	assert.Empty(t, list)

	_, err = f.service.Get(context.Background(), integrationOrgID, "wompi")
	assert.ErrorIs(t, err, domainError.ErrIntegrationNotFound)
}

func TestOrganizationIntegrationService_UnknownProvider(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	_, err := f.service.Get(context.Background(), integrationOrgID, "stripe")
	assert.ErrorIs(t, err, domainError.ErrIntegrationProviderUnknown)
	assert.ErrorIs(t, f.service.Delete(context.Background(), integrationOrgID, "stripe"), domainError.ErrIntegrationProviderUnknown)
}

func TestOrganizationIntegrationService_Delete(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)
	f.repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderWompi).Return(true, nil).Once()
	f.repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderWompi).Return(false, nil).Once()

	require.NoError(t, f.service.Delete(context.Background(), integrationOrgID, "wompi"))
	assert.ErrorIs(t, f.service.Delete(context.Background(), integrationOrgID, "wompi"), domainError.ErrIntegrationNotFound)
}

func TestOrganizationIntegrationService_WritesUnderTheOrganization(t *testing.T) {
	f := newIntegrationFixture(t, sandboxAPIURL)

	_, err := f.service.SetWompi(context.Background(), integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})

	require.NoError(t, err)
	assert.Equal(t, integrationOrgID, f.stored[dto.IntegrationProviderWompi].OrganizationID)
	assert.False(t, strings.Contains(string(f.stored[dto.IntegrationProviderWompi].Secrets), sandboxEvents))
}

func TestOrganizationIntegrationService_RepositoryFailures(t *testing.T) {
	boom := errors.New("db down")
	ctx := context.Background()
	newService := func(t *testing.T) (*OrganizationIntegrationService, *mocks.MockOrganizationIntegrationRepository) {
		repo := mocks.NewMockOrganizationIntegrationRepository(t)
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, integrationOrgID).Return(&dto.Organization{ID: integrationOrgID}, nil).Maybe()
		return NewOrganizationIntegrationService(repo, orgs, &xorCipher{key: 1}, sandboxAPIURL, slog.New(slog.DiscardHandler), nil), repo
	}

	t.Run("upsert", func(t *testing.T) {
		service, repo := newService(t)
		repo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil, boom).Once()
		_, err := service.SetWompi(ctx, integrationOrgID, dto.WompiCredentials{PrivateKey: sandboxPrivateKey, EventsSecret: sandboxEvents})
		assert.ErrorIs(t, err, boom)
	})
	t.Run("get", func(t *testing.T) {
		service, repo := newService(t)
		repo.EXPECT().Get(mock.Anything, dto.IntegrationProviderWompi).Return(nil, false, boom).Once()
		_, err := service.Get(ctx, integrationOrgID, "wompi")
		assert.ErrorIs(t, err, boom)
	})
	t.Run("list", func(t *testing.T) {
		service, repo := newService(t)
		repo.EXPECT().List(mock.Anything).Return(nil, boom).Once()
		_, err := service.List(ctx, integrationOrgID)
		assert.ErrorIs(t, err, boom)
	})
	t.Run("delete", func(t *testing.T) {
		service, repo := newService(t)
		repo.EXPECT().Delete(mock.Anything, dto.IntegrationProviderWompi).Return(false, boom).Once()
		assert.ErrorIs(t, service.Delete(ctx, integrationOrgID, "wompi"), boom)
	})
}
