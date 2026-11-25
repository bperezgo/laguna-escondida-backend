# Edge Sync

Endpoints the cloud serves to edge boxes. The edge binary calls them; nothing else should.

> **Cloud mode only.** Registered by the cloud binary; absent (404) on the edge.

**Auth:** node key, not JWT. Every request sends `X-Node-ID` (the box's node id) and `X-Node-Key`
(the key issued to that node when it enrolled, see [edge-enrollment.md](edge-enrollment.md)). The
cloud accepts it only when the key belongs to that node, the node is not revoked and its
organization is active, and then acts for that node's organization. There is no key shared
between boxes, and the cloud has no `NODE_SYNC_KEY` setting.

**Refusals:** `401` with one body for a missing key, wrong key, unknown node or another node's key;
`401` with `code: node_revoked` for a replaced box; `403` with `code: organization_suspended` for a
suspended organization. Every refusal carries `Retry-After` in seconds (default `21600`, 6 hours;
cloud setting `NODE_REFUSAL_RETRY_AFTER`). The box pauses push, pull and stock pull for that long
(clamped to 1 minute–24 hours, 6 hours when missing), then makes one call. It reports
`node_revoked` and `organization_suspended` as `sync_blocked` on `GET /api/edge/status`. A refused
node's repeat calls are answered from memory for up to 5 minutes, so a reactivation or new
enrollment can take up to 5 minutes to be accepted.

```
HTTP/1.1 403 Forbidden
Content-Type: application/json; charset=utf-8
Retry-After: 21600

{"error": "organization suspended", "code": "organization_suspended"}
```

## Endpoints

| Method | Endpoint                | Description                              |
| ------ | ----------------------- | ---------------------------------------- |
| POST   | `/api/sync/push`        | Edge pushes a batch of outbox operations |
| GET    | `/api/sync/pull`        | Edge pulls reference-data changes        |
| GET    | `/api/sync/pull/stock`  | Edge pulls the stock mirror              |

## Edge version

The edge sends `X-Edge-Version` on every sync request: the release version stamped into the
binary (`vX.Y.Z`; `dev` for an unstamped build).

| Env var            | Meaning                                                                 | Example   |
| ------------------ | ----------------------------------------------------------------------- | --------- |
| `MIN_EDGE_VERSION` | Oldest edge release allowed to sync. Unset accepts any. Must be `vX.Y.Z`. | `v1.3.0`  |
| `NODE_REFUSAL_RETRY_AFTER` | `Retry-After` on every node refusal (sync and telemetry). Go duration, `1m`–`24h`. Default `6h`. Takes effect on a box at its next refusal. | `1h` |

The cloud accepts the request when `MIN_EDGE_VERSION` is unset, the header is missing (a box that
predates it), the version is `dev` or unparsable, or the version is at or above the floor. Raise
the floor only in a cloud release that breaks the sync contract.

### Response when below the floor (426 Upgrade Required)

Checked after node auth, before the handler runs.

```json
{
  "error": "edge version is below the minimum supported",
  "min_version": "v1.3.0"
}
```

The box keeps working offline; it only stops syncing until the appliance is updated to
`min_version` or newer.
