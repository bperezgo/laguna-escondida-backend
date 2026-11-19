package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	wompiTimeout = 10 * time.Second
	// Sandbox and production links both open on this checkout.
	wompiCheckoutURL = "https://checkout.wompi.co/l/"
)

// WompiClient is Wompi's payment links API; each call carries the organization's private key.
type WompiClient struct {
	client  *Client
	baseURL string
}

func NewWompiClient(client *Client, baseURL string) ports.PaymentLinkProvider {
	return &WompiClient{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

type wompiCreateLink struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	SingleUse       bool   `json:"single_use"`
	CollectShipping bool   `json:"collect_shipping"`
	Currency        string `json:"currency"`
	AmountInCents   int64  `json:"amount_in_cents"`
	ExpiresAt       string `json:"expires_at"`
	SKU             string `json:"sku"`
}

type wompiLinkResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *WompiClient) CreateLink(ctx context.Context, privateKey string, req dto.PaymentLinkRequest) (string, error) {
	body, err := json.Marshal(wompiCreateLink{
		Name:          req.Name,
		Description:   req.Description,
		SingleUse:     true,
		Currency:      req.Currency,
		AmountInCents: req.AmountInCents,
		ExpiresAt:     req.ExpiresAt.UTC().Format(time.RFC3339),
		SKU:           req.SKU,
	})
	if err != nil {
		return "", fmt.Errorf("encode Wompi payment link: %w", err)
	}
	raw, err := c.call(ctx, http.MethodPost, "/payment_links", privateKey, body)
	if err != nil {
		return "", fmt.Errorf("create Wompi payment link: %w", err)
	}
	var created wompiLinkResponse
	if err := json.Unmarshal(raw, &created); err != nil || created.Data.ID == "" {
		return "", fmt.Errorf("create Wompi payment link: unexpected answer %q", truncate(raw))
	}
	return created.Data.ID, nil
}

func (c *WompiClient) DeactivateLink(ctx context.Context, privateKey, linkID string) error {
	if _, err := c.call(ctx, http.MethodPatch, "/payment_links/"+url.PathEscape(linkID), privateKey, []byte(`{"active":false}`)); err != nil {
		return fmt.Errorf("deactivate Wompi payment link %s: %w", linkID, err)
	}
	return nil
}

func (c *WompiClient) CheckoutURL(linkID string) string {
	return wompiCheckoutURL + linkID
}

func (c *WompiClient) call(ctx context.Context, method, path, privateKey string, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, wompiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+privateKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("read answer: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(raw))
	}
	return raw, nil
}

func truncate(raw []byte) string {
	const limit = 512
	if len(raw) > limit {
		return string(raw[:limit]) + "…"
	}
	return string(raw)
}
