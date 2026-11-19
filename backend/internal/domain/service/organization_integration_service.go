package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	wompiPrivateKeyField   = "private_key"
	wompiEventsSecretField = "events_secret"
	secretHintLength       = 4
)

// Wompi's key prefixes carry the environment a key belongs to.
var (
	wompiPrivateKeyPrefixes = map[string]dto.IntegrationEnvironment{
		"prv_test_": dto.IntegrationEnvironmentSandbox,
		"prv_prod_": dto.IntegrationEnvironmentProduction,
	}
	wompiEventsSecretPrefixes = map[string]dto.IntegrationEnvironment{
		"test_events_": dto.IntegrationEnvironmentSandbox,
		"prod_events_": dto.IntegrationEnvironmentProduction,
	}
	wompiHostEnvironments = map[string]dto.IntegrationEnvironment{
		"sandbox.wompi.co":    dto.IntegrationEnvironmentSandbox,
		"production.wompi.co": dto.IntegrationEnvironmentProduction,
	}
)

// OrganizationIntegrationService keeps organizations' provider credentials encrypted and
// write-only. A nil cipher means the deployment has no key: nothing is saved or read.
type OrganizationIntegrationService struct {
	integrations ports.OrganizationIntegrationRepository
	orgs         ports.OrganizationRepository
	cipher       ports.SecretCipher
	// wompiEnvironment is the one Wompi's API URL requires; empty accepts either (a local stand-in).
	wompiEnvironment dto.IntegrationEnvironment
	logger           *slog.Logger
	now              func() time.Time
}

func NewOrganizationIntegrationService(
	integrations ports.OrganizationIntegrationRepository,
	orgs ports.OrganizationRepository,
	cipher ports.SecretCipher,
	wompiAPIURL string,
	logger *slog.Logger,
	now func() time.Time,
) *OrganizationIntegrationService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	var wompiEnvironment dto.IntegrationEnvironment
	if parsed, err := url.Parse(wompiAPIURL); err == nil {
		wompiEnvironment = wompiHostEnvironments[strings.ToLower(parsed.Hostname())]
	}
	return &OrganizationIntegrationService{
		integrations: integrations, orgs: orgs, cipher: cipher, wompiEnvironment: wompiEnvironment, logger: logger, now: now,
	}
}

// SetWompi creates or replaces the organization's Wompi integration.
func (s *OrganizationIntegrationService) SetWompi(ctx context.Context, orgID string, creds dto.WompiCredentials) (*dto.OrganizationIntegration, error) {
	if s.cipher == nil {
		return nil, domainError.ErrIntegrationsUnavailable
	}
	creds.PrivateKey = strings.TrimSpace(creds.PrivateKey)
	creds.EventsSecret = strings.TrimSpace(creds.EventsSecret)
	environment, err := s.validateWompi(creds)
	if err != nil {
		return nil, err
	}
	if err = s.requireOrganization(ctx, orgID); err != nil {
		return nil, err
	}
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("encode wompi credentials: %w", err)
	}
	secrets, err := s.cipher.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt wompi credentials: %w", err)
	}
	now := s.now()
	stored, err := s.integrations.Upsert(tenant.WithOrg(ctx, orgID), &dto.OrganizationIntegrationRecord{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Provider:    dto.IntegrationProviderWompi,
		Environment: environment,
		Secrets:     secrets,
		SecretHints: map[string]string{
			wompiPrivateKeyField:   secretHint(creds.PrivateKey),
			wompiEventsSecretField: secretHint(creds.EventsSecret),
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("store wompi integration: %w", err)
	}
	return toIntegration(stored), nil
}

func (s *OrganizationIntegrationService) Get(ctx context.Context, orgID, provider string) (*dto.OrganizationIntegration, error) {
	p, err := s.provider(provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireOrganization(ctx, orgID); err != nil {
		return nil, err
	}
	record, found, err := s.integrations.Get(tenant.WithOrg(ctx, orgID), p)
	if err != nil {
		return nil, fmt.Errorf("read integration: %w", err)
	}
	if !found {
		return nil, domainError.ErrIntegrationNotFound
	}
	return toIntegration(record), nil
}

func (s *OrganizationIntegrationService) List(ctx context.Context, orgID string) ([]*dto.OrganizationIntegration, error) {
	if s.cipher == nil {
		return nil, domainError.ErrIntegrationsUnavailable
	}
	if err := s.requireOrganization(ctx, orgID); err != nil {
		return nil, err
	}
	records, err := s.integrations.List(tenant.WithOrg(ctx, orgID))
	if err != nil {
		return nil, fmt.Errorf("list integrations: %w", err)
	}
	list := make([]*dto.OrganizationIntegration, len(records))
	for i, record := range records {
		list[i] = toIntegration(record)
	}
	return list, nil
}

// Delete leaves links already created as they are; the organization just stops making new ones.
func (s *OrganizationIntegrationService) Delete(ctx context.Context, orgID, provider string) error {
	p, err := s.provider(provider)
	if err != nil {
		return err
	}
	if err = s.requireOrganization(ctx, orgID); err != nil {
		return err
	}
	deleted, err := s.integrations.Delete(tenant.WithOrg(ctx, orgID), p)
	if err != nil {
		return fmt.Errorf("delete integration: %w", err)
	}
	if !deleted {
		return domainError.ErrIntegrationNotFound
	}
	return nil
}

// For is ports.PaymentLinkConfig. Secrets that can't be decrypted (the key changed) count as no
// integration, so an order is refused rather than failed.
func (s *OrganizationIntegrationService) For(ctx context.Context, organizationID string) (dto.WompiCredentials, bool, error) {
	if s.cipher == nil {
		return dto.WompiCredentials{}, false, nil
	}
	record, found, err := s.integrations.Get(tenant.WithOrg(ctx, organizationID), dto.IntegrationProviderWompi)
	if err != nil {
		return dto.WompiCredentials{}, false, fmt.Errorf("read wompi integration: %w", err)
	}
	if !found {
		return dto.WompiCredentials{}, false, nil
	}
	creds, ok := s.decryptWompi(record)
	if !ok {
		s.logger.ErrorContext(ctx, "integration secrets unreadable: treated as not configured; put the keys again",
			slog.String("organization_id", organizationID), slog.String("provider", string(dto.IntegrationProviderWompi)))
		return dto.WompiCredentials{}, false, nil
	}
	return creds, true, nil
}

func (s *OrganizationIntegrationService) decryptWompi(record *dto.OrganizationIntegrationRecord) (dto.WompiCredentials, bool) {
	var creds dto.WompiCredentials
	plaintext, err := s.cipher.Decrypt(record.Secrets)
	if err != nil || json.Unmarshal(plaintext, &creds) != nil {
		return dto.WompiCredentials{}, false
	}
	return creds, creds.PrivateKey != "" && creds.EventsSecret != ""
}

func (s *OrganizationIntegrationService) validateWompi(creds dto.WompiCredentials) (dto.IntegrationEnvironment, error) {
	keyEnv, err := environmentFromPrefix(wompiPrivateKeyField, creds.PrivateKey, wompiPrivateKeyPrefixes, "prv_test_ or prv_prod_")
	if err != nil {
		return "", err
	}
	secretEnv, err := environmentFromPrefix(wompiEventsSecretField, creds.EventsSecret, wompiEventsSecretPrefixes, "test_events_ or prod_events_")
	if err != nil {
		return "", err
	}
	if keyEnv != secretEnv {
		return "", &domainError.IntegrationValidationError{
			Reason: fmt.Sprintf("the private key is for %s and the events secret for %s", keyEnv, secretEnv),
		}
	}
	if s.wompiEnvironment != "" && keyEnv != s.wompiEnvironment {
		return "", &domainError.IntegrationValidationError{
			Reason: fmt.Sprintf("the keys are for %s and this cloud uses Wompi's %s", keyEnv, s.wompiEnvironment),
		}
	}
	return keyEnv, nil
}

func environmentFromPrefix(field, value string, prefixes map[string]dto.IntegrationEnvironment, expected string) (dto.IntegrationEnvironment, error) {
	if value == "" {
		return "", &domainError.IntegrationValidationError{Field: field, Reason: "is required"}
	}
	for prefix, environment := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return environment, nil
		}
	}
	return "", &domainError.IntegrationValidationError{Field: field, Reason: "must start with " + expected}
}

func (s *OrganizationIntegrationService) provider(name string) (dto.IntegrationProvider, error) {
	if s.cipher == nil {
		return "", domainError.ErrIntegrationsUnavailable
	}
	for _, p := range dto.IntegrationProviders {
		if string(p) == name {
			return p, nil
		}
	}
	return "", domainError.ErrIntegrationProviderUnknown
}

func (s *OrganizationIntegrationService) requireOrganization(ctx context.Context, orgID string) error {
	if uuid.Validate(orgID) != nil {
		return domainError.ErrOrganizationNotFound
	}
	if _, err := s.orgs.Get(ctx, orgID); err != nil {
		return err
	}
	return nil
}

func toIntegration(record *dto.OrganizationIntegrationRecord) *dto.OrganizationIntegration {
	integration := &dto.OrganizationIntegration{
		Provider:    record.Provider,
		Environment: record.Environment,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	if record.Provider == dto.IntegrationProviderWompi {
		integration.PrivateKeyHint = record.SecretHints[wompiPrivateKeyField]
		integration.EventsSecretHint = record.SecretHints[wompiEventsSecretField]
		integration.WebhookPath = "/api/webhooks/wompi/" + record.OrganizationID
	}
	return integration
}

func secretHint(secret string) string {
	if len(secret) <= secretHintLength {
		return ""
	}
	return secret[len(secret)-secretHintLength:]
}
