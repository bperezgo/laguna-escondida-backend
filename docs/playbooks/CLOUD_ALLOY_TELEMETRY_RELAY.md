# Cloud Alloy — Edge Telemetry Relay (infra handoff)

**Audience:** whoever owns the `aws-infra` repo. Nothing in this document is applied by this
repository — it describes what the cloud Alloy sidecar needs so the backend's telemetry
ingest works end to end.

**Assumed infra shape:** ECS task running the `laguna-backend` container with a Grafana
Alloy sidecar in the same task definition, `awsvpc` network mode (so both containers share
a network namespace and reach each other over `127.0.0.1` with no port mapping or security
group rule).

---

## 1. What the backend now does, and what it needs from you

The backend exposes `POST /api/telemetry/v1/{logs,traces,metrics}`, authenticates the edge
node with `X-Node-Key`, stamps `tenant.id` / `organization.id` onto every resource, and then
**forwards the export to the Alloy sidecar over OTLP/HTTP** — gzipped protobuf, on the
standard `/v1/logs`, `/v1/traces`, `/v1/metrics` paths.

It attaches **no credential** of its own. Alloy is expected to authenticate to Grafana Cloud
and to own the retry and queueing.

| Backend needs | Value |
| --- | --- |
| `TELEMETRY_FORWARD_URL` | Base URL of the Alloy receiver for relayed edge telemetry, e.g. `http://127.0.0.1:4319` |

Without that variable set (or outside `APP_MODE=cloud`) the ingest routes are simply not
registered and the backend logs `Telemetry ingest disabled` at startup. It still boots.

---

## 2. Changes to the ECS task definition

**Backend container**
- **Add** env var `TELEMETRY_FORWARD_URL` pointing at the sidecar's new receiver port.
- **Remove** `GRAFANA_CLOUD_OTLP_URL`, `GRAFANA_CLOUD_OTLP_USERNAME`, `GRAFANA_CLOUD_OTLP_TOKEN`
  from this container's `environment` / `secrets`. The backend no longer reads them, and
  leaving them attached keeps a secret mounted where nothing uses it.
- Consider a `dependsOn` on the Alloy container (`condition: START`, or `HEALTHY` if the
  sidecar has a health check) so the backend doesn't come up forwarding into a void. Not
  strictly required — a failed forward returns 502 and the edge retries from its own disk
  buffer — but it removes a noisy window on every deploy.

**Alloy container**
- Keep the Grafana Cloud credentials here, injected via ECS `secrets` from Secrets Manager
  or SSM Parameter Store. **This should now be the only place in the task they exist.**
- No new `portMappings` entry is needed for the relay port: it is task-internal over
  loopback. Do **not** expose it to the load balancer or a security group.
- Review CPU/memory. The sidecar previously handled only this service's own telemetry; it
  now also carries every edge box's logs, traces and metrics. Size from the expected node
  count and give it headroom before the first pilot.

**IAM**
- The backend's task role no longer needs read access to the Grafana Cloud secret. The
  execution role still needs it, for the Alloy container's secret injection.

---

## 3. Changes to the Alloy configuration

The key decision: **relayed edge telemetry gets its own receiver and its own pipeline**,
separate from the one handling this service's own telemetry.

*Why:* a shared pipeline would apply the cloud service's own labels and identity processing
to edge data, silently mislabelling which box a log came from. A separate pipeline also
makes per-edge filtering and sampling expressible at all.

### 3.1 New receiver

An OTLP **HTTP** receiver bound to loopback on a dedicated port (e.g. `:4319`), distinct
from the receiver the backend's own SDK exports to (`:4317` gRPC, via
`OTEL_EXPORTER_OTLP_ENDPOINT`). It must accept gzip-encoded protobuf — the default for the
OTLP HTTP receiver — and handle all three signals.

### 3.2 Pipeline for that receiver

Route its logs, traces and metrics to an exporter targeting the Grafana Cloud OTLP gateway,
authenticated with basic auth (instance ID + API token).

**Must not**: overwrite or re-derive `service.name`, `service.instance.id`, `node.id`,
`organization.id`, or `tenant.id`. The edge app stamps the first set and the backend ingest
stamps the tenant context — anything the cloud pipeline adds on top will make edge telemetry
look like it came from the cloud service.

**Batching** is worth adding before the exporter; the edge already batches, but a second
batch smooths bursts from many nodes arriving at once.

### 3.3 Queue and retry — the main reason for this design

Enable `retry_on_failure` and a `sending_queue` on the exporter. This is what the change
buys: a Grafana Cloud outage is absorbed here instead of backing up onto the edge boxes'
bounded disk buffers, which is the "don't saturate the edge" constraint the whole feature
was built around.

Decide between:
- **In-memory queue** — simplest, but a task restart or redeploy drops whatever is queued.
- **File-backed queue** — survives a container restart within the same task, but on Fargate
  the ephemeral volume does not survive a *task* replacement, so it is a partial guarantee.
  Needs a writable volume mounted into the Alloy container and a bounded size.

For the pilot, in-memory with a sane cap is likely enough — the edge retains its own disk
buffer and will resend anything the cloud rejected. Revisit if edge buffers start filling.

### 3.4 Optional, once it works

These are the capabilities that motivated routing through Alloy rather than exporting from
the Go app, and they're all config-only changes from here:

- **Filtering** — drop noisy log records or spans per node, severity or attribute.
- **Tail sampling** — keep traces that errored or ran slow, drop the rest, decided after the
  full trace arrives. The edge's head-based sampler can't do this.
- **Routing** — send a specific node's telemetry to a different tenant or a debug stack.
- **Per-signal exporters / fan-out** — mirror a signal to a second backend.

---

## 4. Verification

1. **Config validity** — `alloy fmt` / validate the config before deploying.
2. **Sidecar reachable** — from the backend container, a POST to
   `http://127.0.0.1:4319/v1/logs` returns 2xx.
3. **End to end** — use the smoke-test curls in `docs/examples/curl-examples.md` (§ Telemetry
   Ingest) with a valid `NODE_SYNC_KEY`; confirm the export lands in Grafana Cloud.
4. **Identity preserved** — the arriving telemetry is filterable by the originating box's
   `node.id` and carries the expected `tenant.id`, and is **not** labelled as the cloud
   service's own telemetry.
5. **Secret hygiene** — `GRAFANA_CLOUD_OTLP_*` appears on the Alloy container only. Confirm
   against the rendered task definition, not just the source template.
6. **Outage behaviour** — block egress to Grafana Cloud briefly; confirm Alloy queues and
   retries rather than erroring straight back to the edge, and that the queue stays bounded.

---

## 5. Rollback

Unset `TELEMETRY_FORWARD_URL` on the backend container and redeploy: the ingest routes stop
being registered, the backend logs `Telemetry ingest disabled`, and edge boxes get a 404 and
hold their telemetry in their local buffers. Nothing else in the backend is affected — the
POS path never touches this code.

---

## Reference

- Endpoint contract, error semantics, full env tables — `docs/api/telemetry.md`
- Decisions and rationale (D3, D3a) — `openspec/changes/forward-edge-telemetry-to-cloud/design.md`
- Outstanding task — 2.5 in that change's `tasks.md`, which this document covers
