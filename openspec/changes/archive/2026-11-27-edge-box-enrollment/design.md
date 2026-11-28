## Context

See proposal.md for motivation. Current state:

- `NodeAuthMiddleware` (cloud) compares `X-Node-Key` with the one `NODE_SYNC_KEY` from the environment, then resolves the organization from `nodes[X-Node-ID]`. It guards `/api/sync/*` and `/api/telemetry/v1/*`.
- `nodes` has `id, name, kind, organization_id, created_at`. Its only row comes from migration 000071 (Laguna's box). The table exists only in the cloud: the edge schema drops it (migration 000072, edge).
- The edge reads `NODE_ID`, `ORGANIZATION_ID`, `CLOUD_SYNC_URL` and `NODE_SYNC_KEY` from its env file and sends `X-Node-ID` and `X-Node-Key` on every sync call. Alloy sends the same pair for telemetry, and `install.ps1` insists the two keys match.
- `api_key_service.go` already issues `lgk_` keys stored as a SHA-256 hex digest plus a recognition prefix.
- `/api/edge/status` is unauthenticated and feeds the POS its online and backlog state from `syncstatus.Tracker`.

## Goals / Non-Goals

**Goals:**

- A node key authenticates exactly one node; the organization follows from that node.
- Enrollment needs nothing but an admin's user token, so an installer can call it later.
- Laguna's box keeps its node id, so its sync history and outbox stay valid.

**Non-Goals:**

- No change to the edge's env variable names or to what the edge sends. `NODE_SYNC_KEY` stays the edge-side name of the box's key.
- No rate limit on enrollment: it sits behind an admin sign-in, and sign-in is the abuse surface.
- No list or revoke endpoints for boxes. Replacement is the only way to revoke for now.
- No behavior on the box for a suspended organization beyond reporting it.

## Decisions

### 1. Per-node key stored as a SHA-256 digest, same scheme as `lgk_` keys

Key is `lgn_` + 32 random bytes, base64url. The cloud stores the hex SHA-256 and the first characters after the prefix. A random 256-bit key needs no slow hash, and the project already made this choice for API keys.

Alternative: one signed token (JWT) per box. Rejected: revocation would need a denylist lookup anyway, so the token buys nothing over a row lookup.

### 2. Auth is "load the node by `X-Node-ID`, compare digests in constant time"

The middleware loads the node row by id, hashes the presented key and compares it to `key_hash` with `subtle.ConstantTimeCompare`. A row with a `NULL` hash never matches. Order of checks: key match first; only then revoked (`401 node_revoked`), then organization status (`403 organization_suspended`). Checking the key first is what keeps the two reasons from leaking to a caller without the key. Unknown node, missing key and wrong key share one `401` body.

`NodeService.OrganizationOf` becomes one call, `Authenticate(ctx, nodeID, key)`, returning the organization id or one of three domain errors. Organization status comes from a join in the node repository query, so auth stays one query.

Alternative: look the node up by key hash alone and drop `X-Node-ID`. Rejected: the edge and Alloy already send both, and `X-Node-ID` is also stamped on telemetry.

### 3. `nodes` gains five columns and a partial unique index (cloud migration)

`key_hash CHAR(64) NULL`, `key_prefix VARCHAR(16) NULL`, `enrolled_at`, `revoked_at`, `last_seen_at` (all nullable timestamps), plus:

```sql
CREATE UNIQUE INDEX uq_nodes_one_active_edge_per_org
  ON nodes (organization_id) WHERE kind = 'edge' AND revoked_at IS NULL;
```

The index is what makes concurrent enrollments safe: the loser gets a unique violation, mapped to `409`. The migration goes under `migrations/cloud/` because the edge has no `nodes` table.

### 4. Enrollment is one transaction with three paths

Inside a unit of work, lock the organization's active edge node (`SELECT ... FOR UPDATE`):

| Active node | `replace` | Result |
|---|---|---|
| none | any | insert a node with a new id and key |
| has no key (pre-existing) | any | set key, name and `enrolled_at` on that row; same node id |
| has a key | `false` | `409 box_already_enrolled` |
| has a key | `true` | set `revoked_at` on it, insert a new node |

The second row is the migration path for Laguna's box: changing its `NODE_ID` would orphan the origin id on its unsynced outbox rows, so the legacy row is adopted, not replaced.

Route: `POST /api/edge/enroll`, cloud routes only, `JWTAuthMiddleware` + `RequirePermission(permissions.EdgeEnroll)`. New permission `edge:enroll`, granted to the admin role only. The organization comes from the token.

### 4b. The response carries the sync address, taken from `PUBLIC_API_URL`

A new required cloud setting, `PUBLIC_API_URL`, read and validated in `LoadCloud` (absolute `http`/`https`, host present, no path, query or fragment; trailing slash trimmed). The enroll service receives it at construction and copies it into `EnrolledEdgeBox.CloudSyncURL` (`cloud_sync_url`). The box writes it as `CLOUD_SYNC_URL`; the sync clients build `<base>/api/sync/...`, which is why a path is refused.

Why a cloud setting and not a value built into the edge binary or the installer: the deployment already knows its own address, and one edge release then works against any cloud (production, staging, a local one).

Limits, accepted:

- The caller still needs an address to reach sign-in and enrollment in the first place. The installer has one built in; it will normally equal `PUBLIC_API_URL`.
- A box learns the address only when it enrolls. Changing the setting later does not reach boxes already enrolled, so the value must be a domain the owner controls (not a load balancer hostname), and moving infrastructure must be a DNS change.

Alternative: derive it from the request's `Host` header. Rejected: behind the load balancer and CDN the header is not reliably the public name, and it would let a caller choose what the box stores.

Alternative: make the setting optional and omit the field when unset. Rejected: an installer would then have to guess, and a silent omission is found only at a customer's install.

### 5. `last_seen_at` is written at most once a minute per node

Telemetry calls arrive every few seconds; one update per request would be a write per call. The auth query returns `last_seen_at`, and the middleware issues `UPDATE ... WHERE id = $1 AND (last_seen_at IS NULL OR last_seen_at < now() - interval '1 minute')` only when the loaded value is older than a minute. A failed update is logged and does not fail the request.

### 6. The box learns the reason from the response `code`

The three sync clients return a typed error when the cloud answers `401` with `code: node_revoked` or `403` with `code: organization_suspended`. The sync scheduler passes it to `syncstatus.Tracker`, which gains a blocked reason next to its online flag; any success clears it. `EdgeStatusResponse` gains `sync_blocked`. State is in memory: after a restart the next sync attempt (within a minute) sets it again.

The box keeps retrying on its normal schedule. A revoked box wastes one small request a minute; a suspended one recovers by itself when the organization is reactivated.

### 7. `NODE_SYNC_KEY` leaves the cloud, stays on the edge

`loadShared()` reads it for both binaries today. It moves to the edge config; the cloud binary stops reading it and `registerCloudRoutes` loses the parameter. No fallback path (owner's decision): from the cloud release until Laguna's box is enrolled, that box cannot sync.

`PushNodeIDFromBody` stays as it is; it runs before auth and only fills the header.

## Risks / Trade-offs

- [Laguna's box cannot sync between the cloud deploy and its enrollment] → The box keeps selling and queues its outbox; nothing is lost. Enroll right after the deploy (see Migration Plan). The pull cursor and outbox are untouched, so sync resumes where it stopped.
- [A leaked node key lets its holder push data as that one restaurant] → Scope is one organization; replacement revokes it. The key sits in two gitignored files on the box, as today.
- [Replacing a box discards whatever the old one had not pushed] → The `409` carries `last_seen_at` so the caller can warn before sending `replace`.
- [An admin token can enroll from anywhere, replacing the real box] → Requires an admin's credentials and an explicit `replace`; the replaced box says so on its screen. A browser approval flow can tighten this later without changing the endpoint.
- [Reinstalling on the same PC while keeping its database gives old outbox rows a new node id] → Not reachable in this change (no installer yet); the installer change must decide between wiping the database and re-using the node.

## Migration Plan

0. aws-infra sets `PUBLIC_API_URL` to the cloud's public domain. This comes first: the new cloud image refuses to boot without it.
1. Release the cloud with this change (`cloud/vX`). The migration adds the columns and the index; Laguna's row has no key, so its box starts getting `401`.
2. Sign in to the cloud as Laguna's admin, call `POST /api/edge/enroll`. The response returns the same node id, a new key, and `cloud_sync_url`, which should equal the box's current `CLOUD_SYNC_URL`.
3. On the box, set `NODE_SYNC_KEY` in `env\edge-node.env` and `EDGE_NODE_KEY` in `env\alloy.env` to the new key, re-run `install.ps1`, confirm `/api/edge/status` reports `online: true`.
4. aws-infra removes `NODE_SYNC_KEY` from the cloud's configuration.

The edge half (reading `code`, `sync_blocked`) ships in the next `edge/v*`; an older box simply shows offline when refused. `edge/compat.json` needs no `min_cloud` bump: the edge code works against an older cloud, which just never sends the two codes. What ties a box to the new cloud is its key, not its version.

Rollback: redeploy the previous cloud image with `NODE_SYNC_KEY` set and put the old shared key back on the box. The down migration drops the index and columns.

## Implementation map

- `backend/internal/platform/postgres/migrations/cloud/` : new migration (columns, partial unique index)
- `backend/internal/domain/ports/node_repository.go`, `backend/internal/platform/postgres/repository/node_repository.go`
- `backend/internal/domain/service/node_service.go` (+ test); `api_key_service.go` for the key and hash pattern to copy
- `backend/internal/domain/error/` : node errors (invalid key, revoked, organization suspended, already enrolled)
- `backend/internal/domain/dto/` : enrollment request and response, `lgn_` prefix
- `backend/internal/domain/permissions/permissions.go`, `role_permissions.go` : `edge:enroll`
- `backend/internal/platform/handler/middleware.go` (`NodeAuthMiddleware`), `node_auth_middleware_test.go`, new enroll handler
- `backend/internal/bootstrap/routes_cloud.go`, `cloud.go`
- `backend/internal/platform/config/shared.go`, `edge.go`, `cloud.go` (+ tests) : move `NODE_SYNC_KEY` to the edge; `PUBLIC_API_URL` in `cloud.go`
- `backend/.env.example`, and every test harness that builds a cloud config (`backend/internal/bootstrap/*_test.go`, `backend/test/acceptance/*/harness_test.go`) : the new required setting
- `backend/internal/platform/httpclient/sync_push_client.go`, `sync_pull_client.go`, `sync_stock_pull_client.go`
- `backend/internal/platform/syncstatus/`, the sync scheduler that calls it, `backend/internal/platform/handler/edge_handler.go`
- `backend/test/acceptance/` : node auth and enrollment
- `backend/docs/api/edge-enrollment.md` (new), `sync.md`, `telemetry.md`
- `frontend/types/edge.ts`, `frontend/lib/api/edge.ts`, `frontend/app/api/edge/status/route.ts`, and the component that shows the offline state
- `edge/env/edge-node.env.example`, `edge/env/alloy.env.example`, `edge/scripts/install.ps1` (wording only)

## Open Questions

- Final Spanish wording of the two POS messages; tasks.md 5.2 carries a working draft.
