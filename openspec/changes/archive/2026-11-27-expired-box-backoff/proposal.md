## Why

Once anyone can start a 14-day trial (`self-serve-signup-trial`), every trial that does not convert leaves a box in the field that the cloud refuses. Today a refused box keeps calling the cloud every minute, forever. Every box also fires on second :00, because the sync crons are `* * * * *`. Unpaid boxes must never be able to degrade the cloud for the customers who pay, and this must ship in the edge release before trials open, so every trial box already behaves.

## What Changes

- **Cloud: refusals cost almost nothing.** When a node is refused (organization suspended or trial expired, node revoked), the cloud remembers that answer in memory for a few minutes. Repeat calls from that node are answered from memory without reading the database.
- **Cloud: refusals say when to come back.** Every node refusal carries a `Retry-After` header. The default is 6 hours and can be changed in the cloud's configuration without an edge release.
- **Edge: a refused box pauses.** When the cloud refuses the box, the box stops its sync loops (push, pull, stock pull) for the `Retry-After` time, then tries once. Telemetry is sent by Alloy: the cloud answers its refused calls with `429` + `Retry-After`, so Alloy waits too, verified first by a spike. Without a `Retry-After`, it falls back to the same 6-hour default. The pause lives in memory only: after a restart the box makes one call, is refused, and pauses again.
- **Edge: boxes don't all call at once.** Each box adds a random offset to its sync schedule, so the paid base does not land on the cloud at the same second.
- The POS keeps showing the existing blocked status. No new screen, button or message.

Not in this change: a way for the customer to retry early, keeping the pause across restarts, blocking offline selling on an expired box, retiring nodes after a long expiry, and the per-address rate limit on the load balancer (aws-infra, done by the operator).

## Capabilities

### New Capabilities

- `edge-sync-refusal-backoff`: what a node refusal costs the cloud and tells the box, how long a refused box stays quiet, and how boxes spread their sync calls.

### Modified Capabilities

None. `edge-box-enrollment` still lives in its in-flight change; this change builds on its refusals without changing them.

## Impact

- **Backend (cloud)**: `NodeAuthMiddleware` / `NodeService.Authenticate` gain a short-lived in-memory record of refused nodes. Refusal responses gain `Retry-After`. There's one new optional setting for the retry delay. No database change.
- **Backend (edge)**: `httpclient/sync_refusal.go` reads `Retry-After`. `cron/edge_sync_scheduler.go` skips its runs while paused and adds the per-box offset. Alloy's config is unchanged.
- **API docs**: the node refusal responses document `Retry-After`.
- **Release**: needs an `edge/v*` release. `MIN_EDGE_VERSION` should be raised to it before trials open, so no trial box runs an older edge.
- **Depends on**: `edge-box-enrollment` (the refusals this change builds on).
