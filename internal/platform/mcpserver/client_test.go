package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
)

func testClient(baseURL string) *Client {
	return NewClient(&Config{
		APIBaseURL:  baseURL,
		Username:    "svc",
		Password:    "secret",
		AuthToken:   "unused-in-client",
		AdminAPIKey: "admin-key",
	})
}

func writeSignIn(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(dto.SignInResponse{Token: "tok-1"})
}

func TestClient_SignsInOnceAndAttachesBearer(t *testing.T) {
	var signInCount int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&signInCount, 1)
		writeSignIn(w)
	})
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"pong":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv.URL)
	for i := 0; i < 2; i++ {
		body, err := c.Get(context.Background(), "/api/ping", nil)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !strings.Contains(string(body), "pong") {
			t.Fatalf("unexpected body: %s", body)
		}
	}
	if got := atomic.LoadInt32(&signInCount); got != 1 {
		t.Fatalf("expected the token to be cached (1 signin), got %d", got)
	}
}

func TestClient_ReauthOnUnauthorized(t *testing.T) {
	var signInCount, pings int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&signInCount, 1)
		writeSignIn(w)
	})
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, _ *http.Request) {
		// Reject the first call to force a re-authentication, then accept.
		if atomic.AddInt32(&pings, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"pong":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv.URL)
	body, err := c.Get(context.Background(), "/api/ping", nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !strings.Contains(string(body), "pong") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := atomic.LoadInt32(&signInCount); got != 2 {
		t.Fatalf("expected 2 signInCount (initial + reauth), got %d", got)
	}
}

func TestClient_PropagatesBackendError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/api/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"kaboom"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv.URL)
	_, err := c.Get(context.Background(), "/api/boom", nil)
	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("error should include status and body, got: %v", err)
	}
}

func TestClient_PostAdminUsesAPIKeyNotBearer(t *testing.T) {
	var signInCount int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&signInCount, 1)
		writeSignIn(w)
	})
	mux.HandleFunc("/api/admin-action", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "admin-key" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv.URL)
	body, err := c.PostAdmin(context.Background(), "/api/admin-action", nil)
	if err != nil {
		t.Fatalf("PostAdmin failed: %v", err)
	}
	if !strings.Contains(string(body), "ok") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := atomic.LoadInt32(&signInCount); got != 0 {
		t.Fatalf("admin calls must not sign in for a JWT, got %d signInCount", got)
	}
}

// TestNewMCPServer_RegistersAllToolsWithoutPanic guards against a future DTO
// change breaking JSON-schema inference: NewMCPServer registers every tool at
// build time, so a bad input type would panic here.
func TestNewMCPServer_RegistersAllToolsWithoutPanic(t *testing.T) {
	if s := NewMCPServer(testClient("http://backend.invalid")); s == nil {
		t.Fatal("expected a non-nil MCP server")
	}
}
