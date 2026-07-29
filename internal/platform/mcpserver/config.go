package mcpserver

import (
	"errors"
	"os"
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
	authToken := os.Getenv("MCP_AUTH_TOKEN")
	if authToken == "" {
		return nil, errors.New("MCP_AUTH_TOKEN is not set (shared secret clients send as a bearer token)")
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
	}, nil
}
