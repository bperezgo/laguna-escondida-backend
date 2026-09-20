## Purpose

Provides an authenticated cloud entry point that verifies an edge node, injects tenant context, and forwards its telemetry to Grafana Cloud — so edge boxes never hold Grafana Cloud credentials.

## ADDED Requirements

### Requirement: Ingest authenticates edge nodes
The cloud telemetry ingest SHALL accept telemetry only from an edge node that presents valid node credentials, reusing the node-authentication model already used by the data-sync path. Telemetry presented without valid credentials SHALL be rejected and SHALL NOT be forwarded.

#### Scenario: Valid node is accepted
- **WHEN** an edge node sends telemetry with a valid node key
- **THEN** the ingest accepts it and forwards it to Grafana Cloud

#### Scenario: Missing or invalid credentials are rejected
- **WHEN** telemetry arrives with no node key or an invalid one
- **THEN** the ingest rejects the request and forwards nothing

### Requirement: Cloud holds telemetry credentials, edge does not
Grafana Cloud credentials SHALL reside only in the cloud telemetry path, and only in the component that performs the egress to Grafana Cloud. Neither edge configuration nor the backend application SHALL contain Grafana Cloud credentials; the ingest SHALL attach the tenant/label context before forwarding.

#### Scenario: Edge configuration has no cloud telemetry credentials
- **WHEN** an edge node's configuration is inspected
- **THEN** it contains no Grafana Cloud credentials, only the address of the cloud ingest and its node key

#### Scenario: The backend forwards without presenting a credential
- **WHEN** the ingest forwards an authenticated node's export upstream
- **THEN** the request carries no Grafana Cloud credential, and the component it forwards to is the one that authenticates to Grafana Cloud

#### Scenario: Tenant context is injected in the cloud
- **WHEN** the ingest forwards an edge node's telemetry to Grafana Cloud
- **THEN** the forwarded telemetry carries the tenant context added by the ingest, not supplied by the edge

### Requirement: Ingest forwards all three signal types
The cloud ingest SHALL forward logs, traces, and metrics received from an authenticated edge node to Grafana Cloud.

#### Scenario: All signals pass through
- **WHEN** an authenticated edge node sends logs, traces, and metrics
- **THEN** all three are forwarded to Grafana Cloud
