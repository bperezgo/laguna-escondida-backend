## Purpose

Attaches a stable per-instance identity to every log, trace, and metric so operators can filter and attribute telemetry to a single edge box in Grafana Cloud.

## ADDED Requirements

### Requirement: Every signal carries per-instance identity
Every log, trace, and metric produced by the edge SHALL carry a stable per-instance node identifier, the organization it belongs to, and its deployment mode (`edge`), such that telemetry can be filtered to one specific edge box.

#### Scenario: Filter telemetry to one box
- **WHEN** an operator queries telemetry in the cloud and filters by a node identifier
- **THEN** only telemetry produced by that edge box is returned

#### Scenario: Two boxes are distinguishable
- **WHEN** two edge boxes with distinct node identifiers are both forwarding telemetry
- **THEN** their logs, traces, and metrics can be told apart by node identifier

### Requirement: Identity is stable across restarts
The per-instance node identifier SHALL remain the same across restarts of the application and the local telemetry collector, so a box's history is continuous.

#### Scenario: Restart keeps the same identity
- **WHEN** the edge application is restarted
- **THEN** telemetry produced after the restart carries the same node identifier as before

### Requirement: Identity is applied uniformly across signal types
The same identity values SHALL be applied to logs, traces, and metrics for a given edge box.

#### Scenario: Correlated signals share identity
- **WHEN** a single request produces a log, a trace, and updates a metric
- **THEN** all three carry the same node identifier and organization
