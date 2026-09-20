# Telemetry Ingest API

Node-authenticated OTLP entry point used by an edge box's Grafana Alloy sidecar to ship its
logs, traces, and metrics to Grafana Cloud. The cloud stamps the tenant context and hands
the export to its **own Alloy sidecar**, which holds the Grafana Cloud credentials and does
the queueing and retrying. So **no part of this backend holds Grafana Cloud credentials** —
the edge holds only its `NODE_SYNC_KEY` and the address of this ingest.

```
edge Alloy ──X-Node-Key──▶ cloud backend ──OTLP/HTTP──▶ cloud Alloy ──auth──▶ Grafana Cloud
             (this API)     auth + tenant stamp          queue + retry
```

The cloud Alloy listens for relayed edge telemetry on a **port of its own**, separate from
the one this backend's own telemetry uses, so edge data gets its own pipeline (filtering,
tail sampling, routing) instead of inheriting the cloud service's labels. What that sidecar
needs on the ECS side is written up in `docs/playbooks/CLOUD_ALLOY_TELEMETRY_RELAY.md`.

> **Cloud mode only.** These routes are registered only when the backend runs with
> `APP_MODE=cloud` **and** `TELEMETRY_FORWARD_URL` is set. Otherwise they do not exist (404)
> and the backend logs `Telemetry ingest disabled` at startup.

> **Not a browser/frontend API.** The only client is the edge Alloy sidecar. Nothing in the
> POS UI calls these endpoints.

## Endpoints

| Method | Endpoint                      | Description                              |
| ------ | ----------------------------- | ---------------------------------------- |
| POST   | `/api/telemetry/v1/logs`      | Relay an OTLP logs export to Grafana Cloud    |
| POST   | `/api/telemetry/v1/traces`    | Relay an OTLP traces export to Grafana Cloud  |
| POST   | `/api/telemetry/v1/metrics`   | Relay an OTLP metrics export to Grafana Cloud |

The paths mirror the OTLP/HTTP layout, so an OTLP exporter only needs
`https://<cloud-host>/api/telemetry` as its base endpoint.

---

## Relay an OTLP Export

`POST /api/telemetry/v1/{logs|traces|metrics}`

**Auth:** `X-Node-Key` header, matched against the cloud's `NODE_SYNC_KEY` — the same node
credential the sync endpoints use. Fails closed: with no node key configured, every request
is rejected.

### Request Headers

| Header             | Required | Description                                                       |
| ------------------ | -------- | ----------------------------------------------------------------- |
| `X-Node-Key`       | Yes      | The edge node's sync key                                           |
| `Content-Type`     | Yes      | Must be `application/x-protobuf`                                   |
| `Content-Encoding` | No       | `gzip` if the body is compressed (what Alloy sends by default)      |

### Request Body

The raw OTLP protobuf export request for the signal — `ExportLogsServiceRequest`,
`ExportTraceServiceRequest`, or `ExportMetricsServiceRequest`. Max **16 MiB** decompressed.

**OTLP/JSON is not accepted.** Its trace and span ids are hex strings, which a generic
protobuf-JSON decoder does not produce, so accepting JSON would silently corrupt ids.
Configure the exporter for protobuf encoding.

### Tenant Context

Before forwarding, the ingest sets these resource attributes on **every** resource in the
export, replacing any value the edge sent:

| Attribute         | Source                                          |
| ----------------- | ----------------------------------------------- |
| `tenant.id`       | `TELEMETRY_TENANT_ID` (defaults to `ORGANIZATION_ID`) |
| `organization.id` | `ORGANIZATION_ID`                               |

Identity the edge stamps itself (`node.id`, `service.instance.id`, `deployment.mode`) is
left untouched, so telemetry stays filterable per box.

### Example Response (200 OK)

An empty body with `Content-Type: application/x-protobuf` — a valid OTLP
`ExportXServiceResponse` with no `partial_success`, meaning the whole export was accepted.

```
HTTP/1.1 200 OK
Content-Type: application/x-protobuf
Content-Length: 0
```

### Error Responses

**401 Unauthorized** — missing or invalid `X-Node-Key`. Nothing is forwarded.

```json
{
  "error": "invalid node key"
}
```

**415 Unsupported Media Type** — `Content-Type` is not `application/x-protobuf`.

```json
{
  "error": "telemetry must be sent as application/x-protobuf"
}
```

**400 Bad Request** — empty or undecodable body, or an invalid gzip stream. The export is
permanently bad; an OTLP client should drop the batch rather than retry.

```json
{
  "error": "malformed telemetry payload: cannot parse invalid wire-format data"
}
```

**502 Bad Gateway** — the cloud Alloy sidecar was unreachable or rejected the export. This
is **retryable**: Alloy keeps the batch in its disk buffer on the edge and resends it. Note
that a 200 means the cloud sidecar accepted the batch, not that Grafana Cloud has stored it
— from that point on, delivery is the cloud sidecar's queue to guarantee.

```json
{
  "error": "could not forward telemetry upstream"
}
```

## Configuration

### Cloud backend (holds no Grafana Cloud credentials)

| Variable                | Required | Description                                                                       |
| ----------------------- | -------- | --------------------------------------------------------------------------------- |
| `TELEMETRY_FORWARD_URL` | Yes      | Cloud Alloy OTLP/HTTP receiver for relayed edge telemetry, e.g. `http://127.0.0.1:4319` |
| `TELEMETRY_TENANT_ID`   | No       | `tenant.id` stamped on relayed telemetry; defaults to `ORGANIZATION_ID`           |
| `NODE_SYNC_KEY`         | Yes      | Shared with edge nodes; already required by the sync endpoints                    |

The Grafana Cloud URL, instance ID, and token live in the **cloud Alloy sidecar's** config,
not here — one place to rotate them, shared with the sidecar's existing egress path.

### Edge (holds no Grafana Cloud credentials either)

| Variable                      | Description                                                        |
| ----------------------------- | ------------------------------------------------------------------ |
| `NODE_SYNC_KEY`               | Sent as `X-Node-Key` by the Alloy sidecar                            |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | The **local** Alloy sidecar, e.g. `http://127.0.0.1:4317`            |
| `OBSERVABILITY_ENABLED`       | `true` to start the app's OTLP exporters                             |
| `NODE_ID`, `ORGANIZATION_ID`  | Stamped onto the app's OTel resource as per-box identity             |
