package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncPushClient_Push_SendsKeyAndParsesAck(t *testing.T) {
	var gotKey, gotPath, gotMethod string
	var gotReq dto.SyncPushRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Node-Key")
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotReq)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dto.SyncPushResponse{
			AckedOpIDs: []string{"op-1"},
			AckedSeqs:  []int64{1},
		})
	}))
	defer server.Close()

	client := NewSyncPushClient(&Client{Client: server.Client()}, server.URL, "secret-key")

	resp, err := client.Push(context.Background(), &dto.SyncPushRequest{
		NodeID: "node-a",
		Ops:    []dto.SyncOutboxEntry{{OpID: "op-1", Seq: 1}},
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/sync/push", gotPath)
	assert.Equal(t, "secret-key", gotKey, "client authenticates with the node key")
	assert.Equal(t, "node-a", gotReq.NodeID, "request body carries the sender node id")
	require.Len(t, gotReq.Ops, 1)
	assert.Equal(t, "op-1", gotReq.Ops[0].OpID)

	assert.Equal(t, []string{"op-1"}, resp.AckedOpIDs)
	assert.Equal(t, []int64{1}, resp.AckedSeqs)
}

func TestSyncPushClient_Push_Non200_IsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewSyncPushClient(&Client{Client: server.Client()}, server.URL, "")

	resp, err := client.Push(context.Background(), &dto.SyncPushRequest{NodeID: "node-a"})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "401")
}
