package error

import "errors"

var (
	ErrTelemetryUnsupportedSignal      = errors.New("unsupported telemetry signal")
	ErrTelemetryUnsupportedContentType = errors.New("unsupported telemetry content type")
	ErrTelemetryEmptyPayload           = errors.New("empty telemetry payload")
	ErrTelemetryMalformedPayload       = errors.New("malformed telemetry payload")
	ErrTelemetryForwardFailed          = errors.New("failed to forward telemetry")
)
