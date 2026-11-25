## Why

A WhatsApp order waits for staff to check a bank transfer and record the payment before the kitchen
sees it. That is slow, and with paid ads bringing new customers it lets someone order and disappear
while the order is held for them. The customer should pay a Wompi link, the order should be paid
the moment Wompi confirms it, and an order nobody pays should not linger. marketing-app's change
`payment-link-orders` relays the link and messages the customer; it depends on this one.

## What Changes

- **Every agent order gets a Wompi payment link.** A takeaway or delivery order created through
  the customer toolset gets a single-use link for the backend's total, valid 30 minutes.
  `create_delivery_order` and `update_my_order` return it as `payment_url`. A change to the
  order deactivates the link and creates another.
- **Wompi's signed webhook marks the order paid.** `transaction.updated` with `APPROVED`, for the
  order's current link and its exact amount, pays the order as `PayOrder` does today: released to
  the kitchen, billed, invoiced, `order.paid` recorded. Once per transaction. A payment that does
  not match is recorded and logged, never applied.
- **An unpaid agent order expires after 30 minutes.** A cloud job deactivates its link and voids it
  (stock returned), unless a payment is pending at Wompi, which it waits for.
- **`get_my_orders` reports expired orders** of the business day as `payment_state: expired`, and
  the current `payment_url` of an order awaiting payment.
- **Two notifications for marketing-app:** `order.paid` and `order.expired` on
  `/backend/notifications`, with `order_id` (and `amount`, `currency` for a payment).
  `order.paid` and `order.cancelled` on `/backend/events` are unchanged.
- **No payment links, no agent order.** An organization without Wompi configured has customer
  order creation refused with `PAYMENT_LINKS_NOT_CONFIGURED`. Staff-created orders and dine-in
  are untouched.
- Wompi keys per organization from the environment, by slug, as the marketing events secret.

## Capabilities

### New Capabilities

- `payment-links`: Wompi configuration per organization, the link lifecycle, and the webhook
  that turns an approved transaction into a payment.

### Modified Capabilities

- `delivery-orders`: agent orders need payment links, carry one, and expire unpaid after 30
  minutes.
- `customer-agent-access`: `payment_url` on create and update, the expired state and current
  link in `get_my_orders`, the refusal without payment links.
- `marketing-events`: `order.paid` and `order.expired` notifications to the customer.

## Impact

- `internal/domain/service/order_service.go` (create, update, pay, expire), a new
  `payment_link_service.go`, a Wompi adapter under `internal/platform/`, a webhook handler.
- Cloud migration: `payment_links` and `payment_link_transactions`, and a void reason on orders.
- `internal/platform/mcpserver/tools_customer.go`: `payment_url`, `expired`, the refusal message.
- `internal/domain/aggregate/marketing_event/`: the two notification bodies.
- `internal/platform/config/`: `WOMPI_*` settings. Docs: `docs/api/orders.md`,
  `docs/api/marketing-events.md`, a new `docs/api/wompi-webhook.md`.
- Cloud only; the edge box neither creates links nor receives the webhook.
- Contract: marketing-app's `orderscontract` scenarios (payment link on create, a new one after
  update, expiry, refusal) must pass against this backend.
