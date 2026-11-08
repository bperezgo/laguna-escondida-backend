package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const marketingEventsSecretPrefix = "MARKETING_APP_EVENTS_SECRET_" //nolint:gosec // G101: an env var name, not a secret

// MarketingEvents is where events for marketing-app go, and each organization's signing secret,
// from MARKETING_APP_EVENTS_SECRET_<SLUG> (slug upper-cased, "-" as "_").
type MarketingEvents struct {
	URL     string
	secrets map[string]string
}

// Enabled: without the URL nothing is recorded or sent.
func (m MarketingEvents) Enabled() bool {
	return m.URL != ""
}

func (m MarketingEvents) SecretFor(slug string) (string, bool) {
	secret, ok := m.secrets[marketingEventsSecretKey(slug)]
	return secret, ok && secret != ""
}

func marketingEventsSecretKey(slug string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(slug), "-", "_"))
}

func loadMarketingEvents() (MarketingEvents, error) {
	events := MarketingEvents{URL: strings.TrimRight(strings.TrimSpace(os.Getenv("MARKETING_APP_EVENTS_URL")), "/"), secrets: map[string]string{}}
	if events.URL != "" {
		parsed, err := url.Parse(events.URL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return MarketingEvents{}, fmt.Errorf("MARKETING_APP_EVENTS_URL must be an absolute http(s) URL, got %q", events.URL)
		}
	}
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if !found || !strings.HasPrefix(name, marketingEventsSecretPrefix) || value == "" {
			continue
		}
		events.secrets[strings.TrimPrefix(name, marketingEventsSecretPrefix)] = value
	}
	return events, nil
}
