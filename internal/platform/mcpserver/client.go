package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

// Client is an authenticated HTTP client for the backend API. It signs in once
// with the configured service account, caches the JWT, and re-authenticates
// transparently on a 401.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	username    string
	password    string
	adminAPIKey string

	mu    sync.Mutex
	token string
}

// NewClient builds a backend API client from the MCP server config.
func NewClient(cfg *Config) *Client {
	return &Client{
		baseURL:     strings.TrimRight(cfg.APIBaseURL, "/"),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		username:    cfg.Username,
		password:    cfg.Password,
		adminAPIKey: cfg.AdminAPIKey,
	}
}

type requestOptions struct {
	// useAdminKey sends the X-API-Key admin header instead of a Bearer JWT.
	useAdminKey bool
}

// Get issues a GET request and returns the raw response body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, query, nil, requestOptions{})
}

// Post issues a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPost, path, nil, body, requestOptions{})
}

// Put issues a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPut, path, nil, body, requestOptions{})
}

// Patch issues a PATCH request with an optional JSON body.
func (c *Client) Patch(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, path, nil, body, requestOptions{})
}

// Delete issues a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, path, nil, nil, requestOptions{})
}

// PostAdmin issues a POST authenticated with the admin API key header.
func (c *Client) PostAdmin(ctx context.Context, path string, body any) ([]byte, error) {
	if c.adminAPIKey == "" {
		return nil, fmt.Errorf("this tool requires LAGUNA_ADMIN_API_KEY to be set")
	}
	return c.do(ctx, http.MethodPost, path, nil, body, requestOptions{useAdminKey: true})
}

// UploadFile posts a multipart/form-data request with a single file field read
// from a path on the MCP server host.
func (c *Client) UploadFile(ctx context.Context, path string, query url.Values, fieldName, filePath string) ([]byte, error) {
	f, err := os.Open(filePath) //nolint:gosec // file_path is an intentional operator-provided upload source
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, copyErr := io.Copy(fw, f); copyErr != nil {
		return nil, fmt.Errorf("copy file: %w", copyErr)
	}
	if closeErr := mw.Close(); closeErr != nil {
		return nil, fmt.Errorf("close multipart writer: %w", closeErr)
	}

	bodyBytes := buf.Bytes()
	contentType := mw.FormDataContentType()

	respBody, status, err := c.doOnce(ctx, http.MethodPost, path, query, bodyBytes, contentType, requestOptions{})
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		c.clearToken()
		respBody, status, err = c.doOnce(ctx, http.MethodPost, path, query, bodyBytes, contentType, requestOptions{})
		if err != nil {
			return nil, err
		}
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("backend POST %s -> %d: %s", path, status, truncate(string(respBody), 4000))
	}
	return respBody, nil
}

// do sends a request, transparently retrying once after re-authentication on a
// 401, and returns the body for any 2xx response.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, opts requestOptions) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyBytes = b
	}

	respBody, status, err := c.doOnce(ctx, method, path, query, bodyBytes, "application/json", opts)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized && !opts.useAdminKey {
		c.clearToken()
		respBody, status, err = c.doOnce(ctx, method, path, query, bodyBytes, "application/json", opts)
		if err != nil {
			return nil, err
		}
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("backend %s %s -> %d: %s", method, path, status, truncate(string(respBody), 4000))
	}
	return respBody, nil
}

func (c *Client) doOnce(ctx context.Context, method, path string, query url.Values, bodyBytes []byte, contentType string, opts requestOptions) ([]byte, int, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, 0, err
	}
	if bodyBytes != nil {
		req.Header.Set("Content-Type", contentType)
	}

	if opts.useAdminKey {
		req.Header.Set("X-API-Key", c.adminAPIKey)
	} else {
		token, tokenErr := c.ensureToken(ctx)
		if tokenErr != nil {
			return nil, 0, tokenErr
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request %s %s failed: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response body: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// ensureToken returns a cached JWT, signing in on first use.
func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token != "" {
		return token, nil
	}

	token, err := c.signIn(ctx)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
	return token, nil
}

func (c *Client) clearToken() {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
}

func (c *Client) signIn(ctx context.Context) (string, error) {
	payload, err := json.Marshal(dto.SignInRequest{Username: c.username, Password: c.password})
	if err != nil {
		return "", fmt.Errorf("marshal signin request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/auth/signin", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("signin request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read signin response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("signin failed: status %d: %s", resp.StatusCode, truncate(string(respBody), 1000))
	}

	var signInResp dto.SignInResponse
	if err := json.Unmarshal(respBody, &signInResp); err != nil {
		return "", fmt.Errorf("decode signin response: %w", err)
	}
	if signInResp.Token == "" {
		return "", fmt.Errorf("signin returned an empty token")
	}
	return signInResp.Token, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
