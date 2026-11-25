## Why

An agent order can be paid only through a Wompi link, which costs 2.65% + $700 + IVA: about 5%
of a typical delivery, and a link is an unfamiliar way to pay in towns like Marinilla. Restaurants
should choose: staff confirm a transfer at no fee, or Wompi confirms a QR (1%), with the link as
a fallback.

## What Changes

- **Each organization chooses a payment strategy for agent orders**, in Ajustes: `manual` or
  `wompi`. It applies to new orders; an order keeps the strategy it was created under.
- **Manual strategy.** The organization writes its transfer instructions in Ajustes and the agent
  order returns them in place of a link. Staff pay the order as at the till, which releases it
  to the kitchen and notifies the customer as today.
- **Wompi strategy pays by QR first.** The order gets a Wompi `BANCOLOMBIA_QR` transaction and
  returns the QR as a PNG image URL, with the link as the fallback. An approved transaction pays
  the order whichever it came from.
- **A QR payment is matched by the order's reference**, since it has no link; the other checks
  stay. A second payment for a paid order is stored as unmatched, as today.
- **Unpaid manual orders expire too**, after a window set in Ajustes (default 60 minutes), unless
  staff have marked the order as under review.
- **Wompi is no longer needed to take agent orders.** An organization on `manual` takes them
  without Wompi. One on `wompi` without keys is still refused, as today. An organization that
  never chose is `wompi`, so nothing changes for existing ones.
- **The QR needs two more Wompi keys** (public key, integrity secret), optional in Ajustes;
  without them the organization keeps the link only. The customer accepts Wompi's terms by
  confirming an order summary that shows them.
- **Agent tools report how to pay** on create, update and status: the strategy, and the
  instructions or the QR image URL and link.

The first task verifies in Wompi's sandbox that `transaction.updated` fires for the QR; if it
does not, the Wompi strategy stays link-only.

Not in scope: Bre-B through another provider, staff-created and dine-in orders.

## Capabilities

### New Capabilities

- `payment-strategies`: the per-organization choice between manual and Wompi, the manual
  instructions and expiry window, and manual confirmation by staff.

### Modified Capabilities

- `payment-links`: a QR transaction per order version beside the link, matched by reference.
  (Introduced by `wompi-payment-links`, not yet archived.)
- `organization-integrations`: the Wompi integration's optional QR keys. (Introduced by
  `organization-integrations`, not yet archived.)
- `delivery-orders`: only `wompi` orders need payment links; manual orders expire on their own
  window.
- `customer-agent-access`: payment instructions, QR image URL and strategy on the order tools.

## Impact

- Backend: the order, payment link, webhook and expiry services, the Wompi client, integration
  settings, the customer MCP tools, a cloud migration, a public QR image endpoint, API docs.
- Frontend: Ajustes, and a staff action to mark an order under review.
- Other repo: marketing-app's `payment-qr-and-manual-instructions` sends the QR image and the
  instructions to the customer; its `orderscontract` scenarios change with this one.
