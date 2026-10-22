## Why

Paying an order closes it, and closing it takes its lines off the kitchen. `PayOrder` soft-deletes the open bill, and every kitchen read and every line transition filters on `open_bills.deleted_at IS NULL`: the pending board (`FindPendingByArea`), the "Comandas Listas" review (`FindCompletedByAreaBetween`), and the load and save behind complete / in-progress / uncomplete / cancel (`FindAggregateByID`, `UpdateProductStatus`). A table that pays before its food is out loses those lines from the board. They are never marked completed, so the restaurant loses their prep times and the dish never appears as served. The data behind kitchen analytics is wrong for every order paid early.

Paid and voided are also indistinguishable today: both just set `deleted_at`. So there is no way to keep "paid, keep cooking" while still dropping "voided, throw it away".

This also blocks the planned delivery flow (`agent-delivery-orders`), where paying first is the rule: food is prepared only after the money is in.

## What Changes

- **Paid is recorded on the order.** `open_bills` gains `paid_at`, set by `pay_order` in the same transaction that closes the order. A closed order with `paid_at` is paid; a closed order without it is voided. Orders closed before this change stay as they are (no backfill): their lines do not reappear.
- **Kitchen lines live on after payment.** A line of a paid order keeps its own state and stays on the kitchen board until it is completed or cancelled. The board, the completed review, and the line transitions treat a paid order like an open one. Voided orders behave as today.
- **The order itself stays closed.** A paid order does not come back to the active order list, cannot be edited, re-paid or voided, and still appears in "Órdenes cerradas hoy". Only its lines' kitchen state moves.
- **No line cancellation after payment.** Cancelling a line of a paid order is refused: the money is already taken and a refund is a staff decision outside the system. Complete, in-progress and uncomplete stay allowed.
- **Lines record when they started and finished.** `open_bills_products` gains `started_at` and `completed_at`, set by the line transitions. Today completion time is read from `updated_at`, which any later write overwrites. The completed review and the SSE payload switch to `completed_at`.
- **Replication keeps paid orders closed.** Today an edge line change on a paid order would push a snapshot whose upsert clears `deleted_at` in the cloud (`open_bill_sync_applier.go:81-86`), bringing the paid order back as active. The sync payload gains the order's `closed_at` and `paid_at` and each line's `started_at` / `completed_at`, and the applier writes them instead of clearing. **Cloud first:** an old cloud receiving a new edge's line update on a paid order would still resurrect it.

Out of scope: holding unpaid takeaway or delivery lines out of the kitchen (that rule needs the order type and ships in `agent-delivery-orders`, which depends on this change); backfilling `paid_at` or line timestamps for history; any analytics endpoint or dashboard on top of the new timestamps; changing how `pay_order` builds the bill or the invoice.

## Capabilities

### New Capabilities
- `kitchen-line-lifecycle`: Paid vs voided orders, kitchen lines that continue after payment, the transitions allowed on a paid order's lines, line start/finish timestamps, and replication that preserves all of it.

### Modified Capabilities
- (none — the project has no durable specs yet; `openspec list --specs` returns nothing.)

## Impact

- **Database:** one migration: `open_bills.paid_at`, `open_bills_products.started_at`, `open_bills_products.completed_at` (all nullable).
- **Domain:** the open-bill aggregate learns whether it is paid; a line refuses cancellation when its order is paid; line transitions stamp `started_at` / `completed_at`; `OrderService.PayOrder` marks the order paid; the line-transition methods load paid orders.
- **Repository:** `open_bill_repository.go`: a paid-aware close in place of `Delete` for the pay path, `FindAggregateByID` and `UpdateProductStatus` accept paid orders, `FindPendingByArea` and `FindCompletedByAreaBetween` include paid orders' lines and read `completed_at`.
- **Sync:** `dto.OpenBillSyncPayload` / `OpenBillSyncProduct` gain the closure and timestamp fields; `open_bill_sync_applier.go` writes them. Old payloads without the fields keep today's behavior.
- **HTTP:** none of the routes change. Line-state endpoints now succeed on paid orders, and line cancel on a paid order returns a 409-class error. The SSE `completed_at` value becomes the real completion time. `docs/api/orders.md` documents both.
- **Rollout:** deploy the cloud before the edge.
- **Frontend:** the kitchen board needs no change to keep showing paid orders' lines. The waiter's view may want to show "paid" on a line whose order has left the active list; optional.
