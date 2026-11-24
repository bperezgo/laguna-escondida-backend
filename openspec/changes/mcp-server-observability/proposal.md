## Why

The validation pass in marketing-app (`docs/observability-evidence.md`) showed that the MCP server is a blind spot: a tool call from the agent produces no span, no shipped log and no metric from `laguna-sync-mcp`, and the backend request it triggers is a separate trace. The agent side is being changed to send `traceparent` (marketing-app change `agent-trace-context-and-resources`); without instrumentation here the header stops at the MCP server and the backend call is still not part of the agent's trace.

## What Changes

- `cmd/mcp-server` initialises the same OpenTelemetry pipeline as the API binaries (traces and logs over OTLP gRPC, W3C propagator, parent-based ratio sampler, `ENVIRONMENT`/`SERVICE_VERSION` resource), under its own service name, disabled unless `OBSERVABILITY_ENABLED=true`.
- The HTTP handler continues the trace of an incoming `traceparent` and records one server span per MCP request, with a child span for each tool call carrying the tool name and outcome.
- The MCP server's call to the backend API sends the active trace context, so the backend's existing server span joins the agent's trace. The backend needs no change to continue it.
- The MCP server's logs are shipped with the trace id, as the API's are.
- The sync rig (`docker-compose.sync.yml`) points the `mcp` service at the existing `alloy` service, so its telemetry reaches the shared observability stack.
- Tool arguments and results are NOT recorded on spans or logs.

Out of scope: the agent side (marketing-app), metrics for the MCP server (its tool latency is visible from the spans), the stdio transport's telemetry (it runs as a Claude Desktop subprocess with no collector), production Alloy wiring.

## Capabilities

### New Capabilities
- `mcp-server-telemetry`: the MCP server's own traces and logs, trace continuation from the caller, and propagation to the backend API.

### Modified Capabilities
None.

## Impact

- `backend/cmd/mcp-server/main.go`, `backend/internal/platform/mcpserver` (HTTP client and tool wrapper), a small reuse of `internal/platform/observability`.
- `backend/docker-compose.sync.yml`: env for the `mcp` service, `depends_on` alloy.
- `backend/alloy/config.alloy` needs no change.
- No API, schema or tool-contract change. New dependency use: `otelhttp` (or manual extract/inject) if not already in `go.mod`.
