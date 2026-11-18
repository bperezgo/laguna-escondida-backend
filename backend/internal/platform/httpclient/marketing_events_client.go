package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/ports"
)

// marketingEventsTimeout bounds one attempt; an attempt that times out is retried later.
const marketingEventsTimeout = 10 * time.Second

// MarketingEventsClient POSTs signed event bodies to marketing-app.
type MarketingEventsClient struct {
	client  *Client
	baseURL string
}

func NewMarketingEventsClient(client *Client, baseURL string) ports.MarketingEventTransport {
	return &MarketingEventsClient{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (c *MarketingEventsClient) Send(ctx context.Context, path string, body []byte, signature string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, marketingEventsTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build marketing event request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-256", signature)
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send marketing event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, nil
}
