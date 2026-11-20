package config

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
)

// ignoredMarketingEventsSecretPrefix held per-organization secrets before they moved to the
// marketing_app organization integration.
const ignoredMarketingEventsSecretPrefix = "MARKETING_APP_EVENTS_SECRET_" //nolint:gosec // G101: an env var name, not a secret

// MarketingEvents is where events for marketing-app go. Each organization's signing secret is
// its marketing_app integration.
type MarketingEvents struct {
	URL string
	// IgnoredEnv names leftover per-organization secret variables, for a startup warning. Never values.
	IgnoredEnv []string
}

// Enabled: without the URL nothing is recorded or sent.
func (m MarketingEvents) Enabled() bool {
	return m.URL != ""
}

func loadMarketingEvents() (MarketingEvents, error) {
	events := MarketingEvents{URL: strings.TrimRight(strings.TrimSpace(os.Getenv("MARKETING_APP_EVENTS_URL")), "/")}
	if events.URL != "" {
		parsed, err := url.Parse(events.URL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return MarketingEvents{}, fmt.Errorf("MARKETING_APP_EVENTS_URL must be an absolute http(s) URL, got %q", events.URL)
		}
	}
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if found && value != "" && strings.HasPrefix(name, ignoredMarketingEventsSecretPrefix) {
			events.IgnoredEnv = append(events.IgnoredEnv, name)
		}
	}
	sort.Strings(events.IgnoredEnv)
	return events, nil
}
