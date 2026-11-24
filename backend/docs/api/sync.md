# Edge Sync

Endpoints the cloud serves to edge boxes. The edge binary calls them; nothing else should.

> **Cloud mode only.** Registered by the cloud binary; absent (404) on the edge.

**Auth:** node key, not JWT. Every request sends `X-Node-Key` (the shared `NODE_SYNC_KEY`) and
`X-Node-ID` (a node registered to an organization). The cloud acts for that organization.

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
