package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncPullClient_Pull_SendsSinceAndKey(t *testing.T) {
	var gotKey, gotPath, gotMethod, gotSince string

	cursor := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Node-Key")
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotSince = r.URL.Query().Get("since")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dto.SyncPullResponse{
			Products: []dto.ProductSyncPayload{{ID: "p1", Name: "Cafe"}},
			Cursor:   cursor,
		})
	}))
	defer server.Close()

	client := NewSyncPullClient(&Client{Client: server.Client()}, server.URL, "secret-key")

	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	resp, err := client.Pull(context.Background(), since)
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/sync/pull", gotPath)
	assert.Equal(t, "secret-key", gotKey)
	assert.Equal(t, since.Format(time.RFC3339Nano), gotSince, "cursor sent as RFC3339Nano since param")

	require.Len(t, resp.Products, 1)
	assert.Equal(t, "p1", resp.Products[0].ID)
	assert.True(t, cursor.Equal(resp.Cursor))
}

func TestSyncPullClient_Pull_Non200_IsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewSyncPullClient(&Client{Client: server.Client()}, server.URL, "")

	resp, err := client.Pull(context.Background(), time.Time{})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "401")
}
