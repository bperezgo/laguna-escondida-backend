// Package observability wires the backend's OpenTelemetry pipeline: a tracer provider and
// a logger provider that export over OTLP/gRPC to the local Alloy sidecar, which forwards
// to Grafana Cloud. The app never holds Grafana Cloud credentials — it only talks to the
// sidecar on localhost; Alloy owns auth and forwarding.
//
// Metrics are deliberately NOT here: they use the Prometheus registry scraped by Alloy
// (see handler.MetricsMiddleware), which needs no OTLP path in Phase 1.
package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitConfig is the subset of config.Config the observability bootstrap needs. Passing a
// small value struct (instead of the whole *config.Config) keeps this platform package
// from importing config and avoids leaking config-wide concerns in here.
type InitConfig struct {
	Enabled          bool
	ServiceName      string
	ServiceVersion   string
	Environment      string
	OTLPEndpoint     string
	TraceSampleRatio float64
}

// Providers holds the initialized OTel providers plus a Shutdown that flushes them so
// main stays clean and callers don't branch on whether observability is enabled.
type Providers struct {
	Shutdown func(ctx context.Context) error
}

func noopShutdown(context.Context) error { return nil }

// Init sets up the resource, tracer provider, and logger provider wired to the OTLP gRPC
// exporters pointing at the local Alloy sidecar, and installs them as the global providers
// (so otelgin, otelzap, and otelslog pick them up). When observability is disabled or the
// endpoint is empty it returns a no-op Shutdown, leaving the global providers as no-ops.
func Init(ctx context.Context, cfg InitConfig) (*Providers, error) {
	if !cfg.Enabled || cfg.OTLPEndpoint == "" {
		return &Providers{Shutdown: noopShutdown}, nil
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironmentName(cfg.Environment),
		// Phase 2 (edge): tenant.id is injected by the backend proxy, not here.
	)

	// Traces. WithEndpointURL (not WithEndpoint) parses the scheme and derives TLS from it
	// (http => plaintext), so the localhost sidecar needs no separate WithInsecure().
	traceExp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpointURL(otlpEndpointURL(cfg.OTLPEndpoint)),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Logs. Same endpoint handling as traces (see above).
	logExp, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpointURL(otlpEndpointURL(cfg.OTLPEndpoint)),
	)
	if err != nil {
		// Roll back the tracer provider we already registered before failing.
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("otlp log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	logglobal.SetLoggerProvider(lp)

	return &Providers{
		Shutdown: func(ctx context.Context) error {
			return errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx))
		},
	}, nil
}

// otlpEndpointURL returns endpoint in the URL form otlp*grpc.WithEndpointURL expects. The
// OTel-standard OTEL_EXPORTER_OTLP_ENDPOINT is a URL with a scheme (aws-infra sets
// "http://127.0.0.1:4317"); a bare "host:port" is also accepted here for local/dev and is
// treated as plaintext, since the Alloy sidecar speaks gRPC without TLS on localhost.
//
// This exists to avoid the WithEndpoint trap: WithEndpoint wants a bare "host:port", so
// handing it a full URL makes gRPC treat the whole string as the host and append the default
// :443, failing every export with "too many colons in address". WithEndpointURL instead
// parses the scheme (http => insecure, https => TLS), so no separate WithInsecure() is needed.
func otlpEndpointURL(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	return "http://" + endpoint
}
