package httpclient

import (
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	*http.Client
}

type ClientOption func(*Client)

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

func NewClient(logger *slog.Logger, opts ...ClientOption) *Client {
	client := &Client{
		Client: &http.Client{
			Transport: NewLoggingTransport(logger),
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}
