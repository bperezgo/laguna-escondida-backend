package httpclient

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type LoggingTransport struct {
	transport http.RoundTripper
	logger    *slog.Logger
}

func NewLoggingTransport(logger *slog.Logger) *LoggingTransport {
	return &LoggingTransport{
		transport: http.DefaultTransport,
		logger:    logger,
	}
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	var requestBody []byte
	if req.Body != nil {
		requestBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	t.logger.InfoContext(req.Context(), "HTTP Client Request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("body", string(requestBody)),
	)

	resp, err := t.transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		t.logger.ErrorContext(req.Context(), "HTTP Client Request Failed",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.Any("error", err),
		)
		return nil, err
	}

	var responseBody []byte
	if resp.Body != nil {
		responseBody, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	}

	t.logger.InfoContext(req.Context(), "HTTP Client Response",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Int("status_code", resp.StatusCode),
		slog.Duration("duration", duration),
		slog.String("body", string(responseBody)),
	)

	return resp, nil
}
