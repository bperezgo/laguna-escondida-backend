## 1. Spike: does Alloy wait out a `429` + `Retry-After`? (gates section 4)

- [x] 1.1 Run the Alloy version pinned for the edge with `edge/alloy/config.alloy` (unchanged), pointing `EDGE_TELEMETRY_INGEST_URL` at a throwaway stub that answers every request `429` with `Retry-After: 600` and logs each arrival time. Feed it telemetry for at least 25 minutes, and record in design decision 6 the version, the measured gaps between requests, and whether the buffer stayed within its cap. Verify: the gaps are about 600s, not 5s–5m backoff, and no request arrives before 600s has passed.
- [x] 1.2 If 1.1 shows Alloy does not wait the full delay, stop. Record the result and the fallback (Alloy → edge-node relay) in design decision 6, and update the spec and these tasks before continuing. Verify: design.md shows the decision either confirmed or replaced.

## 2. Bounded refusal cache (backend, cloud)

- [x] 2.1 Give `platform/cache/memory` an optional maximum entry count. When the cache is full, `Set` stores nothing. Existing callers stay unlimited. Verify: memory cache tests cover the unlimited default, a full cache dropping new keys, an existing key still updatable when full, and space freed by `Sweep` after expiry.
- [x] 2.2 Add `NodeRefusalError{Reason, RetryAfter}` to `domain/error/node_error.go`, unwrapping to its reason. Verify: a test shows `errors.Is` matches `ErrNodeKeyInvalid`, `ErrNodeRevoked` and `ErrNodeOrganizationSuspended` through it, and `errors.As` recovers the delay.
- [x] 2.3 Inject a `ports.KeyValueCache` into `NodeService` and implement design decision 1. On a hit, evaluate the cached record or tombstone with today's checks. Store refusals and not-found for 5 minutes. Never store an accepted node, and drop a cached record that evaluates to accept. Run `make generate-mocks` if a port changed. Verify with `node_service_test.go` cases:
  - a suspended node's second call makes no `FindForAuth`
  - a wrong key against a cached record is still `ErrNodeKeyInvalid`
  - an unknown node is remembered
  - an accepted node is never stored, and `TouchLastSeen` still runs
  - a cached record whose real key then arrives is accepted and evicted
  - a cache error falls back to the database
- [x] 2.4 Wire a dedicated memory cache (50,000 entries, swept) into `NodeService` in `internal/bootstrap/cloud.go`. Verify: `go build ./...` passes and the bootstrap tests pass.

## 3. `Retry-After` and the telemetry `429` (backend, cloud)

- [x] 3.1 Add `NODE_REFUSAL_RETRY_AFTER` to `platform/config/cloud.go`: a Go duration, default `6h`, rejected outside [1m, 24h]. Verify: config tests cover the default, a valid value, `30s` and `48h` rejected, and garbage rejected.
- [x] 3.2 Give `NodeAuthMiddleware` the retry delay and a refusal status (design decision 3). Set `Retry-After` on every refusal branch, and answer `429` with today's body when the refusal status is set. Verify: middleware tests show each refusal with the header on sync (`401`/`401`/`403`) and on telemetry (`429` for all three), the bodies unchanged, and no header on an accepted request.
- [x] 3.3 In `routes_cloud.go`, use the default middleware for `/api/sync/*` and the `429` one for `/api/telemetry/*`. Verify: route tests for a suspended node on `GET /api/sync/pull` (`403` + header) and `POST /api/telemetry/v1/logs` (`429` + header).
- [x] 3.4 Add an acceptance test under `backend/test/acceptance/sync/`. Suspend an enrolled box's organization and call pull twice: both get `403` with `Retry-After`. Reactivate it and wait past the cache window (shortened through the constructor): pull is accepted. Verify with `DB_HOST= make test-acceptance` from `backend/`.

## 4. Pause and spread on the box (backend, edge)

- [x] 4.1 Make `httpclient/sync_refusal.go` return `NodeRefusalError` for `401 invalid node credentials`, `401 node_revoked` and `403 organization_suspended`, reading `Retry-After` in seconds (`0` when missing or unreadable). Verify: client tests against an `httptest` server cover all three, a missing header, an unreadable header, and `500` staying a plain error.
- [x] 4.2 Give `syncstatus.Tracker` `Pause(until)` and `PausedUntil()`, with `RecordSuccess` clearing the pause. Verify: tracker tests cover pause, success clearing it, and `Blocked()` unchanged by a pause alone.
- [x] 4.3 In `EdgeSyncScheduler`, implement design decision 5. `recordRefusal` uses `errors.As`, clamps the delay to [1m, 24h] with a 6h default, keeps today's blocked reasons (none for an invalid key) and pauses. Each job skips while paused. Network errors and `5xx` don't pause. Verify with scheduler tests, using an injected clock:
  - a refusal pauses push, pull and stock pull
  - `999999` is clamped to 24h and a missing delay becomes 6h
  - one call is made after the pause, and a success resumes
  - a network failure does not pause
- [x] 4.4 Implement design decision 7. Pass `cfg.NodeID` to `NewEdgeSyncScheduler` and compute the offset from `fnv32(nodeID)`: under 45s for the minute loops, under 15m for the stock pull. Wait it out on a timer that `Stop()` interrupts, and register the jobs in singleton reschedule mode. Verify:
  - the offset is stable for one node id and within bounds
  - 1,000 random node ids spread over the range, with no second holding more than 5% of them
  - `Stop()` returns promptly during a wait

## 5. Docs, checks and release

- [x] 5.1 Document `Retry-After` on the node refusals in `docs/api/sync.md` and `docs/api/edge-enrollment.md`, the `429` refusals in `docs/api/telemetry.md`, and `NODE_REFUSAL_RETRY_AFTER` wherever cloud settings are listed. Add a `curl -i` refusal example to `docs/examples/curl-examples.md`. Verify: each refusal in the docs shows its status, body and header.
- [x] 5.2 From `backend/`, run `make lint`, `go build ./...` and `make test`, and fix findings. Verify: all three pass.
- [x] 5.3 Note in the release and runbook docs that `MIN_EDGE_VERSION` must be raised to this change's `edge/v*` release before `self-serve-signup-trial` opens signup. Verify: the note is present where the edge release steps are described.
