## Context

See proposal.md for why. Requirements are in `specs/edge-sync-refusal-backoff/spec.md`.

Today:
- **Cloud.** `NodeAuthMiddleware` → `NodeService.Authenticate` makes one indexed `FindForAuth` read and a SHA-256 compare on every sync and telemetry request. It refuses with `401 invalid node credentials`, `401 node_revoked` or `403 organization_suspended`.
- **Edge.** `EdgeSyncScheduler` runs push and pull on `* * * * *` and the stock pull daily (`0 5 * * *`), all through gocron.
  - `httpclient/sync_refusal.go` maps `node_revoked` and `organization_suspended` to domain errors, and `recordRefusal` turns those into `syncstatus.Tracker.RecordBlocked`.
  - A plain `401 invalid node credentials` is a generic failure, so the POS shows offline.
- **Telemetry.** Grafana Alloy sends it straight to `/api/telemetry/v1/*` (`edge/alloy/config.alloy`). The Go edge doesn't see those calls.
- **Existing cache pattern.** The cloud already decorates a repository with `ports.KeyValueCache` (`platform/cache/organization_settings_repository.go`, backed by `platform/cache/memory`).

## Goals / Non-Goals

**Goals:**
- The cloud's cost for a refused box is constant and tiny.
- The box's call rate while refused is set by the cloud.
- The paid base's calls are spread over time.

**Non-Goals:**
- Changing Alloy's configuration. It is steered only by the cloud's answers (decision 6).
- Caching accepted nodes. Their path stays one indexed read, which is negligible at today's scale, and a suspension or revocation keeps taking effect on the next request.
- Sharing the refusal cache across replicas. Each replica keeps its own (decision 1). A shared cache such as ElastiCache Valkey is a separate infra change, best made together with sharing the per-address rate limits once the cloud runs more than one replica.
- New fields on the box's local `GET /api/edge/status`. The POS keeps reading `sync_blocked` as today, so its screen needs no change.

## Decisions

### 1. Remember refusals in NodeService behind a port, not in the middleware

`NodeService.Authenticate` gets a `ports.KeyValueCache` (a dedicated, bounded in-memory instance).

- **Lookup.** Before `FindForAuth`, it looks up `node-auth:<nodeID>`. On a hit, it evaluates the cached `NodeAuthRecord` exactly as it evaluates a fresh one: key compare, revoked, status. So the opaque-reason rule holds, and a wrong key still gets `ErrNodeKeyInvalid`.
- **Store.** After a fresh evaluation that ends in a refusal, it stores the record for 5 minutes. A not-found node is stored as a tombstone.
- **Accepted nodes are never stored**, so a paying box's path is exactly today's.
- **Accept from a cached record.** If a cached record evaluates to accept (cached after a wrong-key request, then the real box calls), it's accepted and dropped from the cache, so the next call takes the normal path, including `TouchLastSeen`.

The verdict lives in the service, so caching there avoids a decorator that would have to re-derive it.

*Alternatives:*
- Cache in the middleware: puts business rules in the handler layer.
- Cache every `FindForAuth` result: cuts paid load too, but makes `LastSeenAt` stale and changes the accepted path. Not needed now.

### 2. Bound the cache by refusing new entries when full

`memory.New` gains an optional maximum entry count (default unlimited, so existing users are unchanged). The node-auth instance uses 50,000. When it's full, `Set` stores nothing, so overflow requests fall through to the database. The existing `Sweep` removes expired entries.

Dropping instead of evicting needs no LRU dependency. Overflow means someone is flooding random ids, which the load balancer limit is for.

### 3. `Retry-After` set in one place

`NodeAuthMiddleware(nodes, retryAfter time.Duration, refusalStatus int)` sets `Retry-After: <seconds>` on its three refusal branches and nowhere else.
- `refusalStatus` is `0` for the sync routes, which keep today's `401`/`403`.
- It is `429` for the telemetry routes (decision 6).

The body is the same either way. A new cloud setting `NODE_REFUSAL_RETRY_AFTER` (Go duration) defaults to `6h`. It's validated as at least `1m` and at most `24h`, which is the edge's clamp, so a configured value always means what it says.

### 4. A refusal carries its delay to the scheduler as a typed error

- **Error type.** A new domain error type, `NodeRefusalError{Reason error, RetryAfter time.Duration}`, wraps `ErrNodeKeyInvalid`, `ErrNodeRevoked` or `ErrNodeOrganizationSuspended`. It implements `Unwrap`, so the existing `errors.Is` checks keep working.
- **Producing it.** `syncStatusError` builds it, now also for `401 invalid node credentials`, reading `Retry-After` as seconds. A missing or unreadable header gives `0`, and the scheduler applies the 6h default.
- **Path.** It travels through `SyncPushService`, `SyncPullService` and `StockPullService` unchanged, since they already wrap with `%w`.

Domain error types are importable by both `httpclient` and `cron` without breaking the import rule.

### 5. The pause lives on the existing tracker

- **New methods.** `syncstatus.Tracker` gains `Pause(until time.Time)` and `PausedUntil() time.Time`. `RecordSuccess` clears the pause.
- **Scheduler.** `recordRefusal` uses `errors.As` for `NodeRefusalError`, clamps the delay to [1m, 24h] (default 6h), records the blocked reason as today and calls `Pause`. An invalid-key refusal pauses but doesn't set a blocked reason, so the POS keeps showing offline as today.
- **Every job.** Each of the three jobs checks `PausedUntil()` first and skips while paused.
- **After the pause.** The first job to run makes the single call, then succeeds or pauses again. Push and pull both run every minute, so "one call" means the first of them, and the second sees the new pause before calling.

One shared clock for all three loops, already injected into the scheduler.

### 6. Telemetry: answer refused telemetry with `429` so Alloy waits

Alloy's `otelcol.exporter.otlphttp` treats `401`/`403` as permanent: it drops the batch and sends the next one, a few calls a minute. It treats `429` as retryable and uses `Retry-After` as the wait. The edge config already has what this needs:
- `num_consumers = 1`, so a waiting batch holds the whole queue.
- `max_elapsed_time = 24h`, above the 6h wait.
- A capped disk buffer, so new telemetry is discarded once it's full.

So the telemetry routes get the middleware with `refusalStatus = 429`, and nothing changes on the box.

*Verified by spike (2026-11-25), decision confirmed:* the exporter waits the full `Retry-After`, beyond `max_interval` (5m).
- Setup: `grafana/alloy:v1.19.2` (the `edge/versions.json` pin, Linux image of the release the box runs on Windows) with `config.alloy` unchanged, env overrides only. A stub answered every request `429` + `Retry-After: 600`. Fed metrics (scraped every 15s) and OTLP logs (every 5s) for 24 minutes.
- Result: requests per signal at 15:51:00 → 16:01:00 → 16:11:00 (metrics) and 15:51:40 → 16:01:40 → 16:11:40 (logs). Every gap was 600.0s, none early, no backoff retries in between. Alloy logged `Throttle (10m0s) … interval=10m0s` on each retry.
- Buffer: 544 KB on disk throughout, far under the 32 MiB cap. No queue-full or retries-exhausted errors.
- Notes: each signal has its own queue, so a refused box makes up to 3 telemetry calls (logs, metrics, traces) per `Retry-After` window. An Alloy restart forgets the wait: its first batch is sent at once, refused, and waits again.

The edge-node relay fallback isn't needed.

*Alternative:* stopping the Alloy service from the edge-node. Rejected: it needs service-control rights the edge-node shouldn't have.

### 7. Per-box offset: a hash of the node id, slept before the call

- **Offset.** `fnv32(nodeID)` gives a stable value: modulo 45s for the minute loops, modulo 15m for the stock pull.
- **When it runs.** Each job waits its offset before the pause check and the call. The wait is a timer selected against a stop channel closed by `Stop()`, so shutdown never waits on it.
- **No overlap.** Jobs register with `gocron.WithSingletonMode(gocron.LimitModeReschedule)`, so an offset run can't overlap the next tick.
- **Input.** `cfg.NodeID` is passed to `NewEdgeSyncScheduler`.

*Alternative:* rewriting cron expressions with a seconds field only works for minute-level expressions. The daily one would need minute arithmetic on arbitrary user cron strings.

## Risks / Trade-offs

- **A customer who pays waits up to 6h for the box to resume.** Mitigation: restarting the edge service clears the pause (spec), so support can tell them to restart. The operator can also lower `NODE_REFUSAL_RETRY_AFTER`, which affects boxes from their next refusal.
- **Multiple cloud replicas each make one database read per refused node per 5 minutes.** That's negligible.
- **Alloy may not honor a long `Retry-After`.** Mitigation: the spike runs first, before the rest is built, and the edge-node relay is the fallback. Even if both fail, every refused call is still answered from memory.
- **A refused box's telemetry sits in its buffer until it's discarded.** That's accepted: an expired organization's telemetry has no value.
- **Boxes older than this release don't pause.** Mitigation: raise `MIN_EDGE_VERSION` to this release before signup opens. Only Laguna's box exists today.
- **A full cache sends overflow to the database.** Mitigation: the per-address rate limit on the load balancer (operator, aws-infra).

## Migration Plan

1. Ship the cloud part (`cloud/v*`). Refusals gain `Retry-After`, and current boxes ignore it, which is harmless.
2. Ship the edge part (`edge/v*`) and update Laguna's box.
3. Before `self-serve-signup-trial` opens signup, raise `MIN_EDGE_VERSION` to that edge release.

Rollback: each part is independent. Reverting either one restores today's behavior with no data to undo.

## Implementation map

- **Cloud**:
  - `internal/domain/service/node_service.go` (+ `node_service_test.go`)
  - `internal/domain/error/node_error.go`
  - `internal/platform/cache/memory/cache.go` (+ test)
  - `internal/platform/handler/middleware.go` (+ test)
  - `internal/platform/config/cloud.go` (+ test)
  - `internal/bootstrap/cloud.go`, `routes_cloud.go` (sync routes get the default middleware, telemetry routes the `429` one)
- **Edge**:
  - `internal/platform/httpclient/sync_refusal.go` (+ test)
  - `internal/platform/syncstatus/tracker.go` (+ test)
  - `internal/platform/cron/edge_sync_scheduler.go` (+ test)
  - `internal/bootstrap/edge.go`
- **Spike**: a throwaway stub plus `edge/alloy/config.alloy` (read only). Record the outcome in decision 6.
- **Docs**: `docs/api/sync.md`, `docs/api/edge-enrollment.md` (refusal responses), and the `docs/api/telemetry.md` for the `429`.

## Open Questions

None.
