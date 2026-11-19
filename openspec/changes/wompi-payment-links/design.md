## Context

The customer toolset creates takeaway and delivery orders with an origin (`channel_id`, set by
marketing-app). Those orders are held out of the kitchen until `PayOrder` runs, which today only
staff do (the `agent` role cannot pay). `PayOrder` writes the bill, queues the e-invoice for any
non-cash payment, records `order.paid` in the marketing outbox and releases the lines to the
kitchen.

marketing-app's `payment-link-orders` change has already fixed the contract this backend must
meet: its `whatsapp-agent/internal/orderscontract` scenarios and `evals/ordersfake` describe the
tool shapes, and its `POST /backend/notifications` already accepts `order.paid` and
`order.expired`.

## Goals / Non-Goals

**Goals:**
- An agent order is paid only from Wompi's verified webhook or by staff, never by the agent.
- One link per order version; a stale link never pays the current order.
- An order nobody pays is gone after 30 minutes, with its stock back.

**Non-Goals:**
- Refunds, partial payments, tips, other providers.
- Links for staff-created or dine-in orders, or on the edge box.
- Configuring Wompi from the ops UI (environment only, for now).

## Decisions

### D1. "Agent order" means an order the customer toolset created

An order created by an agent-only caller (the customer toolset's API key), which can only create
takeaway and delivery. Not the origin channel: marketing-app's contract client sends no
`X-Channel-Id`, and the payment rule must not depend on attribution. Creation is keyed on the
caller; change, void and expiry are keyed on the order having a link. Marketing events still need
a channel, as today. Staff-created takeaway orders get no link and are paid at the till as today.
No new flag on the order.

### D2. The link is created after the order commits

A Wompi call inside the order transaction would hold the transaction open on a network call and
leave a link for an order that rolled back. So: commit the order, then create the link and store
it in `payment_links`. If Wompi fails, the order is returned without `payment_url` and the error
is logged; marketing-app's agent escalates it to staff. Such an order never expires (D5), so
staff can still pay or void it.

The link: `single_use: true`, `collect_shipping: false`, `currency: COP`,
`amount_in_cents` = total × 100, `expires_at` = order creation + 30 minutes (UTC), `sku` = the
order id, and a `name`/`description` naming the restaurant and the order's short identifier.
Wompi refusing payment after `expires_at` removes most of the expiry race on its side.
`payment_url` is `https://checkout.wompi.co/l/<link id>`.

### D3. A change deactivates the link and creates another

`update_my_order` commits the new lines, deactivates the current link (`active: false`), and
creates a new one for the new total, with the same `expires_at`: a change does not extend the
30 minutes. `payment_links` keeps every link with `status` `active | replaced | expired`;
exactly one is `active` per order. Cancelling (`cancel_my_order`, staff void) deactivates the
current link too.

### D4. The webhook pays only the current link, for its exact amount, once

`POST /api/webhooks/wompi/:organization_id`, public, cloud only. Each organization's Wompi account
is configured in Wompi's dashboard with its own URL, so the path names the organization and its
events secret.

1. Verify the checksum: SHA-256 of the values named by `signature.properties`, then
   `timestamp`, then the events secret; compare with `signature.checksum`. A mismatch is `401`
   and logged. Wompi retries non-2xx three times in 24 hours.
2. Only `transaction.updated` is handled; any other event is answered `200` and ignored.
3. Insert the transaction into `payment_link_transactions` keyed by Wompi's transaction id. A
   transaction already seen with the same status is answered `200` and nothing else happens.
4. Find the link by the transaction's payment link id. `APPROVED` pays the order through
   `PayOrder` in the organization's context (`OrganizationRunner.Run`) only when the link is the
   order's `active` one, the order is unpaid and not voided, and `amount_in_cents` and `currency`
   match the link. Otherwise the transaction is stored as `unmatched` and logged as an error:
   money was taken that no order received, and staff refund it in Wompi (out of scope).
5. `PENDING` is stored, so the expiry job waits for it. `DECLINED`, `VOIDED` and `ERROR` are
   stored and change nothing; the link stays usable until it expires.

Payment code for the bill and e-invoice by `payment_method_type`: `CARD` → `credit_card`, `PSE`
→ `transfer_debit_bank`, `NEQUI`, `BANCOLOMBIA_TRANSFER`, `DAVIPLATA` and anything else →
`transfer_debit_interbank`. No fiscal customer: consumidor final.

### D5. Expiry is a cloud job, a void with a reason

Every 15 seconds (marketing-app's contract waits TTL + 30 s for the expiry), per organization
(`OrganizationRunner.ForEach`): agent orders unpaid, not voided, with an `active` link whose
`expires_at` has passed. For each: if a `PENDING` transaction exists for any of its links, skip it
until that transaction resolves (approved: it is paid; otherwise the next run voids it). Else
deactivate the link at Wompi and void the order exactly as `DeleteOrder` does (soft
delete, stock returned, `order.cancelled` recorded) with `void_reason = payment_expired`, record
`order.expired`, and mark the link `expired`. An order without a link (D2) is never expired.

`void_reason` (`payment_expired`, null otherwise) is a new nullable column on the order, so
`get_my_orders` can list expired orders of the business day as `payment_state: expired` while
other voided orders stay hidden.

### D6. Two more notification types on the existing outbox

The outbox row has an internal type and a path. Two new internal types,
`order.paid.notification` and `order.expired.notification`, send the wire types `order.paid` and
`order.expired` to `/backend/notifications`, in the notifications envelope, with
`data: {order_id, amount, currency}` for a payment and `{order_id}` for an expiry. `order.paid` is
recorded for every payment of an agent order, at the till too, since the customer is told either
way. Recorded in the same transaction as the payment or the void, as every event.

### D7. Configuration from the environment, as the marketing events secret

`WOMPI_API_URL` (`https://sandbox.wompi.co/v1` or `https://production.wompi.co/v1`) is shared.
Per organization, by slug: `WOMPI_PRIVATE_KEY_<SLUG>` and `WOMPI_EVENTS_SECRET_<SLUG>`. An
organization has payment links configured when both are set. `PAYMENT_LINK_TTL` (default `30m`)
is how long a link lives, shortened only for tests. Startup logs which active
organizations have them. Moving these to encrypted per-organization settings, as e-invoicing
credentials will be, is later work.

### D8. The refusal is checked before the order is built

Customer order creation for an organization without payment links is refused with `422`,
`PAYMENT_LINKS_NOT_CONFIGURED`, after validation and the ordering-availability check, before the
zone check. The MCP turns it into the tool error
`the order was not created: payment links are not configured for this organization`, which
marketing-app's contract matches on `payment links are not configured`.

## Risks / Trade-offs

- [A customer pays a link just before a change replaces it] → stored `unmatched`, logged, refunded
  by staff. Rare: a change needs the customer's own confirmation in the chat.
- [Wompi approves after the order expired] → `expires_at` on the link makes Wompi refuse new
  payments; a payment already pending is waited for (D5). Anything left is `unmatched`.
- [Webhook lost for 24 hours] → the order expires and the money is `unmatched`. Mitigation later:
  reconcile by reading the transaction from Wompi before voiding.
- [Wompi down at creation] → order without link, escalated by the agent; never auto-expired.

## Migration Plan

1. Deploy with no `WOMPI_*` keys: every organization refuses agent orders. Laguna has no orders
   agent enabled yet, so nothing changes for customers.
2. marketing-app deploys `payment-link-orders` (harmless before events arrive).
3. Set Laguna's sandbox keys and webhook URL on a local backend run with a short
   `PAYMENT_LINK_TTL`, and run marketing-app's L2 contract test with `ORDERS_PAYMENT_TTL` set to
   the same value and `ORDERS_MCP_TOKEN_NO_PAYMENTS` (a key of an organization without Wompi).
4. Production keys; marketing-app enables Laguna's orders agent.

Rollback: unset the keys; agent orders are refused again, staff orders untouched.

## Open Questions

- Wompi's API for deactivating a link (`PATCH /v1/payment_links/:id` with `active: false`) and the
  transaction field naming its link (`payment_link_id`): confirm in sandbox before task 3.
  From the docs (2026-11-18), pending the sandbox run in 5.2: the transaction object carries
  `payment_link_id`; the checksum is as D4 (`signature.properties` values, `timestamp`, secret),
  also sent as `X-Event-Checksum`; retries at 30 min, 3 h, 24 h. The deactivation call is not in
  Wompi's Colombia docs; the `@pulgueta/wompi` SDK sends `PATCH /payment_links/:id {active}`
  with the private key, which D3 assumes.
- Whether `CARD` can be split into credit and debit from `payment_method.extra` for the invoice.
