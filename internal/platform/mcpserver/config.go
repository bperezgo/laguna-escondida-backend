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
	// Username and Password authenticate the service account used to obtain a JWT.
	Username string
	Password string
	// AuthToken is the shared secret clients must present as a bearer token on
	// the MCP endpoint (Authorization: Bearer <token>).
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

// NewConfig reads the MCP server configuration from the environment.
func NewConfig() (*Config, error) {
	base := os.Getenv("LAGUNA_API_URL")
	if base == "" {
		return nil, errors.New("LAGUNA_API_URL is not set (e.g. http://localhost:8080)")
	}
	username := os.Getenv("LAGUNA_USERNAME")
	if username == "" {
		return nil, errors.New("LAGUNA_USERNAME is not set")
	}
	password := os.Getenv("LAGUNA_PASSWORD")
	if password == "" {
		return nil, errors.New("LAGUNA_PASSWORD is not set")
	}
	// AuthToken guards the HTTP endpoint only; the stdio transport is local and
	// needs no shared secret, so main enforces this per-transport.
	authToken := os.Getenv("MCP_AUTH_TOKEN")

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

	return &Config{
		Addr:        addr,
		APIBaseURL:  base,
		Username:    username,
		Password:    password,
		AuthToken:   authToken,
		AdminAPIKey: os.Getenv("LAGUNA_ADMIN_API_KEY"),
		Toolset:     toolset,
	}, nil
}
