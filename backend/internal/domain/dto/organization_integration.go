package dto

import "time"

type IntegrationProvider string

const (
	IntegrationProviderWompi        IntegrationProvider = "wompi"
	IntegrationProviderMarketingApp IntegrationProvider = "marketing_app"
)

// IntegrationProviders is every provider an organization can be connected to, in list order.
var IntegrationProviders = []IntegrationProvider{IntegrationProviderMarketingApp, IntegrationProviderWompi}

type IntegrationEnvironment string

const (
	IntegrationEnvironmentSandbox    IntegrationEnvironment = "sandbox"
	IntegrationEnvironmentProduction IntegrationEnvironment = "production"
)

// OrganizationIntegration is an integration as it is read: hints of its secrets, never the secrets.
type OrganizationIntegration struct {
	Provider          IntegrationProvider    `json:"provider"`
	Environment       IntegrationEnvironment `json:"environment,omitempty"`
	PrivateKeyHint    string                 `json:"private_key_hint,omitempty"`
	EventsSecretHint  string                 `json:"events_secret_hint,omitempty"`
	SigningSecretHint string                 `json:"signing_secret_hint,omitempty"`
	WebhookPath       string                 `json:"webhook_path,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

type OrganizationIntegrationListResponse struct {
	Integrations []*OrganizationIntegration `json:"integrations"`
}

// OrganizationIntegrationRecord is the stored integration; Secrets is the ciphertext of the
// provider's credentials as JSON.
type OrganizationIntegrationRecord struct {
	ID             string
	OrganizationID string
	Provider       IntegrationProvider
	Environment    IntegrationEnvironment
	Secrets        []byte
	SecretHints    map[string]string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WompiCredentials are an organization's Wompi private key and events secret: the PUT body, and
// the plaintext of a Wompi integration's Secrets.
type WompiCredentials struct {
	PrivateKey   string `json:"private_key"`
	EventsSecret string `json:"events_secret"`
}

// MarketingAppCredentials is the PUT body of a marketing-app integration, and the plaintext of its Secrets.
type MarketingAppCredentials struct {
	SigningSecret string `json:"signing_secret"`
}

// GeneratedMarketingAppSecret is the only response that carries a secret: the one just generated.
type GeneratedMarketingAppSecret struct {
	*OrganizationIntegration
	SigningSecret string `json:"signing_secret"`
}
