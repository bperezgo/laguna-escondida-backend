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
