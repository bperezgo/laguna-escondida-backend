package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	wompiPrivateKeyField   = "private_key"
	wompiEventsSecretField = "events_secret"
	wompiPublicKeyField    = "public_key"
	wompiIntegrityField    = "integrity_secret"
	secretHintLength       = 4

	marketingAppSigningSecretField     = "signing_secret"
	marketingAppSigningSecretMinLength = 32
	generatedSecretBytes               = 32
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
	wompiPublicKeyPrefixes = map[string]dto.IntegrationEnvironment{
		"pub_test_": dto.IntegrationEnvironmentSandbox,
		"pub_prod_": dto.IntegrationEnvironmentProduction,
	}
	wompiIntegrityPrefixes = map[string]dto.IntegrationEnvironment{
		"test_integrity_": dto.IntegrationEnvironmentSandbox,
		"prod_integrity_": dto.IntegrationEnvironmentProduction,
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
	creds.PublicKey = strings.TrimSpace(creds.PublicKey)
	creds.IntegritySecret = strings.TrimSpace(creds.IntegritySecret)
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
	hints := map[string]string{
		wompiPrivateKeyField:   secretHint(creds.PrivateKey),
		wompiEventsSecretField: secretHint(creds.EventsSecret),
	}
	if creds.QREnabled() {
		hints[wompiPublicKeyField] = secretHint(creds.PublicKey)
		hints[wompiIntegrityField] = secretHint(creds.IntegritySecret)
	}
	return s.store(ctx, orgID, dto.IntegrationProviderWompi, environment, plaintext, hints)
}

// SetMarketingApp creates or replaces the organization's marketing-app integration with a secret
// it already has, so moving it over needs no change in marketing-app.
func (s *OrganizationIntegrationService) SetMarketingApp(ctx context.Context, orgID string, creds dto.MarketingAppCredentials) (*dto.OrganizationIntegration, error) {
	if s.cipher == nil {
		return nil, domainError.ErrIntegrationsUnavailable
	}
	creds.SigningSecret = strings.TrimSpace(creds.SigningSecret)
	if err := validateMarketingAppSecret(creds.SigningSecret); err != nil {
		return nil, err
	}
	if err := s.requireOrganization(ctx, orgID); err != nil {
		return nil, err
	}
	return s.storeMarketingApp(ctx, orgID, creds)
}

// GenerateMarketingAppSecret creates or replaces the marketing-app integration with a random secret,
// returned this once. Only marketing_app has a generated secret; any other provider is unknown here.
func (s *OrganizationIntegrationService) GenerateMarketingAppSecret(ctx context.Context, orgID, provider string) (*dto.GeneratedMarketingAppSecret, error) {
	p, err := s.provider(provider)
	if err != nil {
		return nil, err
	}
	if p != dto.IntegrationProviderMarketingApp {
		return nil, domainError.ErrIntegrationProviderUnknown
	}
	if err = s.requireOrganization(ctx, orgID); err != nil {
		return nil, err
	}
	raw := make([]byte, generatedSecretBytes)
	if _, err = rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate marketing-app secret: %w", err)
	}
	creds := dto.MarketingAppCredentials{SigningSecret: base64.RawURLEncoding.EncodeToString(raw)}
	integration, err := s.storeMarketingApp(ctx, orgID, creds)
	if err != nil {
		return nil, err
	}
	return &dto.GeneratedMarketingAppSecret{OrganizationIntegration: integration, SigningSecret: creds.SigningSecret}, nil
}

func (s *OrganizationIntegrationService) storeMarketingApp(ctx context.Context, orgID string, creds dto.MarketingAppCredentials) (*dto.OrganizationIntegration, error) {
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("encode marketing-app credentials: %w", err)
	}
	return s.store(ctx, orgID, dto.IntegrationProviderMarketingApp, "", plaintext, map[string]string{
		marketingAppSigningSecretField: secretHint(creds.SigningSecret),
	})
}

func (s *OrganizationIntegrationService) store(
	ctx context.Context, orgID string, provider dto.IntegrationProvider, environment dto.IntegrationEnvironment,
	plaintext []byte, hints map[string]string,
) (*dto.OrganizationIntegration, error) {
	secrets, err := s.cipher.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt %s credentials: %w", provider, err)
	}
	now := s.now()
	stored, err := s.integrations.Upsert(tenant.WithOrg(ctx, orgID), &dto.OrganizationIntegrationRecord{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Provider:    provider,
		Environment: environment,
		Secrets:     secrets,
		SecretHints: hints,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return nil, fmt.Errorf("store %s integration: %w", provider, err)
	}
	return toIntegration(stored), nil
}

func validateMarketingAppSecret(secret string) error {
	switch {
	case secret == "":
		return &domainError.IntegrationValidationError{Field: marketingAppSigningSecretField, Reason: "is required"}
	case strings.ContainsFunc(secret, unicode.IsSpace):
		return &domainError.IntegrationValidationError{Field: marketingAppSigningSecretField, Reason: "must not contain whitespace"}
	case len(secret) < marketingAppSigningSecretMinLength:
		return &domainError.IntegrationValidationError{
			Field: marketingAppSigningSecretField, Reason: fmt.Sprintf("must be at least %d characters", marketingAppSigningSecretMinLength),
		}
	}
	return nil
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

// MarketingAppSecret is ports.MarketingEventSecrets (through MarketingAppSecrets), read on every
// call so a secret put, generated or removed takes effect on the next event.
func (s *OrganizationIntegrationService) MarketingAppSecret(ctx context.Context, organizationID string) (string, bool, error) {
	if s.cipher == nil {
		return "", false, nil
	}
	record, found, err := s.integrations.Get(tenant.WithOrg(ctx, organizationID), dto.IntegrationProviderMarketingApp)
	if err != nil {
		return "", false, fmt.Errorf("read marketing-app integration: %w", err)
	}
	if !found {
		return "", false, nil
	}
	secret, ok := s.decryptMarketingApp(record)
	if !ok {
		s.logger.ErrorContext(ctx, "integration secrets unreadable: treated as not configured; put or generate the secret again",
			slog.String("organization_id", organizationID), slog.String("provider", string(dto.IntegrationProviderMarketingApp)))
		return "", false, nil
	}
	return secret, true, nil
}

func (s *OrganizationIntegrationService) decryptMarketingApp(record *dto.OrganizationIntegrationRecord) (string, bool) {
	var creds dto.MarketingAppCredentials
	plaintext, err := s.cipher.Decrypt(record.Secrets)
	if err != nil || json.Unmarshal(plaintext, &creds) != nil {
		return "", false
	}
	return creds.SigningSecret, creds.SigningSecret != ""
}

// MarketingAppSecrets adapts the service to ports.MarketingEventSecrets; For is already PaymentLinkConfig's.
type MarketingAppSecrets struct {
	Integrations *OrganizationIntegrationService
}

func (a MarketingAppSecrets) SecretFor(ctx context.Context, organizationID string) (string, bool, error) {
	return a.Integrations.MarketingAppSecret(ctx, organizationID)
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
	if err = validateWompiQR(creds, keyEnv); err != nil {
		return "", err
	}
	return keyEnv, nil
}

// validateWompiQR checks the optional QR pair: given together, and for the private key's environment.
func validateWompiQR(creds dto.WompiCredentials, keyEnv dto.IntegrationEnvironment) error {
	switch {
	case creds.PublicKey == "" && creds.IntegritySecret == "":
		return nil
	case creds.PublicKey == "":
		return &domainError.IntegrationValidationError{Field: wompiPublicKeyField, Reason: "is required with integrity_secret"}
	case creds.IntegritySecret == "":
		return &domainError.IntegrationValidationError{Field: wompiIntegrityField, Reason: "is required with public_key"}
	}
	for _, c := range []struct {
		field, value, expected string
		prefixes               map[string]dto.IntegrationEnvironment
	}{
		{wompiPublicKeyField, creds.PublicKey, "pub_test_ or pub_prod_", wompiPublicKeyPrefixes},
		{wompiIntegrityField, creds.IntegritySecret, "test_integrity_ or prod_integrity_", wompiIntegrityPrefixes},
	} {
		env, err := environmentFromPrefix(c.field, c.value, c.prefixes, c.expected)
		if err != nil {
			return err
		}
		if env != keyEnv {
			return &domainError.IntegrationValidationError{
				Field: c.field, Reason: fmt.Sprintf("is for %s and the private key for %s", env, keyEnv),
			}
		}
	}
	return nil
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
	switch record.Provider {
	case dto.IntegrationProviderWompi:
		integration.PrivateKeyHint = record.SecretHints[wompiPrivateKeyField]
		integration.EventsSecretHint = record.SecretHints[wompiEventsSecretField]
		integration.PublicKeyHint = record.SecretHints[wompiPublicKeyField]
		integration.IntegrityHint = record.SecretHints[wompiIntegrityField]
		qr := integration.PublicKeyHint != "" && integration.IntegrityHint != ""
		integration.QREnabled = &qr
		integration.WebhookPath = "/api/webhooks/wompi/" + record.OrganizationID
	case dto.IntegrationProviderMarketingApp:
		integration.SigningSecretHint = record.SecretHints[marketingAppSigningSecretField]
	}
	return integration
}

func secretHint(secret string) string {
	if len(secret) <= secretHintLength {
		return ""
	}
	return secret[len(secret)-secretHintLength:]
}
