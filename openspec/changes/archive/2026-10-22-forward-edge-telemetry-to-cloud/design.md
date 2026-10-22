## Context

See `proposal.md` (Why) for motivation and `specs/` for the behavior contract. The relevant current state:

- The in-app OTel pipeline is **built and committed** (`internal/platform/observability/observability.go`, `logging.go`, `slog_fanout.go`; `otelgin` in `cmd/main.go`; Prometheus `/metrics` on `:9090` via `handler/metrics_middleware.go`). Its package doc already states the intended shape: *"export over OTLP/gRPC to the local Alloy sidecar, which forwards to Grafana Cloud. The app never holds Grafana Cloud credentials."*
- `observability.Init` no-ops unless `OBSERVABILITY_ENABLED=true` and `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Its `InitConfig` carries service name/version/environment and a trace sample ratio — **but no node identity**. `NodeID`, `OrganizationID`, and `AppMode` exist in `config` (used by sync) and never reach observability.
- Edge nodes already authenticate to the cloud for data sync via `NODE_SYNC_KEY` + `NodeAuthMiddleware`.
- Destination is Grafana Cloud stack `wittycrane116` (Loki / Tempo / Mimir), which exposes a single OTLP gateway with basic auth.
- The edge is a Windows box, frequently offline; the cloud path is a separate AWS deployment that already sets the OTLP endpoint to a sidecar.

## Goals / Non-Goals

**Goals:**
- Durable, offline-tolerant forwarding of all three signals with a hard edge-disk cap.
- Per-instance identity on every signal, wired from existing config with the smallest possible app change.
- Zero Grafana Cloud credentials on the edge; a single authenticated cloud channel reusing node auth.

**Non-Goals (design-level):**
- Dashboards and alerting (deferred — see proposal).
- Profiling/Pyroscope, even though the stack supports it.
- Changing how services log or emit metrics — identity rides on the resource, not on call sites.

## Decisions

### D1 — Local agent is Grafana Alloy, running as a Windows service
The edge runs an Alloy instance that receives OTLP (gRPC `:4317` / HTTP `:4318`) on localhost, scrapes the app's `:9090/metrics`, buffers to disk, and forwards. **Why Alloy over app-direct-to-cloud:** the Go OTel SDK only has an in-memory batch queue — telemetry is lost on restart and can grow unbounded during an outage. Alloy gives a disk-backed queue with a size cap, which is exactly the offline + don't-saturate requirement. **Why Alloy over vanilla OTel Collector:** Grafana-native config and first-class Grafana Cloud integration; either would work, Alloy is the smaller operational bet here. The app already targets a localhost sidecar, so no app rewrite.

### D2 — Forward through the cloud backend (Option B), not directly to Grafana Cloud
The edge Alloy sends to a **node-authenticated telemetry ingest in the cloud backend**, which injects tenant/label context and relays to the Grafana Cloud OTLP gateway. **Why over direct (Option A):** edge boxes hold no Grafana Cloud credentials, rotation happens in one place, tenant isolation is enforced server-side, and it reuses the existing `NODE_SYNC_KEY` / `NodeAuthMiddleware` model instead of inventing new auth — matching the code's own *"tenant.id is injected by the backend proxy"* note. **Trade-off:** one extra hop and a cloud component that Option A avoids; acceptable because the ingest is telemetry-only (its downtime never touches the POS) and the node count is small.

### D3 — The cloud ingest authenticates and stamps; the cloud Alloy sidecar does the egress
Implement the ingest as an OTLP/HTTP endpoint on the cloud backend guarded by `NodeAuthMiddleware`, which stamps tenant/label context and forwards to the **cloud Alloy sidecar** (`TELEMETRY_FORWARD_URL`, OTLP/HTTP). Alloy holds the Grafana Cloud credentials and does the queueing, retrying and egress. **Why split this way:** auth must be in Go (only the backend knows node keys) and the tenant stamp is the security boundary (tested here), but egress is a job the cloud sidecar already does for this service's own telemetry — putting a second copy of the Grafana Cloud token in the app would mean two secrets, two rotation points, and a second egress path with no retry or queue at all. It also restores the invariant `internal/platform/observability` states in its package doc: *"the app never holds Grafana Cloud credentials."* **What this buys:** Alloy's processors (filter, tail sampling, routing, per-signal exporters) become available to edge telemetry by config change, and a Grafana Cloud outage is absorbed by the cloud queue instead of backing up onto the edge's bounded disk buffer (D6 lever 1). **Trade-off:** a 2xx to the edge now means the cloud sidecar accepted the batch, not that Grafana Cloud stored it — standard OTLP hop semantics, and durability moves to a disk-backed queue that is better placed than the edge's.

### D3a — Relayed edge telemetry gets its own receiver port on the cloud Alloy
The cloud Alloy exposes a receiver dedicated to relayed edge telemetry (e.g. `:4319`), separate from the one this backend's own telemetry uses. **Why:** a shared pipeline would apply the cloud service's external labels and identity processing to edge data, and would make per-edge filtering or sampling impossible to express. Three lines of Alloy config buy an explicit "edge data gets edge processing" boundary.

### D4 — One protocol edge→cloud: OTLP for all three signals
The edge Alloy converts scraped Prometheus metrics to OTLP and sends logs, traces, and metrics to the cloud ingest over OTLP. **Why:** a single authenticated channel and a single relay path, rather than a separate metrics `remote_write` that would need its own auth and its own cloud endpoint. Metrics are still Prometheus-native inside the app and inside Grafana Cloud (Mimir); OTLP is only the transport between edge Alloy and the ingest.

### D5 — Identity: app stamps logs/traces, Alloy stamps metrics, from one config source
Add `NodeID`, `OrganizationID`, and `Mode` to `observability.InitConfig` and set them as OTel resource attributes (`service.instance.id` = node id, plus `node.id`, `organization.id`, `deployment.mode=edge`); wire them from `cfg` in `cmd/main.go`. Metrics are scraped by Alloy, so Alloy adds the **same** identity as labels/external-labels (its config reads the node id/org from edge config). **Why split:** the app owns what it emits directly (logs/traces) and already knows `NodeID`; metrics identity is added where metrics are collected, so it stays correct even if the app never labels a metric. Both read identity from the same edge config values, so they cannot drift.

### D6 — "Don't saturate the edge" is four explicit levers
1. Alloy disk buffer: a bounded file-storage WAL. The cap is enforced in **bytes** (`sending_queue.sizer = "bytes"`), per signal. When it is full Alloy **rejects new telemetry** and keeps what is already queued — its persistent queue has no drop-oldest mode (`block_on_overflow` is the only overflow knob), so a cap-filling outage loses its tail, not its head.
2. Trace sampling: edge `OTEL_TRACES_SAMPLER_ARG` < 1.0 (already wired; default low, tunable).
3. Minimum forwarded log severity (INFO by default); sub-INFO not shipped.
4. App stdout on the headless Windows service: **not** persisted unbounded — either discarded (logs already ship via OTLP) or a small capped rotating file for on-box debugging.

## Risks / Trade-offs

- **Grafana Cloud volume/cost blowup** → sampling + min log level + low-cardinality identity labels (node/org/mode only); revisit if trace volume is high.
- **Alloy WAL still grows if misconfigured** → enforce the byte cap on the sending queue; note the cap is **per signal** and the bbolt file allocates roughly twice the queued bytes, so budget disk as ~6x the configured cap. A "buffer near cap" alert is a follow-up.
- **Cloud ingest is a telemetry SPOF** → by design telemetry-only; if it's down the edge buffers and retries, POS is unaffected.
- **Node key does double duty (sync + telemetry)** → a compromised key affects both channels; accepted for one node identity, rotation is cloud-side.
- **Edge clock skew distorts timestamps** → require NTP on the Windows box; note in the runbook.
- **OTLP-in-Go relay effort** → keep it a thin passthrough; the collector fallback (D3) is the escape hatch if it grows.

## Migration Plan

1. Land the app identity change (D5) — a no-op while `OBSERVABILITY_ENABLED=false`, safe to ship ahead of everything else.
2. Stand up the cloud ingest (D2/D3) behind `NodeAuthMiddleware`; hold Grafana Cloud creds in cloud env.
3. Pilot one edge box: install Alloy as a Windows service (D1), code is in `../laguna-escondida-edge`, point `OTEL_EXPORTER_OTLP_ENDPOINT` at localhost, set `OBSERVABILITY_ENABLED=true`, and verify logs/traces/metrics appear in Grafana Cloud filtered by node id.
4. Roll out to remaining boxes.

**Rollback:** set `OBSERVABILITY_ENABLED=false` (exporters no-op) and/or stop the Alloy service — the app is unaffected either way.

## Open Questions

- Concrete defaults for the Alloy disk cap and the edge trace sample ratio — tunable at deploy; does not change the approach or task breakdown.
- Keep a small capped rotating stdout file on the edge, or discard entirely — ops preference.
