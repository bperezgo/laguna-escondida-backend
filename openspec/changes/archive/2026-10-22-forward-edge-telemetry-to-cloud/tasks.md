## 1. Per-instance identity (Go) — safe to ship first

- [x] 1.1 Add `NodeID`, `OrganizationID`, and `Mode` fields to `observability.InitConfig` and set them as OTel resource attributes (`service.instance.id` = node id, plus `node.id`, `organization.id`, `deployment.mode`); verify with a unit test asserting the built resource carries all four attributes when enabled.
- [x] 1.2 Pass `cfg.NodeID`, `cfg.OrganizationID`, and `cfg.AppMode` from `cmd/main.go` into `observability.Init`; verify `go build ./...` succeeds and the app boots with the new wiring.
- [x] 1.3 Verify the no-op path is unchanged: with `OBSERVABILITY_ENABLED=false` `Init` still returns a no-op shutdown and no exporters start (existing `observability_test.go` passes); run `make lint` and `go test ./internal/platform/observability/...`.

## 2. Cloud telemetry ingest (Option B)

- [x] 2.1 Add a node-authenticated telemetry ingest endpoint on the cloud backend guarded by `NodeAuthMiddleware`; verify a handler test where a valid `NODE_SYNC_KEY` is accepted and a missing/invalid key is rejected (401/403) with nothing forwarded.
- [x] 2.2 Forward received OTLP (logs, traces, metrics) to the cloud Alloy sidecar over OTLP/HTTP, injecting tenant/label context on the way through; verify a test against a stub collector shows all three signal types relayed with the tenant context added by the ingest and no credential attached to the request.
- [x] 2.3 Add cloud-only config for the sidecar's receiver URL (`TELEMETRY_FORWARD_URL`); verify the ingest routes are wired only in cloud mode with it set, and that no Grafana Cloud credentials appear in any backend or edge config.
- [ ] 2.5 Add a dedicated OTLP/HTTP receiver for relayed edge telemetry on the cloud Alloy sidecar (its own port, its own pipeline, exporting to Grafana Cloud with a `sending_queue` + `retry_on_failure`); verify an export POSTed to the ingest reaches Grafana Cloud and is not labelled as the cloud service's own telemetry. *(Config lives in the `aws-infra` repo, not here — handoff plan in `docs/playbooks/CLOUD_ALLOY_TELEMETRY_RELAY.md`.)*
- [x] 2.4 If steps 2.1–2.2 introduce a new port (e.g., a telemetry forwarder), run `make generate-mocks` and add the corresponding service test; verify `go test ./...` passes.

## 3. Edge Alloy sidecar

- [x] 3.1 Author the edge Alloy config: OTLP receivers on `localhost:4317`/`:4318`, Prometheus scrape of `localhost:9090`, convert scraped metrics to OTLP, and a single OTLP exporter to the cloud ingest authenticated with the node key; verify the config passes `alloy fmt`/validation and a local run receives and forwards all three signals.
- [x] 3.2 Configure disk-backed buffering with a bounded byte cap, discarding new telemetry when full; verify by simulating an offline period (ingest unreachable) then reconnecting — buffered telemetry is delivered and the on-disk buffer stays bounded by the cap.
- [x] 3.3 Inject the same identity labels (`node.id`, `organization.id`, `deployment.mode`) onto scraped metrics from edge config, matching the app's resource attributes; verify a metric in Grafana Cloud is filterable by node id.
- [ ] 3.4 Package Alloy as a Windows service that starts on boot; verify the service runs, survives reboot, and is listening on the localhost OTLP and scrape ports.

## 4. Edge volume + stdout controls

- [x] 4.1 Set edge defaults and make them configurable: trace sample ratio (`OTEL_TRACES_SAMPLER_ARG` < 1.0), minimum forwarded log severity (INFO), and the Alloy buffer cap size; verify DEBUG logs are not forwarded while INFO logs are.
- [x] 4.2 Define the app's stdout policy on the headless Windows service (discard, since logs ship via OTLP, or a small capped rotating file); verify stdout does not grow unbounded over a sustained run.

## 5. Rollout + end-to-end verification

- [ ] 5.1 Pilot one edge box: set `OBSERVABILITY_ENABLED=true` and point `OTEL_EXPORTER_OTLP_ENDPOINT` at the localhost Alloy; verify logs, traces, and metrics all appear in Grafana Cloud filtered to that box's node id.
- [ ] 5.2 Offline/reconnect end-to-end on the pilot: disconnect the box, generate telemetry and POS activity, then reconnect; verify the backlog is delivered, the cap held during the outage, and POS request handling was never blocked or slowed.
- [ ] 5.3 Rollback check: set `OBSERVABILITY_ENABLED=false` and/or stop the Alloy service; verify the app stays healthy with no telemetry-related errors.
- [ ] 5.4 Write the edge operator runbook (install/configure the sidecar, NTP requirement, tunables for cap/sample/log-level); verify a fresh install can be completed by following it.

## 6. Final guardrails

- [x] 6.1 Verify the full repo gate is green: `make lint`, `go build ./...`, `go test ./...`, and `openspec validate forward-edge-telemetry-to-cloud`.
