package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// SyncPushClient POSTs a batch of outbox ops to a peer's /api/sync/push, authenticating
// with the shared node key (X-Node-Key, matched by the cloud's NodeAuthMiddleware).
type SyncPushClient struct {
	client  *Client
	baseURL string
	nodeKey string
}

func NewSyncPushClient(client *Client, baseURL, nodeKey string) ports.SyncPushClient {
	return &SyncPushClient{client: client, baseURL: baseURL, nodeKey: nodeKey}
}

func (c *SyncPushClient) Push(ctx context.Context, req *dto.SyncPushRequest) (res *dto.SyncPushResponse, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal sync push request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/sync/push", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build sync push request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-Key", c.nodeKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send sync push request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sync push response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync push returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var out dto.SyncPushResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("unmarshal sync push response: %w", err)
	}
	return &out, nil
}
