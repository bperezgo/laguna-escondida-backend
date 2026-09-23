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

// SyncStockPullClient GETs the cloud's changed stock from /api/sync/pull/stock,
// authenticating with the shared node key (X-Node-Key). The cursor is sent as an
// RFC3339Nano `since` query param, exactly like the reference pull — but on its own
// bookmark, so the daily refresh and the minute-by-minute reference pull stay independent.
type SyncStockPullClient struct {
	client  *Client
	baseURL string
	nodeKey string
}

func NewSyncStockPullClient(client *Client, baseURL, nodeKey string) ports.SyncStockPullClient {
	return &SyncStockPullClient{client: client, baseURL: baseURL, nodeKey: nodeKey}
}

func (c *SyncStockPullClient) PullStock(ctx context.Context, since time.Time) (res *dto.SyncStockPullResponse, err error) {
	endpoint := fmt.Sprintf("%s/api/sync/pull/stock?since=%s", c.baseURL, url.QueryEscape(since.UTC().Format(time.RFC3339Nano)))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build stock pull request: %w", err)
	}
	httpReq.Header.Set("X-Node-Key", c.nodeKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send stock pull request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stock pull response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stock pull returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var out dto.SyncStockPullResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("unmarshal stock pull response: %w", err)
	}
	return &out, nil
}
