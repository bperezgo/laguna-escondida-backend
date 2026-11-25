# Edge Box Enrollment

How an organization's admin registers the restaurant's edge box with the cloud, and what the box does once the cloud refuses it.

- **Cloud only.** The edge does not serve `POST /api/edge/enroll` (`404`).
- **One active box per organization.** Enrolling while one is active is refused with `409` unless `replace` is `true`, which revokes the old box and registers the new one in one step.
- **Its own key.** Each box gets its own `lgn_` key. The cloud stores only a SHA-256 hash and the first 8 characters after `lgn_`; the plaintext is in the enrollment response only. A key works for its own node and therefore only for that node's organization.
- **Revoked boxes are kept**, never deleted.

## Endpoints

| Method | Endpoint           | Auth                              | Description                    |
| ------ | ------------------ | --------------------------------- | ------------------------------ |
| POST   | `/api/edge/enroll` | Admin's token (`edge:enroll`)     | Register a box, get its key    |
| GET    | `/api/edge/status` | None                              | Box connectivity (see below)   |

## Enroll a box

`POST /api/edge/enroll`

The organization is always the token's own. A manager's token is refused.

### Request

```json
{ "name": "Caja principal", "replace": false }
```

| Field     | Type    | Required | Description                                               |
| --------- | ------- | -------- | --------------------------------------------------------- |
| `name`    | string  | Yes      | 1 to 255 characters after trimming                        |
| `replace` | boolean | No       | Default `false`. `true` revokes the organization's active box |

### Response — 201 Created

```json
{
  "node_id": "ce6ebef8-37cf-576b-b7a8-15aaaaae44a1",
  "organization_id": "bb0cad3e-be13-4bad-9a37-bba9525bfece",
  "organization_name": "Laguna Escondida",
  "node_key": "lgn_q3L0...43 characters",
  "cloud_sync_url": "https://api.example.com"
}
```

`node_key` is shown once; the response is sent with `Cache-Control: no-store`. On the box, set `NODE_ID` to `node_id` and `ORGANIZATION_ID` to `organization_id`, and `NODE_SYNC_KEY` (edge) and `EDGE_NODE_KEY` (Alloy) to `node_key`. `CLOUD_SYNC_URL` to `cloud_sync_url`, used as is.

`cloud_sync_url` is the cloud's `PUBLIC_API_URL` setting, with a trailing slash removed. The cloud refuses to boot unless it is an absolute `http` or `https` URL with a host and no path, query or fragment. A box keeps the value it was given, so set it to a domain you control, not a load balancer hostname.

A box the cloud registered before node keys existed keeps its `node_id`: the enrollment gives that row its key and the name sent, with or without `replace`. Until then it is refused with `401`.

### Errors

**400 Bad Request** - Empty or too long `name`, or an invalid body

```json
{ "error": "box name is required", "field": "name" }
```

**401 Unauthorized** - Missing or invalid token

**403 Forbidden** - The token's user lacks `edge:enroll` (only admin holds it)

**404 Not Found** - Sent to an edge box

**409 Conflict** - The organization has an active box and `replace` is not `true`, or two enrollments raced and this one lost (`active_box` is then `null`)

```json
{
  "error": "organization already has an active box",
  "code": "box_already_enrolled",
  "active_box": {
    "name": "Caja principal",
    "enrolled_at": "2026-11-24T15:00:00Z",
    "last_seen_at": "2026-11-24T22:41:00Z"
  }
}
```

`last_seen_at` is `null` when the box has never called. Warn before sending `replace`: whatever the old box had not pushed is discarded.

## Box refusals

A box's sync and telemetry requests are refused as follows. The reasons are given only to a caller that presents the node's correct key. Telemetry routes answer every refusal with `429` and the same body, because the box's telemetry collector (Alloy) only waits out a `429`.

| Sync status | Telemetry status | Body                                                                   | Meaning |
| ----------- | ---------------- | ---------------------------------------------------------------------- | ------- |
| 401         | 429              | `{"error": "invalid node credentials"}`                                | Missing key, wrong key, unknown node, or another node's key; one body for all |
| 401         | 429              | `{"error": "node revoked", "code": "node_revoked"}`                    | The box was replaced |
| 403         | 429              | `{"error": "organization suspended", "code": "organization_suspended"}` | The organization is suspended; works again once reactivated |

Every refusal carries `Retry-After: <seconds>`, from the cloud setting `NODE_REFUSAL_RETRY_AFTER` (Go duration between `1m` and `24h`, default `6h`, so `21600`). An accepted request has no `Retry-After`. The box stops syncing for that long, then makes one call; restarting the edge service ends the pause early. The cloud remembers a refusal for up to 5 minutes, so a reactivation takes up to 5 minutes to reach the box's next call.

## Box status

`GET /api/edge/status` (on the box) includes `sync_blocked`:

| Value         | Meaning                                                              |
| ------------- | -------------------------------------------------------------------- |
| `"replaced"`  | The cloud last refused the box with `node_revoked`                   |
| `"suspended"` | The cloud last refused the box with `organization_suspended`         |
| `null`        | Otherwise, and always in the cloud; back to `null` after the next accepted sync |

While it is set, `online` is `false`, the box keeps serving the restaurant locally, and the POS tells admins and managers why. A plain network failure leaves it `null`.
