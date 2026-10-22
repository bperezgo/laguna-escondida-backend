## Context

See `proposal.md` (Why) for motivation and `specs/` for the behavior contract. This section covers only the mechanics that make the inversion small.

### Stock is already represented twice on the wire

Every stock write — a sale, a purchase, a manual adjustment, a batch count — commits three things in one transaction: the new amount, a `historic_stock` movement row, and a `sync_outbox` snapshot op. The helpers are `createAndSyncHistoric` and `appendStockOutbox` (`stock_service.go:301-340`, `268-299`), and both are called from `StockService` and `StockEventHandler`.

```
  EDGE, one transaction per movement          CLOUD, one transaction per op
  +-------------------------------+           +--------------------------------+
  | stock.amount +/- change       |           |                                |
  | historic_stock row (DELTA) ---------------> historic_stock row inserted    |
  |   op_id = outbox op_id        | seq = N   |   amount NOT touched            |
  | sync_outbox (ABSOLUTE) -------------------> stock.amount = snapshot        |
  |   full stock snapshot         | seq = N+1 |   last snapshot wins            |
  +-------------------------------+           +--------------------------------+
```

The cloud receives both. It inserts the delta as accounting and assigns the amount from the absolute (`stock_sync_applier.go:56-63`). That assignment is the entirety of "the edge owns stock". This change deletes the second arrow and makes the first one write the amount.

### Why the fold is exactly-once for free

`SyncService.ApplyPush` opens one `UnitOfWork.Do` per op, calls `SyncInboxRepository.MarkApplied(op_id)`, and **returns early without dispatching when the op was already applied** (`sync_service.go:45-58`). Appliers join that ambient transaction through `postgres.GetTxOrDB`. So an applier that does `amount += change` runs at most once per `op_id`, atomically with the inbox row that proves it ran. No new machinery, no compensation logic, no `UnitOfWork` nesting problem.

### Two details that shape the fold

- **The grain is `product_id`.** `stock`'s primary key is `(product_id, version)`, but every read and write keys on `product_id` alone (`stock_repository.go:62-98`), so the fold targets the live row for a product and the `version` column stays the vestigial thing it already is.
- **The movement op always precedes its snapshot.** `createAndSyncHistoric` is called before `appendStockOutbox` in every write path, so the delta always carries the lower `seq` and the cloud applies it first. That ordering is what makes the migration safe (see Migration Plan, step 2).

### The pull channel

Reference data (products, users, suppliers, responsibilities, ingredients) flows cloud → edge as *state*, over a timestamp cursor the edge owns, with no acknowledgement (`sync_reference_service.go:27-47`). It uses none of `sync_outbox`/`sync_inbox`. Its known weakness — a commit-order race that can hide a row until its next update — is acceptable for a cache that is rewritten daily.

## Goals / Non-Goals

**Goals:**
- One writer of the absolute, and make it the node where stock is actually managed.
- Reuse the existing push and pull channels; add no transport.
- Leave every offline-critical edge path untouched: selling, decrementing, queueing.
- Keep on-hand rebuildable from its movements at all times.

**Non-Goals:**
- Freshness on the edge. The restaurant's numbers are a day old by design.
- Removing the `stock.version` column or reworking the composite key.
- Retention or pruning of `historic_stock`, which this change makes load-bearing.
- Any availability check. Nothing gates a sale today and nothing does after.

## Decisions

### D1 — The fold lives in the `historic_stock` applier, not in a separate projector

The applier that inserts a replicated movement also applies it: `amount += change`, creating the row at `change` when the product has none.

**Why over a projector that re-sums the ledger periodically:** a projector is O(history) per product and needs its own consistency story against in-flight batches. The applier is already inside the transaction that guarantees exactly-once, so the incremental fold is both cheaper and stronger. The re-sum still exists — as an audit (D6), not as the write path.

### D2 — The edge stops emitting the snapshot op, and the cloud stops applying it

`appendStockOutbox` goes away on the edge; `StockSyncApplier` stops assigning `amount`.

**Alternative considered — keep applying it into a separate `edge_reported_amount` column for drift detection.** Rejected: the daily refresh (D4) overwrites the edge's amount from the cloud, so the two numbers agree by construction for most of the day and the signal is thin. D6's re-sum detects real divergence better. Keeping an absolute on the wire also leaves a value that a future reader could be tempted to assign, which is exactly the failure this change removes.

### D3 — Authored writes move to the cloud; the box keeps only consequence writes

The dividing line is not "who touches stock" but **what the write is derived from**:

```
  consequence of a business event      authored by a person
  recorded where it happens            against a number they read
  ------------------------------       ---------------------------
  sale decrement          EDGE         create stock row      CLOUD
  purchase entry receipt  EDGE/CLOUD   adjust amount         CLOUD
                                       delete stock row      CLOUD
                                       batch count           CLOUD
```

Consequence writes are self-describing deltas and fold correctly from anywhere, which is why a delivery recorded at the restaurant stays valid. Authored writes need the number they were computed against to be the real one, so they belong where the truth is.

**Why not leave them all on the edge:** `POST /api/stock` sets an absolute outright, and a batch count computed against the edge's day-old cache produces a delta that lands on a base the cloud never had (the wrong-base problem that motivated this whole decision).

### D4 — `stock` becomes a pull entity on its own daily cron

The edge refreshes its amounts from the cloud once a day, on a cursor of its own, separate from the reference-pull cron.

**Why not add stock to the existing reference pull:** `stock.updated_at` changes on every sale, so the shared pull would carry a steady stream of rows over the link this architecture exists to tolerate, to refresh a cache nothing reads for decisions. Daily is both less machinery and less traffic.

**Why daily rather than hourly:** the numbers should be stable during service. A refresh mid-shift makes displayed on-hand jump when the office records something; running it at daily close or before opening keeps each day internally consistent.

**Why a separate cursor:** the reference pull's `last_pulled_cursor` advances on its own cadence; sharing it would couple the two jobs and make either one's failure hide the other's progress.

### D5 — Every movement is a delta; nothing on the ledger sets an absolute

A batch count is recorded as `change = counted - the cloud's current on-hand`.

**Why not record a count as "set to N":** `amount == SUM(change)` is the property that makes the ledger the truth and the amount a derivable view of it. A `set` row breaks the fold — the amount stops being rebuildable, and every reconciliation query becomes "sum the changes since the last set". The delta also commutes with sales replicated after the count, which a `set` would erase.

**What makes the delta correct here, when it would not have been on the edge:** the count is authored in the cloud (D3), so the basis it subtracts from *is* the number it is folded into.

**Residual exposure:** sales made at the restaurant but not yet replicated are missing from the cloud's number at count time, so they subtract twice — once implicitly (the goods were already off the shelf when counted) and once when they arrive. This is bounded by how stale the edge's push is, which the cloud can measure (`MAX(sync_inbox.applied_at)` for edge-origin ops) and surface next to the counting screen. Counts are rare and deliberate; a measured warning is the proportionate answer.

### D6 — The invariant is enforced by a reconciliation job, not a constraint

`amount == SUM(historic_stock.change)` cannot be a database constraint. A scheduled job re-sums each product's movements and reports divergence with both values.

**Why it matters more than usual here:** the fold is now on the critical path of every replicated sale. A bug in it corrupts amounts quietly. But the ledger — the input — is untouched by the bug, so any amount can be rebuilt; the job is what turns a silent corruption into a report.

### D7 — The cloud does not queue sync ops for its own stock writes

`createAndSyncHistoric` unconditionally appends an outbox op. On the cloud, nothing consumes cloud-origin stock ops (the edge learns through the pull), so they would accumulate undelivered exactly like the stranded `bill` CUFE/Tascode rows do today.

The emission becomes a constructor-injected collaborator that is real on the edge and a no-op on the cloud, rather than a mode flag read inside the service — the domain layer should not know which node it is running on. The local `historic_stock` row is still written on both nodes; only the outbox op is suppressed.

### D8 — The edge keeps writing its local movement row

Nothing on the edge reads movement history (spec: history is a cloud concern), but the local row remains the natural, transactional source of the op that gets pushed, and dropping it would mean surgery on every write path for no behavioral gain. Its rows become unread local residue; pruning is deferred.

## Risks / Trade-offs

- **The edge's numbers are stale within the day** → accepted by design; nothing gates on them, and the spec states the refresh contract so the frontend can label the value rather than present it as live.
- **A count taken while the edge is behind subtracts twice** → the cloud exposes how recently it last applied an edge movement; the counting screen warns. Bounded by how rarely counts happen.
- **A refresh that runs while movements are still queued** briefly shows a number that is too high (the queued sales are in the cloud's future) → self-heals at the next refresh, and the cloud's number is never wrong, only the display.
- **The fold is on the critical path of push apply** → D6's re-sum is the detector, and the ledger stays intact so any amount is rebuildable.
- **Losing the Windows box with queued movements** loses those sales' stock effect and leaves the cloud high → strictly better than today, where losing the box loses the truth itself. The cloud holds the orders those movements came from, so the gap is reconstructable; doing so automatically is out of scope.
- **`historic_stock` becomes load-bearing and unbounded** → pruning now requires a baseline/snapshot strategy rather than a plain delete. Named as future work, not solved here.
- **Rolling back after authoring has moved** means cloud-authored movements the edge never saw are lost when the edge's absolute reasserts itself → rollback is safe only before step 4 of the migration.

## Migration Plan

1. **Schema, inert.** Add `historic_stock.kind` with a default for existing rows. No behavior change on either node.
2. **Enable the fold while the assignment is still live.** Because a movement op always carries a lower `seq` than its own snapshot op, the cloud applies the fold and then overwrites it with the edge's absolute — so the cloud's numbers are byte-for-byte what they are today, while the fold runs against real traffic. Run the reconciliation job (D6) in report-only mode and confirm it finds nothing. This validates the riskiest part of the change in production at zero risk.
3. **Flip ownership.** Drain the edge's outbox completely, stop assigning from the snapshot, and write one `opening_balance` movement per product so the ledger sums to the adopted amount. The cloud's amounts do not move at cutover — the mirror already equals the edge's numbers by construction.
4. **Move authoring.** Register the four write endpoints in cloud mode, remove them from edge mode, and switch the web app's stock screens. After this step, rollback loses cloud-authored movements.
5. **Edge cleanup.** Ship the daily stock pull and stop emitting snapshot ops.

**Rollback:** before step 4, re-enable the snapshot applier — the edge's absolute reasserts itself per product on its next movement, and the ledger rows remain valid under either owner. After step 4, roll forward instead.

## Open Questions

- What time the daily refresh should run (daily close versus pre-opening). Operational; changes no spec and no task.
- Whether the batch screen's warning should be advisory or should block a count when the edge's last movement is older than some threshold. Frontend policy; the backend exposes the signal either way.
- Retention for `historic_stock` now that the amount depends on it.
