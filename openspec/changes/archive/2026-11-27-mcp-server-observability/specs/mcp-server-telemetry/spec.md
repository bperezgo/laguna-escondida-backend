## ADDED Requirements

### Requirement: The MCP server reports its own traces and logs

The MCP server SHALL export traces and logs, identified by its own service name, environment and version, whenever observability is enabled, and SHALL do nothing and start normally when it is not. It SHALL record a span for each MCP request it serves and a child span for each tool call, naming the tool and whether it succeeded. Its log lines emitted while serving a request SHALL carry that request's trace identifier.

#### Scenario: A tool is called

- **WHEN** a client calls a tool on the MCP server with observability enabled
- **THEN** a trace is exported containing a span for the request and a child span named for the tool
- **AND** a failed tool call marks its span as failed

#### Scenario: Observability is disabled

- **WHEN** the MCP server starts with observability disabled
- **THEN** it serves tool calls exactly as before and exports nothing

#### Scenario: The collector is unreachable

- **WHEN** observability is enabled and the collector cannot be reached
- **THEN** the MCP server starts and serves tool calls, and export failures do not fail or delay them

#### Scenario: Reading a request's logs

- **WHEN** a log line written while serving a tool call is found
- **THEN** it carries the trace identifier of that call and leads to its trace

### Requirement: The MCP server continues its caller's trace

When a request arrives with a W3C `traceparent`, the MCP server SHALL record its spans as part of that trace, under the caller's span, and SHALL follow the caller's sampling decision. A request without the header SHALL start a new trace, sampled by the configured ratio.

#### Scenario: A sampled caller

- **WHEN** a request arrives with a `traceparent` marked sampled
- **THEN** the MCP server's spans carry that trace identifier and parent under the caller's span

#### Scenario: An unsampled caller

- **WHEN** a request arrives with a `traceparent` marked not sampled
- **THEN** the MCP server records no spans for it

#### Scenario: No header

- **WHEN** a request arrives with no `traceparent`
- **THEN** the MCP server starts its own trace for it

### Requirement: The MCP server forwards the trace to the backend

When a tool call makes a request to the backend API, the MCP server SHALL send the active trace context on it, so the backend's span for that request is part of the same trace. Forwarding the header SHALL NOT change what the backend authenticates, authorizes or returns.

#### Scenario: A tool call reads from the backend

- **WHEN** a tool call fetches data from the backend while traced
- **THEN** the backend request carries a `traceparent` with the call's trace identifier
- **AND** the backend's server span for it appears in that trace under the tool call

#### Scenario: Retried requests

- **WHEN** the MCP server retries a backend request after refreshing its token
- **THEN** both attempts carry the trace context

### Requirement: Telemetry does not record tool content

Spans and logs of the MCP server SHALL NOT contain tool arguments, tool results, customer identifiers such as the customer phone header, or credentials. They MAY contain the tool name, the HTTP method and route of the backend request, status codes and durations.

#### Scenario: A customer-facing tool is called

- **WHEN** a tool is called with a customer phone and a delivery address
- **THEN** no exported span attribute or log field contains either value, nor the API key

### Requirement: The local sync rig ships the MCP server's telemetry

The local rig SHALL run the MCP server with observability enabled and pointed at the rig's collector, so that a tool call made against it appears in the shared observability stack under the MCP server's service name.

#### Scenario: Calling a tool against the rig

- **WHEN** a tool call is made to the MCP server of the running rig
- **THEN** its trace and logs appear in the observability stack's Grafana under the MCP server's service name
