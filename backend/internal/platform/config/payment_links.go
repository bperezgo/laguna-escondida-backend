package config

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	defaultWompiAPIURL    = "https://production.wompi.co/v1"
	defaultPaymentLinkTTL = 30 * time.Minute
	defaultPaymentQRWait  = 5 * time.Second
)

// ignoredWompiEnvPrefixes held per-organization keys before they moved to organization integrations.
var ignoredWompiEnvPrefixes = []string{"WOMPI_PRIVATE_KEY_", "WOMPI_EVENTS_SECRET_"}

// PaymentLinks is the Wompi API and link lifetime shared by every organization. Each
// organization's keys are its Wompi integration.
type PaymentLinks struct {
	APIURL string
	TTL    time.Duration
	// QRWait is how long creating an order waits for Wompi to return its QR.
	QRWait time.Duration
	// IgnoredEnv names leftover per-organization key variables, for a startup warning. Never values.
	IgnoredEnv []string
}

func NewPaymentLinks(apiURL string, ttl time.Duration) PaymentLinks {
	return PaymentLinks{APIURL: strings.TrimRight(apiURL, "/"), TTL: ttl, QRWait: defaultPaymentQRWait}
}

func loadPaymentLinks() (PaymentLinks, error) {
	links := NewPaymentLinks(strings.TrimSpace(envOr("WOMPI_API_URL", defaultWompiAPIURL)), defaultPaymentLinkTTL)
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
	if raw := strings.TrimSpace(os.Getenv("PAYMENT_QR_WAIT")); raw != "" {
		wait, parseErr := time.ParseDuration(raw)
		if parseErr != nil || wait <= 0 {
			return PaymentLinks{}, fmt.Errorf("PAYMENT_QR_WAIT must be a positive duration such as 5s, got %q", raw)
		}
		links.QRWait = wait
	}
	links.IgnoredEnv = ignoredWompiEnv()
	return links, nil
}

func ignoredWompiEnv() []string {
	var names []string
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if !found || value == "" {
			continue
		}
		for _, prefix := range ignoredWompiEnvPrefixes {
			if strings.HasPrefix(name, prefix) {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}
