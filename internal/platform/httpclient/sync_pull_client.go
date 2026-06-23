package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// SyncPullClient GETs reference changes from a peer's /api/sync/pull, authenticating with
// the shared node key (X-Node-Key). The cursor is sent as an RFC3339Nano `since` query param.
type SyncPullClient struct {
	client  *Client
	baseURL string
	nodeKey string
}

func NewSyncPullClient(client *Client, baseURL, nodeKey string) ports.SyncPullClient {
	return &SyncPullClient{client: client, baseURL: baseURL, nodeKey: nodeKey}
}

func (c *SyncPullClient) Pull(ctx context.Context, since time.Time) (res *dto.SyncPullResponse, err error) {
	endpoint := fmt.Sprintf("%s/api/sync/pull?since=%s", c.baseURL, url.QueryEscape(since.UTC().Format(time.RFC3339Nano)))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build sync pull request: %w", err)
	}
	httpReq.Header.Set("X-Node-Key", c.nodeKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send sync pull request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sync pull response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync pull returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var out dto.SyncPullResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("unmarshal sync pull response: %w", err)
	}
	return &out, nil
}
