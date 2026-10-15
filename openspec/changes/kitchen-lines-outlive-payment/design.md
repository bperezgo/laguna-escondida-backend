## Context

See proposal.md (Why). The current state that shapes the approach:

- **"Closed" is `open_bills.deleted_at`.** `PayOrder` and `DeleteOrder` both call `openBillRepo.Delete`, which only sets it. Around ten reads use `deleted_at IS NULL` to mean "active" (the active list, `FindByID`, `FindByIDWithProducts`, the closed-today list and more).
- **Paying does not soft-delete the lines.** Only the header is closed (`open_bill_repository.go:1005-1008` relies on this). A paid order's lines are still `deleted_at IS NULL` rows.
- **Line transitions** (`CompleteOpenBillProduct`, `UncompleteOpenBillProduct`, `SetOpenBillProductInProgress`, `CancelOpenBillProduct`) load through `FindAggregateByID` (header `deleted_at IS NULL`) and save through `UpdateProductStatus`. The header update there filters on `deleted_at IS NULL`, so it would match zero rows for a closed order without erroring; the line updates filter on the line's own `deleted_at`. `UpdateOrder` also calls `FindAggregateByID`, after a strict `FindByIDWithProducts`.
- **The kitchen reads** join `open_bills ... AND open_bills.deleted_at IS NULL` and report completion as the line's `updated_at` (`FindPendingByArea`, `FindCompletedByAreaBetween`).
- **The sync applier** upserts the header with `deleted_at` in its update list and the model's zero value, so every applied snapshot clears `deleted_at` (`open_bill_sync_applier.go:81-86`). A paid order's line transition on the edge appends an `update` snapshot after the pay's `delete` tombstone, and applying it in order reopens the order in the cloud.

## Goals / Non-Goals

**Goals:**
- One definition of "visible to the kitchen", used by every kitchen read and the transition load.
- Every existing meaning of `deleted_at IS NULL` ("active", "editable", "payable") stays exactly as it is.
- Edge-only deployment order does not matter for correctness of new data, as long as the cloud goes first.

**Non-Goals:**
- Redefining "closed". `deleted_at` keeps meaning closed; `paid_at` only says why.
- Changing the derived header status (`deriveStatusFromProducts`). On a closed order it stays a derived value nobody acts on.

## Decisions

### D1 — Add `paid_at`; keep `deleted_at` as "closed"

`PayOrder` calls a new repository method `ClosePaid(ctx, id, paidAt)` that sets `deleted_at` and `paid_at` together. `DeleteOrder` keeps calling `Delete`.

**Alternative considered — stop soft-deleting on pay and add a `closed_at`.** That would rewrite the meaning of every `deleted_at IS NULL` read: the active list, edit, pay, the closed-today list, and sync tombstones. Each would need re-auditing. Keeping `deleted_at` and adding one column changes only the kitchen paths, which are the ones that need to change.

**No backfill.** Historic paid orders keep `paid_at IS NULL` and stay invisible to the kitchen. Backfilling them would put long-finished lines from weeks ago back on today's board with state `created`.

### D2 — A dedicated kitchen load, not a looser `FindAggregateByID`

A new port method `FindAggregateForKitchen(ctx, id)` loads the aggregate when `deleted_at IS NULL OR paid_at IS NOT NULL`, and sets `paidAt` on the aggregate. The four line-transition methods switch to it. `FindAggregateByID` stays strict, so `UpdateOrder` and anything added later cannot accidentally act on a paid order.

The aggregate gains `paidAt *time.Time` (via `NewAggregateFromRepository`) and `IsPaid()`. `CancelProduct` returns `ErrCannotCancelPaidOrderLine` when paid, which the handler maps to 409. The other transitions don't look at it.

**Why the rule lives in the aggregate:** "no cancellation after payment" is a business rule. Putting it in the handler or the query would let the next caller skip it.

### D3 — One SQL predicate for "kitchen-visible"

```
open_bills.deleted_at IS NULL OR open_bills.paid_at IS NOT NULL
```

It lives as one repository helper (a GORM scope or a const fragment) used by `FindPendingByArea`, `FindCompletedByAreaBetween` and `FindAggregateForKitchen`. In `UpdateProductStatus`, the header update keeps its `deleted_at IS NULL` filter: on a paid order it updates nothing, which is correct because the header is closed. The line updates already match.

`agent-delivery-orders` will extend the same helper with its hold rule, so there is one place to read what the kitchen sees.

### D4 — Line timestamps set by the aggregate, not the repository

`OpenBillProduct` gains `startedAt` and `completedAt`. Transitions set them using the rules in the spec: `SetInProgress` sets the start if it is empty, `Complete` sets the completion, and `Uncomplete` clears the completion, plus the start when it comes from `in_progress`. `UpdateProductStatus` writes both columns. `UpdateOrder`'s line upsert does not write them, so a later edit never moves them.

`FindPendingByArea` and `FindCompletedByAreaBetween` read `completed_at` instead of `updated_at`. The completed review filters its `[from, to)` window on `completed_at`. Historic completed lines have `completed_at IS NULL` and fall back to `updated_at` with `COALESCE`, so "Comandas Listas" for days before the deploy still shows them.

### D5 — The sync payload carries closure and timing; the applier assigns instead of clearing

`OpenBillSyncPayload` gains `ClosedAt *time.Time` and `PaidAt *time.Time` (`omitempty`). `OpenBillSyncProduct` gains `StartedAt` and `CompletedAt`. `appendOpenBillOutbox` fills them from the aggregate.

The applier sets `deleted_at = payload.ClosedAt` and `paid_at = payload.PaidAt` in the upsert, and the line timestamps on lines. For an old payload these are nil, so `deleted_at` is cleared as today, which is the behavior old edges rely on to resurrect a re-created order. `paid_at` gets nil, which is correct for an old edge because it never pays-and-continues.

The pay's tombstone stays as it is. The cloud's `applyDelete` sets `deleted_at` but not `paid_at`, so the pay path must also send the paid instant. `PayOrder` writes an `update` snapshot carrying `ClosedAt` and `PaidAt` **instead of** the tombstone. The cloud then knows the order is paid, not voided, which its own kitchen views need once deliveries are prepared there.

**Alternative considered — keep the tombstone and make the applier ignore updates to closed orders.** That would also block legitimate re-creates and would never transmit `paid_at`. Rejected.

**Rollout constraint:** an old cloud applier ignores the new fields and clears `deleted_at`, reopening paid orders. So the cloud deploys first. An old edge talking to a new cloud is safe, because it sends no new fields and gets today's behavior.

## Risks / Trade-offs

- **[Cloud deployed after the edge]** → Because pay now sends an `update` snapshot instead of a tombstone (D5), an old cloud would reopen **every** order paid on a new edge, not just those with later line activity. Mitigation: the migration plan orders cloud first, and task 5.3 checks it before the edge goes out. Orders already reopened are repaired by re-sending their snapshot from the edge.
- **[Lines of paid orders stay on the board forever if nobody finishes them]** → Same as an open order today. A cook clears them by completing or cancelling them, and cancelling is not allowed on paid orders. If this happens in practice, a staff "close kitchen line" action is the follow-up; not added preemptively.
- **[Frontend screens assume "not in active list" means "done"]** → The waiter's view may lose track of a paid table whose food is still cooking. Mitigation: no API breaks; the kitchen board is the source of truth for food state. A "paid" badge is optional frontend work.
- **[`COALESCE(completed_at, updated_at)` mixes accurate and approximate times]** → Only for lines completed before the deploy. Analytics should filter on `completed_at IS NOT NULL` for accurate prep times.

## Migration Plan

1. Migration `000061_paid_orders_and_line_timing`: add nullable `open_bills.paid_at`, `open_bills_products.started_at`, `open_bills_products.completed_at`. Down drops them.
2. Deploy the cloud.
3. Deploy the edge during a quiet period. From then on, payments set `paid_at` and the kitchen keeps paid lines.
4. Rollback: redeploying the old edge restores today's behavior; paid lines leave the board again on the next load. Leave the columns in place and only run the down migration if the change is abandoned.
