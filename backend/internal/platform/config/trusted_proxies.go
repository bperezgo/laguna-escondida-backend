package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// loadTrustedProxies reads TRUSTED_PROXIES, comma-separated IPs or CIDRs. Unset returns nil,
// which leaves the router's default: every peer is trusted to report the client address.
func loadTrustedProxies() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw == "" {
		return nil, nil
	}
	var proxies []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(entry); err != nil && net.ParseIP(entry) == nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES entry %q is not an IP or CIDR", entry)
		}
		proxies = append(proxies, entry)
	}
	return proxies, nil
}
