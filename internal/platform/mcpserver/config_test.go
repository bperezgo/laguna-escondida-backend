package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_ExactlyOneSignInCredential(t *testing.T) {
	cases := []struct {
		name     string
		email    string
		username string
		wantErr  bool
	}{
		{name: "email for the cloud", email: "agent@example.com"},
		{name: "username for an edge box", username: "svc"},
		{name: "neither", wantErr: true},
		{name: "both", email: "agent@example.com", username: "svc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LAGUNA_API_URL", "http://localhost:8080")
			t.Setenv("LAGUNA_PASSWORD", "secret")
			t.Setenv("LAGUNA_EMAIL", tc.email)
			t.Setenv("LAGUNA_USERNAME", tc.username)

			cfg, err := NewConfig()

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.email, cfg.Email)
			assert.Equal(t, tc.username, cfg.Username)
		})
	}
}

func TestClient_SignsInWithTheConfiguredCredential(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "cloud", cfg: Config{Email: "agent@example.com", Password: "secret"}, want: `{"email": "agent@example.com", "password": "secret"}`},
		{name: "edge", cfg: Config{Username: "svc", Password: "secret"}, want: `{"username": "svc", "password": "secret"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent json.RawMessage
			mux := http.NewServeMux()
			mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&sent)
				writeSignIn(w)
			})
			mux.HandleFunc("/api/ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) })
			srv := httptest.NewServer(mux)
			defer srv.Close()
			tc.cfg.APIBaseURL = srv.URL

			_, err := NewClient(&tc.cfg).Get(context.Background(), "/api/ping", nil)

			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(sent))
		})
	}
}

func TestNewConfig_CustomerToolsetNeedsNoIdentityOfItsOwn(t *testing.T) {
	t.Setenv("LAGUNA_API_URL", "http://localhost:8080")
	t.Setenv("MCP_TOOLSET", "customer")
	for _, name := range []string{"MCP_AUTH_TOKEN", "LAGUNA_EMAIL", "LAGUNA_USERNAME", "LAGUNA_PASSWORD"} {
		t.Setenv(name, "")
	}

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Equal(t, ToolsetCustomer, cfg.Toolset)
}

func TestNewConfig_CustomerToolsetIgnoresTheOldSettings(t *testing.T) {
	t.Setenv("LAGUNA_API_URL", "http://localhost:8080")
	t.Setenv("MCP_TOOLSET", "customer")
	t.Setenv("MCP_AUTH_TOKEN", "shared-secret")
	t.Setenv("LAGUNA_EMAIL", "agent@example.com")
	t.Setenv("LAGUNA_USERNAME", "svc")
	t.Setenv("LAGUNA_PASSWORD", "secret")

	cfg, err := NewConfig()

	require.NoError(t, err)
	assert.Empty(t, cfg.AuthToken)
	assert.Empty(t, cfg.Email)
	assert.Empty(t, cfg.Username)
	assert.Empty(t, cfg.Password)
}

func TestNewConfig_FullToolsetStillRequiresItsServiceAccount(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "no credential", env: map[string]string{"LAGUNA_PASSWORD": "secret"}, wantErr: "exactly one of"},
		{name: "no password", env: map[string]string{"LAGUNA_EMAIL": "agent@example.com"}, wantErr: "LAGUNA_PASSWORD"},
	}
	t.Setenv("LAGUNA_API_URL", "http://localhost:8080")
	t.Setenv("MCP_TOOLSET", "full")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"LAGUNA_EMAIL", "LAGUNA_USERNAME", "LAGUNA_PASSWORD"} {
				t.Setenv(name, tc.env[name])
			}

			_, err := NewConfig()

			assert.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Setenv("LAGUNA_EMAIL", "agent@example.com")
	t.Setenv("LAGUNA_USERNAME", "")
	t.Setenv("LAGUNA_PASSWORD", "secret")
	t.Setenv("MCP_AUTH_TOKEN", "shared-secret")
	cfg, err := NewConfig()
	require.NoError(t, err)
	assert.Equal(t, "shared-secret", cfg.AuthToken)
	assert.Equal(t, "secret", cfg.Password)
}
