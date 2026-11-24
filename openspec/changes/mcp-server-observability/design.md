## Context

See proposal.md. Today `cmd/mcp-server/main.go` builds a config, an API client and the MCP server, serves `/mcp` over streamable HTTP (or stdio), and never calls `observability.Init`. The API binaries do, from `internal/bootstrap`, with `internal/platform/observability` (OTLP gRPC traces and logs, `propagation.TraceContext{}`, `ParentBased(TraceIDRatioBased)`, resource with `deployment.environment.name`). The MCP server calls the backend through `mcpserver.Client` using a plain `http.Client`, which injects nothing. The sync rig already runs an `alloy` service that cloud sends to.

## Goals / Non-Goals

**Goals:**
- A tool call's trace continues from the agent, through the MCP server, into the backend request.
- Reuse the existing observability package rather than a second pipeline.

**Non-Goals:**
- Metrics for the MCP server, stdio telemetry, changes to production Alloy.
- Recording tool arguments or results.

## Decisions

**D1. Reuse `observability.Init` with a config built from the same variables.** The MCP server has no `Shared` config loader; the task adds a small loader for `OBSERVABILITY_ENABLED`, `OTEL_SERVICE_NAME` (default `laguna-mcp`), `ENVIRONMENT`, `SERVICE_VERSION`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_SAMPLER_ARG`, reusing the config package's parsing if it can be called without the rest of the API config. Node and organization fields stay empty; the resource must not claim an organization the MCP server does not have (it serves many).

**D2. Extract in the HTTP handler with `otelhttp`.** Wrapping the `/mcp` handler with `otelhttp.NewHandler` extracts `traceparent`, follows the sampling flag, and creates the request span. Manual extraction is the fallback if the dependency is unwanted. `/health` is excluded so probes do not create traces.

**D3. Tool-call spans in one place.** The server registers tools through a common path in `mcpserver`; a middleware on the MCP server (the SDK's receiving middleware) starts a span per `tools/call` named for the tool. The span records the tool name and success only, never the input. If the SDK offers no middleware, the task wraps registration instead.

**D4. Inject on the backend client.** `mcpserver.Client`'s `http.Client` gets an `otelhttp` transport (or a small `RoundTripper` that injects), so every call, including the 401-retry, carries the header. The client's own spans for the backend call are accepted, as they show the MCP-to-backend latency.

**D5. Logs carry the trace id through the same slog bridge as the API.** The MCP server's `log.Printf` lines are moved to the structured logger only where they are emitted while serving a request; startup lines can stay.

**D6. Rig wiring.** `mcp` gets `OBSERVABILITY_ENABLED: "true"`, `OTEL_SERVICE_NAME: laguna-mcp`, `OTEL_EXPORTER_OTLP_ENDPOINT: http://alloy:4317` and `depends_on` alloy, mirroring cloud. Alloy's config already accepts any app on the compose network.

## Risks / Trade-offs

- [Customer phone or key leaks into a span through an HTTP attribute such as the full URL or a header capture] → `otelhttp` does not capture headers by default; a test asserts no exported attribute contains a sentinel phone and key, and query strings are not recorded.
- [The server span for `/mcp` is long-lived on streamable connections] → only one request per tool call is expected from the agent (standalone SSE is disabled there); the task checks span durations in the rig.
- [Service name `laguna-mcp` is new in dashboards] → documented in the rig header comment.
