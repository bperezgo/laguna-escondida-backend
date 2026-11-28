## Why

Every edge box authenticates to the cloud with one shared `NODE_SYNC_KEY`, and the organization comes from the `X-Node-ID` header, so anyone holding that key can sync as any restaurant. Boxes are also registered only by a database migration, and their identity is typed by hand into an env file. Both block the goal of a customer installing the box alone from a public download.

## What Changes

- New cloud endpoint `POST /api/edge/enroll`: a signed-in admin of an organization registers a box and receives its `organization_id`, `node_id`, a node key shown once, and `cloud_sync_url`, the address the box syncs against. That address comes from a new required cloud setting, `PUBLIC_API_URL`, so the deployment decides where boxes sync.
- The cloud stores only a hash of each node key. A key is valid only for its own node and therefore only for that node's organization.
- One active box per organization. Enrolling while one is active is refused with the active box's name and last-seen time; enrolling with an explicit replace revokes the old box and registers the new one.
- A revoked box, or a box of a suspended organization, is refused on sync and telemetry.
- The box records that the cloud refused it as replaced, and the POS tells the admin instead of queueing silently.
- The cloud records when each box last called it.
- **BREAKING**: the shared `NODE_SYNC_KEY` is removed from the cloud. There is no fallback: Laguna's existing box stops syncing until it is enrolled and its env file updated, which the owner does by hand. Its enrollment keeps its existing node id.

Not in this change: the installer, the edge's unused cloud secrets (`edge-no-cloud-secrets`), a browser approval flow, and more than one box per organization.

## Capabilities

### New Capabilities

- `edge-box-enrollment`: registering a box for an organization, issuing and verifying its own node key, the one-active-box rule with replacement, revocation, and what a refused box reports.

### Modified Capabilities

- `tenant-isolation`: a sync request is accepted only when its node key belongs to the node it names; a valid key presented with another node's id is rejected.

## Impact

- **Backend (cloud)**: `nodes` table gains key hash, key prefix, enrolled, revoked and last-seen columns and a one-active-edge-node-per-organization constraint; `NodeAuthMiddleware`, `NodeService` and the node repository; new enroll handler and route; `NODE_SYNC_KEY` no longer read by the cloud binary.
- **Backend (edge)**: still sends `X-Node-Key` and `X-Node-ID`; sync clients and the status tracker learn the "replaced" refusal.
- **Frontend**: the edge status surface shows the replaced-box message.
- **API docs**: new `backend/docs/api/edge-enrollment.md`; `sync.md` and `telemetry.md` updated for per-box keys.
- **Edge appliance**: `env/edge-node.env.example`, `env/alloy.env.example` and `install.ps1` wording (the key is per box; the two values still match each other).
- **Operations**: after the cloud release, Laguna's box must be enrolled once (sign in, call the endpoint, paste the returned values) before it syncs again. aws-infra sets `PUBLIC_API_URL` before the cloud release (the cloud refuses to boot without it) and drops `NODE_SYNC_KEY`.
