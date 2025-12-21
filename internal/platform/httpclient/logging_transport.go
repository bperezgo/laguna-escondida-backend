package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type LoggingTransport struct {
	transport http.RoundTripper
	logger    *zap.Logger
}

func NewLoggingTransport(logger *zap.Logger) *LoggingTransport {
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

	t.logger.Info("HTTP Client Request",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.String("body", string(requestBody)),
	)

	resp, err := t.transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		t.logger.Error("HTTP Client Request Failed",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return nil, err
	}

	var responseBody []byte
	if resp.Body != nil {
		responseBody, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	}

	t.logger.Info("HTTP Client Response",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration),
		zap.String("body", string(responseBody)),
	)

	return resp, nil
}
