package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	wompiPrivateKeyPrefix   = "WOMPI_PRIVATE_KEY_"   //nolint:gosec // G101: an env var name, not a secret
	wompiEventsSecretPrefix = "WOMPI_EVENTS_SECRET_" //nolint:gosec // G101: an env var name, not a secret
	defaultWompiAPIURL      = "https://production.wompi.co/v1"
	defaultPaymentLinkTTL   = 30 * time.Minute
)

// PaymentLinks is the Wompi API shared by every organization, and each organization's private key
// and events secret from WOMPI_PRIVATE_KEY_<SLUG> and WOMPI_EVENTS_SECRET_<SLUG>.
type PaymentLinks struct {
	APIURL        string
	TTL           time.Duration
	privateKeys   map[string]string
	eventsSecrets map[string]string
}

// NewPaymentLinks is PaymentLinks keyed by organization slug, for an assembly built without the environment.
func NewPaymentLinks(apiURL string, ttl time.Duration, privateKeys, eventsSecrets map[string]string) PaymentLinks {
	links := PaymentLinks{APIURL: strings.TrimRight(apiURL, "/"), TTL: ttl, privateKeys: map[string]string{}, eventsSecrets: map[string]string{}}
	for slug, key := range privateKeys {
		links.privateKeys[marketingEventsSecretKey(slug)] = key
	}
	for slug, secret := range eventsSecrets {
		links.eventsSecrets[marketingEventsSecretKey(slug)] = secret
	}
	return links
}

// ConfiguredFor: an organization has payment links only with both its key and its secret.
func (p PaymentLinks) ConfiguredFor(slug string) bool {
	_, hasKey := p.PrivateKeyFor(slug)
	_, hasSecret := p.EventsSecretFor(slug)
	return hasKey && hasSecret
}

func (p PaymentLinks) PrivateKeyFor(slug string) (string, bool) {
	key, ok := p.privateKeys[marketingEventsSecretKey(slug)]
	return key, ok && key != ""
}

func (p PaymentLinks) EventsSecretFor(slug string) (string, bool) {
	secret, ok := p.eventsSecrets[marketingEventsSecretKey(slug)]
	return secret, ok && secret != ""
}

func loadPaymentLinks() (PaymentLinks, error) {
	links := PaymentLinks{
		APIURL:        strings.TrimRight(strings.TrimSpace(envOr("WOMPI_API_URL", defaultWompiAPIURL)), "/"),
		TTL:           defaultPaymentLinkTTL,
		privateKeys:   map[string]string{},
		eventsSecrets: map[string]string{},
	}
	parsed, err := url.Parse(links.APIURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return PaymentLinks{}, fmt.Errorf("WOMPI_API_URL must be an absolute http(s) URL, got %q", links.APIURL)
	}
	if raw := strings.TrimSpace(os.Getenv("PAYMENT_LINK_TTL")); raw != "" {
		ttl, parseErr := time.ParseDuration(raw)
		if parseErr != nil || ttl <= 0 {
			return PaymentLinks{}, fmt.Errorf("PAYMENT_LINK_TTL must be a positive duration such as 30m, got %q", raw)
		}
		links.TTL = ttl
	}
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if !found || value == "" {
			continue
		}
		switch {
		case strings.HasPrefix(name, wompiPrivateKeyPrefix):
			links.privateKeys[strings.TrimPrefix(name, wompiPrivateKeyPrefix)] = value
		case strings.HasPrefix(name, wompiEventsSecretPrefix):
			links.eventsSecrets[strings.TrimPrefix(name, wompiEventsSecretPrefix)] = value
		}
	}
	return links, nil
}
