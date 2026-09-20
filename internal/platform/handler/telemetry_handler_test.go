package handler

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainerror "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const telemetryNodeKey = "node-secret"

// telemetryPayload is a minimal but structurally valid OTLP protobuf body: field 1
// (resource_logs/resource_spans/resource_metrics) holding one empty message.
var telemetryPayload = []byte{0x0a, 0x00}

func telemetryRouter(cfg *config.Config, forwarder *mocks.MockTelemetryForwarder) *gin.Engine {
	ingestService := service.NewTelemetryIngestService(forwarder, "tenant-abc", "org-123")
	h := NewTelemetryHandler(ingestService)

	router := gin.New()
	router.POST("/api/telemetry/v1/logs", NodeAuthMiddleware(cfg), h.IngestLogsHandler)
	router.POST("/api/telemetry/v1/traces", NodeAuthMiddleware(cfg), h.IngestTracesHandler)
	router.POST("/api/telemetry/v1/metrics", NodeAuthMiddleware(cfg), h.IngestMetricsHandler)
	return router
}

func telemetryRequest(path, nodeKey string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", dto.TelemetryContentTypeProtobuf)
	if nodeKey != "" {
		req.Header.Set("X-Node-Key", nodeKey)
	}
	return req
}

func TestTelemetryHandler_ValidNodeKeyIsAccepted(t *testing.T) {
	for path, signal := range map[string]dto.TelemetrySignal{
		"/api/telemetry/v1/logs":    dto.TelemetrySignalLogs,
		"/api/telemetry/v1/traces":  dto.TelemetrySignalTraces,
		"/api/telemetry/v1/metrics": dto.TelemetrySignalMetrics,
	} {
		t.Run(string(signal), func(t *testing.T) {
			forwarder := mocks.NewMockTelemetryForwarder(t)
			forwarder.On("Forward", mock.Anything, mock.MatchedBy(func(e dto.TelemetryExport) bool {
				return e.Signal == signal && bytes.Equal(e.Body, telemetryPayload)
			})).Return(nil).Once()

			w := httptest.NewRecorder()
			telemetryRouter(&config.Config{NodeSyncKey: telemetryNodeKey}, forwarder).
				ServeHTTP(w, telemetryRequest(path, telemetryNodeKey, telemetryPayload))

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// A rejected node must not reach the forwarder at all: auth that merely changed the
// response code while still relaying would leak an unauthenticated node's telemetry into
// the tenant's Grafana stack.
func TestTelemetryHandler_InvalidOrMissingNodeKeyForwardsNothing(t *testing.T) {
	tests := []struct {
		name    string
		nodeKey string
	}{
		{name: "missing key", nodeKey: ""},
		{name: "wrong key", nodeKey: "not-the-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forwarder := mocks.NewMockTelemetryForwarder(t)

			w := httptest.NewRecorder()
			telemetryRouter(&config.Config{NodeSyncKey: telemetryNodeKey}, forwarder).
				ServeHTTP(w, telemetryRequest("/api/telemetry/v1/logs", tt.nodeKey, telemetryPayload))

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			forwarder.AssertNotCalled(t, "Forward", mock.Anything, mock.Anything)
		})
	}
}

// Alloy compresses exports by default, so the ingest has to unwrap gzip before the OTLP
// bytes reach the relay.
func TestTelemetryHandler_GzippedBodyIsDecompressed(t *testing.T) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(telemetryPayload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	forwarder := mocks.NewMockTelemetryForwarder(t)
	forwarder.On("Forward", mock.Anything, mock.MatchedBy(func(e dto.TelemetryExport) bool {
		return bytes.Equal(e.Body, telemetryPayload)
	})).Return(nil).Once()

	req := telemetryRequest("/api/telemetry/v1/traces", telemetryNodeKey, buf.Bytes())
	req.Header.Set("Content-Encoding", "gzip")

	w := httptest.NewRecorder()
	telemetryRouter(&config.Config{NodeSyncKey: telemetryNodeKey}, forwarder).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTelemetryHandler_NonProtobufContentTypeIsRejected(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)

	req := telemetryRequest("/api/telemetry/v1/logs", telemetryNodeKey, telemetryPayload)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	telemetryRouter(&config.Config{NodeSyncKey: telemetryNodeKey}, forwarder).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	forwarder.AssertNotCalled(t, "Forward", mock.Anything, mock.Anything)
}

func TestTelemetryHandler_MalformedPayloadIsClientError(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)
	forwarder.On("Forward", mock.Anything, mock.Anything).
		Return(domainerror.ErrTelemetryMalformedPayload).Once()

	w := httptest.NewRecorder()
	telemetryRouter(&config.Config{NodeSyncKey: telemetryNodeKey}, forwarder).
		ServeHTTP(w, telemetryRequest("/api/telemetry/v1/metrics", telemetryNodeKey, []byte{0xff}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// An upstream failure has to come back retryable, so Alloy keeps the batch buffered on the
// edge instead of dropping it.
func TestTelemetryHandler_UpstreamFailureIsRetryable(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)
	forwarder.On("Forward", mock.Anything, mock.Anything).Return(errors.New("gateway down")).Once()

	w := httptest.NewRecorder()
	telemetryRouter(&config.Config{NodeSyncKey: telemetryNodeKey}, forwarder).
		ServeHTTP(w, telemetryRequest("/api/telemetry/v1/logs", telemetryNodeKey, telemetryPayload))

	assert.Equal(t, http.StatusBadGateway, w.Code)
}
