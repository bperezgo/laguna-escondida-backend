## 1. Pipeline in the MCP server

- [x] 1.1 Load the observability settings in `cmd/mcp-server` (same variables as the API, service name default `laguna-mcp`, no node or organization) and call `observability.Init` on startup with a shutdown on exit; verify it starts normally with observability disabled and with an unreachable collector
- [x] 1.2 Wrap `/mcp` with a handler that continues the incoming `traceparent` and follows its sampling flag, excluding `/health`; verify with a recording tracer for a sampled, an unsampled and a header-less request
- [x] 1.3 Add a span per tool call named for the tool, marked failed on error, with no input or output recorded; verify with a recording tracer and a sentinel-value test that no attribute holds the arguments, results, customer phone or key

## 2. Forward to the backend

- [x] 2.1 Make `mcpserver.Client` inject the active trace context on every backend request including the 401 retry; verify with a test server that sees the same trace id on both attempts and that auth and body are unchanged
- [x] 2.2 Move the request-time log lines to the structured logger so they carry the trace id; verify a log line from a tool call has the call's trace id

## 3. Sync rig

- [x] 3.1 In `docker-compose.sync.yml` give `mcp` `OBSERVABILITY_ENABLED`, `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT` pointing at `alloy` and `depends_on` it; verify `docker compose up` brings the rig up and a tool call shows the MCP server's trace and logs in Grafana under its service name (observability-stack running)
- [ ] 3.2 With marketing-app's `agent-trace-context-and-resources` applied, send one message through the agent that calls an MCP tool; verify one trace contains the agent spans, the MCP server's spans and the backend's server span, and the logs of the three share its trace id
