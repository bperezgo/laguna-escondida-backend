## Purpose

Gets logs, traces, and metrics off an offline-prone edge box and to the cloud without ever blocking the POS or filling the edge's disk, by buffering locally and forwarding when connectivity allows.

## ADDED Requirements

### Requirement: Application exports telemetry to a local endpoint
The POS application SHALL export logs and traces to a telemetry endpoint on the local host and expose its metrics for local collection, rather than sending directly to the cloud. The application SHALL NOT require or hold cloud telemetry credentials.

#### Scenario: Cloud is unreachable
- **WHEN** the edge has no internet connectivity and the application produces logs, traces, and metrics
- **THEN** the application continues serving POS requests normally with no added latency or errors from telemetry export

#### Scenario: Local collector is down
- **WHEN** the local telemetry collector is not running and the application emits telemetry
- **THEN** POS request handling still succeeds and no request fails because telemetry could not be delivered

### Requirement: Telemetry is buffered durably within a bounded cap
The edge SHALL persist telemetry that has not yet been forwarded to local durable storage so it survives collector restarts, and SHALL bound that storage by a configured maximum size, so its disk use does not grow with the length of an outage. When the buffer is full, the edge SHALL discard newly produced telemetry rather than exceed the cap, preserving what is already buffered.

#### Scenario: Reconnect after an offline period
- **WHEN** the edge was offline while producing telemetry and connectivity is restored
- **THEN** telemetry buffered during the outage is forwarded to the cloud

#### Scenario: Buffer reaches its cap during a long outage
- **WHEN** the edge stays offline long enough to fill the telemetry buffer to its configured cap
- **THEN** local disk used by the buffer stays bounded by that cap instead of growing with the outage, and telemetry produced after the cap is reached is discarded rather than buffered

#### Scenario: Buffered telemetry survives a collector restart
- **WHEN** the local telemetry collector is restarted while it holds telemetry that has not been forwarded
- **THEN** that telemetry is still forwarded once connectivity is restored

### Requirement: All three signal types are forwarded
The edge SHALL forward logs, traces, and metrics to the cloud telemetry ingest.

#### Scenario: Each signal reaches the cloud
- **WHEN** the edge is online and the application has produced logs, traces, and metrics
- **THEN** all three signal types are present in the cloud for that edge

### Requirement: Edge-side volume controls
The edge SHALL apply configured controls that limit telemetry volume — at minimum a trace sampling ratio and a minimum log severity that is forwarded — so that steady-state telemetry does not saturate edge storage or the connection.

#### Scenario: Sub-INFO logs are not forwarded
- **WHEN** the minimum forwarded log severity is configured to INFO and the application emits DEBUG logs
- **THEN** the DEBUG logs are not forwarded to the cloud
