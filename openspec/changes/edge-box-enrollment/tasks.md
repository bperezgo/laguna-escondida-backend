## 1. Node storage (backend, cloud)

- [x] 1.1 Add cloud migration 000094 giving `nodes` the columns `key_hash`, `key_prefix`, `enrolled_at`, `revoked_at`, `last_seen_at` and the partial unique index of design decision 3, with its down migration; verify the migration acceptance tests pass and that inserting a second non-revoked edge node for one organization fails
- [x] 1.2 Extend the node port and repository: load a node for auth with its organization's status, find and lock an organization's active edge node, insert, set key, revoke, and the throttled `last_seen_at` update of design decision 5; regenerate mocks; verify with repository tests against the test database

## 2. Node authentication (backend, cloud)

- [x] 2.1 Add the node errors (invalid key, revoked, organization suspended, already enrolled) and `NodeService.Authenticate(nodeID, key)` with the check order of design decision 2; verify with unit tests for: own key, wrong key, unknown node, another node's key, node without a key, revoked node with right and wrong key, suspended organization
- [x] 2.2 Rewrite `NodeAuthMiddleware` to use `Authenticate`: one `401` body for the three plain cases, `401` + `code: node_revoked`, `403` + `code: organization_suspended`, and the `last_seen_at` update on accepted requests only; verify `node_auth_middleware_test.go` covers every scenario of spec requirements "The cloud accepts a box only with its own key" and "Revoked boxes and suspended organizations are refused with a reason"
- [x] 2.3 Move `NODE_SYNC_KEY` out of the shared config into the edge config and drop the parameter from `registerCloudRoutes`; verify the config tests show the cloud boots without it and the edge still refuses to boot without it

## 3. Enrollment (backend, cloud)

- [x] 3.1 Add the `edge:enroll` permission, granted to the admin role only; verify the role-permission tests show admin has it and manager does not
- [x] 3.2 Add the enrollment DTOs, the `lgn_` key generator and hash (same scheme as `api_key_service.go`), and the enroll service method implementing the four paths of design decision 4 in one transaction; verify with unit tests for first box, `409` with `active_box`, replace, replace with nothing active, and adoption of a node without a key keeping its id
- [x] 3.3 Add the enroll handler and `POST /api/edge/enroll` on the cloud routes behind `JWTAuthMiddleware` and `RequirePermission(EdgeEnroll)`; verify handler tests for `201`, `400` on empty name, `401` without a token, `403` for a manager, and that the edge router answers `404`
- [x] 3.4 Add acceptance tests under `backend/test/acceptance/sync/`: enroll then pull with the returned key; two concurrent enrollments give one `201` and one `409`; replace makes the old key answer `node_revoked`; organization A's key with organization B's node id answers `401`; verify with `DB_HOST= make test-acceptance` from `backend/`

## 4. Refusal reason on the box (backend, edge)

- [x] 4.1 Make the three sync clients return a typed error when the cloud answers `401`/`node_revoked` or `403`/`organization_suspended`; verify with client unit tests against an `httptest` server for both codes and for a plain `401`
- [x] 4.2 Give `syncstatus.Tracker` a blocked reason set by the scheduler from that error and cleared by any success, and add `sync_blocked` to `EdgeStatusResponse` (`null` in the cloud); verify tracker and `edge_handler` tests cover replaced, suspended, cleared after success, and plain network failure staying `null`

## 5. Frontend

- [x] 5.1 Add `sync_blocked` to `frontend/types/edge.ts` and pass it through `lib/api/edge.ts` and `app/api/edge/status/route.ts`; verify `pnpm lint` and `pnpm test` pass from `frontend/`
- [x] 5.2 Show admins and managers a message where the offline state is shown: replaced → "Esta caja fue reemplazada por otra. Ya no sincroniza con la nube."; suspended → "La cuenta está suspendida. Esta caja no sincroniza con la nube."; verify with a test of the logic that picks the message (both reasons, `null`, and a server role seeing nothing), and by loading the POS against a status stub for each reason

## 6. Contract and appliance docs

- [x] 6.1 Write `backend/docs/api/edge-enrollment.md` (request, `201`, `400`, `401`, `403`, `409` with `active_box`) and update `sync.md` and `telemetry.md` for per-box keys, the two `code` values and the removal of the shared key from the cloud's settings; document `sync_blocked` where `/api/edge/status` is described; verify every status code in the specs appears in the docs
- [x] 6.2 Update the wording in `edge/env/edge-node.env.example`, `edge/env/alloy.env.example` and `edge/scripts/install.ps1` messages: the key is this box's own, issued at enrollment; verify `edge-ci.yml` still passes

## 7. Checks

- [x] 7.1 Run `make lint` and `make test` from `backend/`, and `pnpm lint` and `pnpm test` from `frontend/`; verify all pass
- [x] 7.2 Run `openspec validate edge-box-enrollment --strict`; verify it reports the change as valid

## 8. Sync address in the enrollment response (added after sections 1 to 7 were built)

- [x] 8.1 Add the required cloud setting `PUBLIC_API_URL` to `LoadCloud` with the validation of design decision 4b (absolute `http`/`https`, host, no path, query or fragment; trailing slash trimmed); verify config tests for: valid value, trailing slash trimmed, unset, no scheme, a path, and that the edge loads with it unset or invalid
- [x] 8.2 Add `cloud_sync_url` to the enrollment response, filled from the setting passed to the enroll service at construction; verify the service and handler tests assert the field on all three success paths (first box, replace, adoption)
- [x] 8.3 Set the value in every test harness and fixture that builds a cloud config, and in `backend/.env.example`; verify `go build ./...` and `make test` pass from `backend/`
- [x] 8.4 Extend the enrollment acceptance test: the box syncs using the returned `cloud_sync_url` unaltered; verify with `DB_HOST= make test-acceptance` from `backend/`
- [x] 8.5 Document the field and the setting in `backend/docs/api/edge-enrollment.md` and add the `curl` example to `backend/docs/examples/curl-examples.md` if it is not there; verify the documented `201` body matches the handler test's
- [x] 8.6 Run `make lint` from `backend/` and `openspec validate edge-box-enrollment --strict`; verify both pass

## 9. Rollout (owner, after merge)

- [ ] 9.1 Ask aws-infra to set `PUBLIC_API_URL` to the cloud's public domain before the release; verify the value is a domain you control, not a load balancer hostname
- [ ] 9.2 Release the cloud (`cloud/vX`); verify the migration ran and Laguna's box now gets `401` on sync
- [ ] 9.3 Sign in as Laguna's admin and call `POST /api/edge/enroll`; verify the returned `node_id` equals the box's current `NODE_ID` and `cloud_sync_url` equals its current `CLOUD_SYNC_URL`
- [ ] 9.4 Put the new key in `env\edge-node.env` (`NODE_SYNC_KEY`) and `env\alloy.env` (`EDGE_NODE_KEY`) on the box and re-run `install.ps1`; verify `/api/edge/status` reports `online: true` and the queued outbox drains
- [ ] 9.5 Ask aws-infra to remove `NODE_SYNC_KEY` from the cloud's configuration; verify the cloud still boots and the box still syncs
- [ ] 9.6 Cut the next `edge/v*` so the box reports `sync_blocked`; verify the box updates and its status includes the field
