## Why

The edge (a Windows box running on-site) does all the real POS work, but its telemetry never leaves the machine: the in-app OpenTelemetry pipeline (`internal/platform/observability`) is already built and only exports to a **localhost Alloy sidecar that does not exist on the edge**. So today, when something breaks at a restaurant, there is no way to see its logs, traces, or metrics from the cloud. We want per-instance visibility in Grafana Cloud (stack `wittycrane116`: Loki, Tempo, Mimir) without saturating the edge's limited disk and while tolerating an edge that is routinely offline.

Linear ticket: **BSP-19**.

## What Changes

- **Ship the edge sidecar.** Add a Grafana Alloy sidecar to the Windows edge that receives OTLP (logs + traces) on `localhost` and scrapes the app's Prometheus `/metrics`, then forwards all three signals to the cloud. The Go app already emits to `localhost`, so this is deployment/config, not app code.
- **Survive being offline without filling the disk.** The sidecar buffers to a disk WAL with a **hard size cap**; on reconnect it forwards the backlog. Once the cap is reached the sidecar discards newly produced telemetry rather than growing (see design D6) — an outage keeps its earliest telemetry, which is where the cause usually is. Because the app→sidecar hop is `localhost` (always up), the POS is never blocked or slowed when the box is offline. This is the single mechanism that satisfies both "works offline" and "don't saturate the edge."
- **Attach per-instance identity to every signal.** Thread the existing `NODE_ID` (plus organization and `mode=edge`) into the OTel resource so every log/trace/metric is filterable per box in Grafana. Today the resource carries only service name/version/environment; `NodeID` exists in config but never reaches `observability.Init` (its `InitConfig` has no such field).
- **Forward through the cloud backend, not directly (Option B).** The edge sidecar sends to a **cloud-side telemetry ingest** that authenticates the node with its existing `NODE_SYNC_KEY`, injects the tenant/label context, and forwards to Grafana Cloud. Edge boxes therefore hold **no Grafana Cloud credentials** — matching the intent already noted in code (*"tenant.id is injected by the backend proxy"*) and reusing the node-auth model the sync path already uses.
- **Make "don't saturate the edge" an explicit policy**, not an accident: disk-buffer cap, edge trace sample ratio (< 1.0), the minimum log level shipped to cloud, and what happens to the app's always-on stdout on a headless Windows service.
- All three signals — **logs, traces, and metrics** — are in scope.

Out of scope (deferred to follow-up tickets): Grafana per-instance dashboards, and "box offline / sync-lag" alerting. This change gets the data flowing and labeled; visualizing and alerting on it is separate.

## Capabilities

### New Capabilities
- `edge-telemetry-forwarding`: The edge sidecar that receives the app's OTLP (logs, traces) and scrapes its Prometheus metrics, buffers durably to disk within a bounded cap while offline, and forwards all three signals to the cloud ingest — with the app never blocking on export.
- `telemetry-instance-identity`: Per-instance identity (node id, organization, `mode=edge`) attached to the OTel resource / metric labels so every signal is attributable and filterable per edge box in Grafana Cloud.
- `cloud-telemetry-ingest`: The cloud-side authenticated telemetry ingest that verifies an edge node via `NODE_SYNC_KEY`, injects tenant context, and forwards logs/traces/metrics to Grafana Cloud, so edge nodes never hold Grafana Cloud credentials.

### Modified Capabilities
- (none — the project has no existing durable specs; `openspec list --specs` is empty. The behaviors above are introduced by the three new capabilities.)

## Impact

- **Go (small):** `observability.InitConfig` and `observability.Init` gain node/org/mode fields and set them as OTel resource attributes; `cmd/main.go` passes `cfg.NodeID`, `cfg.OrganizationID`, and `cfg.AppMode` into the observability bootstrap. No change to how services log — identity rides on the resource, so all existing signals inherit it.
- **Cloud backend (Option B):** a new telemetry-ingest path that authenticates edge nodes (reusing the `NodeAuthMiddleware` / `NODE_SYNC_KEY` model) and forwards to Grafana Cloud with credentials held only in the cloud. Design.md weighs implementing this as a Go OTLP endpoint vs. a cloud-side Alloy/collector fronted by node auth.
- **Deployment / non-Go footprint (the bulk):** an Alloy configuration for the edge, its Windows-service install, and the config/secret wiring (cloud ingest URL, node key, buffer cap, sample ratio, log level). This lives outside `internal/` — unusual for this repo's hexagonal layout, so it is called out explicitly.
- **Config:** new/edge-specific settings for the sidecar and identity; the existing `OTEL_EXPORTER_OTLP_ENDPOINT`, `OBSERVABILITY_ENABLED`, `OTEL_TRACES_SAMPLER_ARG`, and `METRICS_PORT` knobs are reused.
- **Transport separation:** telemetry uses its own path (edge Alloy → cloud ingest → Grafana Cloud), independent of the business-data sync (`CLOUD_SYNC_URL` / outbox). Same offline-tolerant spirit, separate channel.
- **Docs:** edge operator/runbook notes for installing and configuring the sidecar; no public API docs unless the cloud ingest exposes an HTTP endpoint (decided in design.md).
