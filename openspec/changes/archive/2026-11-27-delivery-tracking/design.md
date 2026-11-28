## Context

See `proposal.md` for the why, and `specs/` for the behavior. The current state that shapes the approach:

- **An order has no stored state after payment.** Paying closes the `open_bill`: it sets `deleted_at` and `paid_at` (`OpenBillRepository.ClosePaid`). The header status is derived from the lines (`deriveStatusFromProducts`). Paid and voided orders are told apart by the predicates from `kitchen-lines-outlive-payment` (`kitchenLoadableOpenBill`).
- **Kitchen completion is `CompleteOpenBillProduct`.** It completes the line and calls `aggregate.TryComplete()`, which reports whether every line is now finished. It then saves through `persistAndSyncStatus` in one `unitOfWork.Do`. `UncompleteOpenBillProduct` can reopen a line, so a "ready" order can go back to preparing.
- **Takeaway and delivery orders taken by the agent live in the cloud.** The cloud runs their kitchen board. Edge orders reach the cloud as `dine_in`. The sync applier's on-conflict update lists the columns it refreshes explicitly, so new columns on `open_bills` are never overwritten by a push.
- **Cloud-only routes are registered in `bootstrap/routes_cloud.go`.** A route that is not registered answers `404` on the box.
- **Marketing events are recorded through `marketingEvents.RecordOrder` inside the transaction, into an outbox.** Customer-facing order messages use a `*.notification` outbox type and the notifications envelope (`order.expired` is the closest model: it is a notification only).
- **Roles are fixed ids**, inserted by shared migrations that must match `permissions.Role*` (`000063` added `agent` = 6). Users are pulled to the edge, so a role row must exist on both sides. The set of cloud roles is duplicated: `permissions.CloudRoles` in the backend and `CLOUD_ROLE_NAMES` in the frontend.
- **`/deliveries` already exists.** Its tabs are gated by permission (`DeliveriesPageClient`), and it reports `404` as "solo en la nube".
- **Browser printing exists** (`ReceiptPrintContent`, `browserPrint`). There is no QR library in the frontend.

## Goals / Non-Goals

**Goals:**
- Cheap dashboard queries filtered by current status, together with an append-only history.
- Two couriers claiming the same order: exactly one wins.
- `order.ready` sent at most once per order, even when the kitchen undoes a line.
- No change to the order, payment, kitchen or sync contracts.

**Non-Goals:**
- No edge schema or sync changes beyond the shared role row.
- The customer agent's MCP tools do not expose delivery status. The customer hears about it through marketing-app.
- No push or SSE stream for the dashboard (see D6).
- No reassigning a courier while an order is on the road, other than fail and re-dispatch.

## Decisions

### D1 — Current status on `open_bills`, history in its own table

A cloud migration adds these columns to `open_bills`: `delivery_status` (nullable; `out_for_delivery` | `delivered` | `failed`), `delivery_status_at`, `courier_user_id` (FK users, nullable), `courier_name` (nullable), `delivery_failure_reason` (nullable) and `ready_at` (nullable). A new table, `order_delivery_events`, holds `id`, `organization_id`, `open_bill_id`, `status`, `courier_user_id`, `courier_name`, `reason`, `note`, `actor_user_id` and `occurred_at`. It is tenant-scoped like the other tenant tables, append-only, and written in the same transaction as the column update.

`preparing` and `ready` are never stored. A read derives them from the lines when `delivery_status` is null, as the spec says.

*Alternative: history only, with the current status taken as the latest row.* Rejected, because every dashboard read would need a lateral join per order, and the status filter would become a subquery. The two can't drift apart, since a single repository method writes both.

*Alternative: a new aggregate table instead of columns on `open_bills`.* Rejected, because the dashboard always reads the order's contact, location and lines from `open_bills` anyway.

### D2 — Transitions as a small domain policy

A pure `delivery` policy in the domain takes the order type, paid/voided state, the derived status, the current courier and the actor (`manage`, or `carry` with the actor's user id). It returns either the next state or one of three errors: `INVALID_DELIVERY_TRANSITION` (409, carrying the current status), forbidden (403), or a validation error. All the rules in the spec's transition and "who may" tables live there, and they are unit-tested as a table. The service loads, asks the policy, then persists.

The transition check runs before the permission check. A courier asking for an impossible transition on their own order therefore gets `409`. A courier touching someone else's order gets `403`.

### D3 — Claims are a conditional update

Dispatch, deliver and fail update `open_bills` with `WHERE id = ? AND delivery_status IS NOT DISTINCT FROM ? AND courier_user_id IS NOT DISTINCT FROM ?` (the values the policy saw). If zero rows change, the order was changed in the meantime: the service reloads it and answers `409 INVALID_DELIVERY_TRANSITION` with the fresh status. When two couriers press "Lo llevo" together, one gets the order and the other gets a 409, which the courier page shows as "Otro repartidor ya lo tomó".

*Alternative: `SELECT ... FOR UPDATE`.* That also works, but the conditional update needs no extra round trip, and it fits the unit of work as it is.

### D4 — `ready_at` and `order.ready` from the kitchen path

In `CompleteOpenBillProduct` and `CancelOpenBillProduct`, when `TryComplete` reports the order is finished, the order is paid, and `ready_at` is null, `persistAndSyncStatus` also sets `ready_at = now`. For takeaway it records `order.ready` through `recordOrderEvent`, in the same transaction. The `ready_at IS NULL` guard (again a conditional update) makes it fire once: an undo followed by a re-complete does not notify again. `ready_at` gives the dashboard "time in ready" and the history its "became ready" instant. A dispatch from `preparing` leaves it null.

*Alternative: derive "ready" only when reading, and emit the event from a poller.* Rejected, because the event would no longer be in the same transaction as the change, which the marketing-events spec requires.

### D5 — Permissions

There are three new permissions:
- `deliveries:read` (the dashboard list and history): admin, manager;
- `deliveries:manage` (any transition, the list of couriers to assign): admin, manager;
- `deliveries:carry` (the courier lists, claim, own deliver and fail): courier.

The transition endpoints accept `manage` or `carry`. Without either they answer 403, and the policy narrows what `carry` may do.

The courier role is id 7, inserted by a **shared** migration so pulled users keep a valid role on the box. It is added to `CloudRoles` and to `CLOUD_ROLE_NAMES`. On the box it holds no edge permissions.

### D6 — The dashboard polls

The "Pedidos" tab and the courier page refetch every 10 s while visible, and right after any action. Volumes are tens of orders a day, so polling meets "without a reload" with no new stream.

*Alternative: an SSE stream like the kitchen's.* Deferred. The SSE hub is per kitchen area and needs heartbeat care behind the proxy. Polling can be replaced by it later without changing a spec.

### D7 — Endpoints (cloud only, `routes_cloud.go`)

```
GET  /api/deliveries/orders?status=&order_type=   deliveries:read
GET  /api/deliveries/orders/:id/history           deliveries:read
GET  /api/deliveries/couriers                     deliveries:manage   (active courier users)
GET  /api/deliveries/mine                         deliveries:carry    -> {claimable, on_the_road}
POST /api/deliveries/orders/:id/dispatch          manage|carry        {courier_user_id} | {courier_name}
POST /api/deliveries/orders/:id/deliver           manage|carry
POST /api/deliveries/orders/:id/fail              manage|carry        {reason, note?}
```

A courier's dispatch may omit `courier_user_id`, which then means themselves. If they name anyone else, it's a 403. The order and history read models reuse the existing contact, location and map-link fields. The contract goes in `backend/docs/api/deliveries.md`.

### D8 — Notifications follow the `order.expired` pattern

There are four new `MarketingEventType` notification values. A builder puts `order_id` and `order_type` into the notification `data`, plus `courier_name` (the user's display name, or the third-party name) or `reason`. The note is not a parameter of the builder at all, so it can't leak by mistake. All four are recorded through `recordOrderEvent`, which already skips orders without a channel.

### D9 — Frontend

- **"Pedidos" tab** (first in `/deliveries`, gated by `deliveries:read`): columns Preparando, Listo, En camino, Fallido and Entregado hoy. Each card shows the time in its status and its actions (Despachar, Entregado / Recogido, No se pudo entregar, Reenviar, Historial, Imprimir). The dispatch dialog offers a courier select or a third-party name. The fail dialog offers a reason select and a note.
- **`/my-deliveries` ("Mis entregas")**: a mobile-first courier page with "Para llevar" (claimable) and "En camino" (mine), map and `tel:` links, and the deliver and fail actions. It has a home tile gated by `deliveries:carry`.
- **Bag slip**: a new print component on the existing `browserPrint` path, with the QR code drawn client-side by the `qrcode` package. The map link must not be sent to a third-party QR service, because it is a customer's address.
- **User form**: "Repartidor" label, and the email-required helper names it.
- **Proxy routes** under `app/api/deliveries/...`, following the existing pattern.

## Risks / Trade-offs

- [marketing-app answers `400` to unknown types → events are rejected, not retried, and logged as errors] → Check marketing-app's behaviour first. Ship its handling (or a 2xx for unknown types) before this goes to the cloud. Rollout step 1.
- [The kitchen never completes lines, so orders stay "preparing"] → Managers can dispatch from `preparing` (spec). The dashboard shows time in status.
- [A takeaway is never picked up and stays on the dashboard] → The manager marks it delivered or voids it. Voided orders leave the list.
- [A courier user is deactivated while carrying orders] → Managers can still deliver or fail those orders. History keeps the user id.
- [Courier pages expose customer phones and addresses to couriers] → Only for claimable and their own orders, and with no amounts (spec).
- [Polling every 10 s on several screens] → A small indexed query: index `open_bills (organization_id, delivery_status)`, and filter on `paid_at` for the active set.
- [Courier users are pulled to the edge box] → No edge permissions, so a sign-in there shows nothing. Accepted.

## Migration Plan

1. Confirm or ship marketing-app's handling of `order.ready`, `order.out_for_delivery`, `order.delivered` and `order.delivery_failed`.
2. Deploy the cloud with the new migrations (next free versions):
   - shared: the courier role;
   - cloud: the `open_bills` columns and index, and `order_delivery_events`. The same migration
     marks every takeaway and delivery order paid before it as `delivered` at its `paid_at`, with
     no history entry, so the dashboard doesn't open with every past order shown as ready.
   - edge: twins of the courier role and the `open_bills` columns (`shared/` is frozen at v71, and
     the parity test requires the same `open_bills` schema on both sides). The box never writes them.
   All of them are additive.
3. The frontend ships in the same release. The edge box gets the shared role migration at its next normal release.
4. Create courier users (with email) in Usuarios.

Rollback: revert the release. The down migrations drop the table and columns, and delete role 7 only if no user holds it. Orders keep working, since nothing outside this change reads the new columns.

## Open Questions

- Slip format: 80 mm thermal roll (like the receipt) or A6 label. This only affects the print CSS.
- Poll interval: 10 s is a starting point, to tune after use.
