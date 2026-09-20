package handler

import (
	"compress/gzip"
	"errors"
	"io"
	"mime"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainerror "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

// maxTelemetryBodyBytes bounds a single OTLP export. Alloy batches before sending, so a
// well-behaved edge stays far below this; the cap exists so one misconfigured node cannot
// make the cloud read an unbounded body into memory.
const maxTelemetryBodyBytes = 16 << 20

// TelemetryHandler is the OTLP/HTTP entry point for edge nodes. It is deliberately thin:
// decompress, hand the raw OTLP bytes to the ingest service, and translate its errors —
// the relay itself never interprets the payload beyond the tenant stamp.
type TelemetryHandler struct {
	ingestService *service.TelemetryIngestService
}

func NewTelemetryHandler(ingestService *service.TelemetryIngestService) *TelemetryHandler {
	return &TelemetryHandler{ingestService: ingestService}
}

func (h *TelemetryHandler) IngestLogsHandler(c *gin.Context) {
	h.ingest(c, dto.TelemetrySignalLogs)
}

func (h *TelemetryHandler) IngestTracesHandler(c *gin.Context) {
	h.ingest(c, dto.TelemetrySignalTraces)
}

func (h *TelemetryHandler) IngestMetricsHandler(c *gin.Context) {
	h.ingest(c, dto.TelemetrySignalMetrics)
}

func (h *TelemetryHandler) ingest(c *gin.Context, signal dto.TelemetrySignal) {
	body, err := readTelemetryBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contentType, _, err := mime.ParseMediaType(c.ContentType())
	if err != nil {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "invalid content type"})
		return
	}

	ingestErr := h.ingestService.Ingest(c.Request.Context(), dto.TelemetryExport{
		Signal:      signal,
		ContentType: contentType,
		Body:        body,
	})
	if ingestErr != nil {
		respondTelemetryError(c, ingestErr)
		return
	}

	// An empty body is a valid OTLP ExportXServiceResponse (no partial_success), which is
	// what a fully accepted export means.
	c.Data(http.StatusOK, dto.TelemetryContentTypeProtobuf, nil)
}

func readTelemetryBody(c *gin.Context) ([]byte, error) {
	var reader io.Reader = http.MaxBytesReader(c.Writer, c.Request.Body, maxTelemetryBodyBytes)

	if c.GetHeader("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, errors.New("invalid gzip body")
		}
		defer func() { _ = gzipReader.Close() }()
		reader = gzipReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.New("could not read request body")
	}
	return body, nil
}

// respondTelemetryError maps ingest failures onto the status codes an OTLP client acts on:
// 4xx makes Alloy drop the batch as permanently bad, 502 makes it retry and keep buffering.
func respondTelemetryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainerror.ErrTelemetryUnsupportedContentType):
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "telemetry must be sent as " + dto.TelemetryContentTypeProtobuf})
	case errors.Is(err, domainerror.ErrTelemetryEmptyPayload),
		errors.Is(err, domainerror.ErrTelemetryMalformedPayload),
		errors.Is(err, domainerror.ErrTelemetryUnsupportedSignal):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not forward telemetry upstream"})
	}
}
