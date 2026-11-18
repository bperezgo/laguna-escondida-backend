package mcpserver

import (
	"errors"
	"fmt"
	"os"
)

// Toolset selects which tools the server registers; it is fixed at startup.
type Toolset string

const (
	// ToolsetFull is every staff tool, for Claude Code and Claude Desktop.
	ToolsetFull Toolset = "full"
	// ToolsetCustomer is only the customer tools, for the customer-facing WhatsApp agent.
	ToolsetCustomer Toolset = "customer"
)

// Config holds everything the MCP server needs to talk to the backend API and
// to serve MCP over HTTP. It is intentionally small and independent from the
// backend's own config.Config: this process is only an HTTP client of the API.
type Config struct {
	// Addr is the listen address for the MCP HTTP endpoint (e.g. ":8090").
	Addr string
	// APIBaseURL is the base URL of the backend API (e.g. "http://localhost:8080").
	APIBaseURL string
	// Email (cloud) or Username (edge), with Password, authenticate the full toolset's service
	// account. Exactly one of Email and Username is set; all three are empty for the customer toolset.
	Email    string
	Username string
	Password string
	// AuthToken is the shared secret clients of the full toolset present as a bearer token on
	// the MCP endpoint. The customer toolset takes each caller's organization API key instead.
	AuthToken string
	// AdminAPIKey is optional; only the *-document-urls admin tools require it.
	AdminAPIKey string
	// Toolset is MCP_TOOLSET: full (default) or customer.
	Toolset Toolset
}

// ValidateTransport refuses the customer toolset over stdio: stdio carries no bearer token and
// no per-request headers, so the customer's phone and location could not come from the channel.
func (c *Config) ValidateTransport(stdio bool) error {
	if stdio && c.Toolset == ToolsetCustomer {
		return errors.New("MCP_TOOLSET=customer is only served over HTTP; do not pass --stdio")
	}
	return nil
}

// NewConfig reads the MCP server configuration from the environment. The customer toolset holds
// no identity of its own, so its service-account and shared-secret settings are ignored.
func NewConfig() (*Config, error) {
	base := os.Getenv("LAGUNA_API_URL")
	if base == "" {
		return nil, errors.New("LAGUNA_API_URL is not set (e.g. http://localhost:8080)")
	}

	toolset := Toolset(os.Getenv("MCP_TOOLSET"))
	switch toolset {
	case "":
		toolset = ToolsetFull
	case ToolsetFull, ToolsetCustomer:
	default:
		return nil, fmt.Errorf("MCP_TOOLSET %q is unknown (use %q or %q)", toolset, ToolsetFull, ToolsetCustomer)
	}

	addr := os.Getenv("MCP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	cfg := &Config{
		Addr:        addr,
		APIBaseURL:  base,
		AdminAPIKey: os.Getenv("LAGUNA_ADMIN_API_KEY"),
		Toolset:     toolset,
	}
	if toolset == ToolsetCustomer {
		return cfg, nil
	}

	cfg.Email = os.Getenv("LAGUNA_EMAIL")
	cfg.Username = os.Getenv("LAGUNA_USERNAME")
	if (cfg.Email == "") == (cfg.Username == "") {
		return nil, errors.New("set exactly one of LAGUNA_EMAIL (cloud) or LAGUNA_USERNAME (edge)")
	}
	cfg.Password = os.Getenv("LAGUNA_PASSWORD")
	if cfg.Password == "" {
		return nil, errors.New("LAGUNA_PASSWORD is not set")
	}
	// AuthToken guards the HTTP endpoint only; the stdio transport is local and
	// needs no shared secret, so main enforces this per-transport.
	cfg.AuthToken = os.Getenv("MCP_AUTH_TOKEN")
	return cfg, nil
}
