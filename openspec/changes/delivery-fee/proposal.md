## Why

A delivery costs the restaurant money, and today nothing charges for it: the order total, the
Wompi link and the invoice hold only the products. The customer, whether talking to the agent
or ordering through staff, must hear the delivery cost before committing, pay it with the
order, and get it on a valid electronic invoice.

## What Changes

- **A delivery zone has a flat fee.** Admins and managers set it as a final price with the ICO
  included (like a product's `total_price_with_taxes`); the base and the ICO amount are derived
  and stored with it, as for products. No fee, or `0`, means free delivery. Existing zones start
  at `0`, so nothing changes until a price is set.
- **The point check returns the fee**, so the agent and staff can quote it before the order is
  created.
- **A delivery order records its fee at creation** from the zone it is accepted in. Redrawing or
  repricing the zone, or replacing the order's lines, never changes it. The order total, and so
  the Wompi payment link and the paid amount, include it. Order reads return it apart from the
  lines.
- **The fee is invoiced as its own "Domicilio" line taxed with ICO 8%**, following DIAN Oficio
  664 (904106) of 2022: a restaurant's own delivery charge is part of the restaurant service
  (art. 512-9 E.T.). The tip stays outside the base. A free delivery adds no line. The fee is
  never a product line: it does not reach the kitchen boards or stock.
- **Reports show delivery revenue on its own**: the daily close and the financial summary
  report the delivery fees (gross, base and ICO) besides the sales they already show, without
  changing how the existing totals reconcile.
- **The agent tells the customer.** `check_delivery_location` returns the zone's fee and its
  description tells the agent to say it before taking the order; the order the agent creates or
  reads shows the fee and the total that includes it.
- **Staff UI**: the zone form gets a fee field; creating a delivery order shows the zone's fee
  once the pin is checked, and the order detail shows it.

## Capabilities

### New Capabilities
- `delivery-fee-invoicing`: how a recorded delivery fee is carried to the bill, the electronic
  invoice (its own ICO line) and the reports.

### Modified Capabilities
- `delivery-zones`: a zone has a fee, managed with the zone and returned by the point check.
- `delivery-orders`: a delivery order records its fee, which is part of its total and payment
  and stays fixed.
- `customer-agent-access`: `check_delivery_location` and the order views return the fee.

## Impact

- Backend: migrations (fee columns on `delivery_zones`, the order and the bill); the zone
  aggregate and service; order creation and total; bill creation at payment; invoice submission
  (an extra item); financial summary and daily close; MCP customer tools.
- Frontend: zone admin form, delivery order creation and order detail, daily close and financial
  screens. Same PR as the backend.
- Docs: `backend/docs/api/delivery-zones.md`, `orders.md`, `financial.md`, `invoices.md`.
- Not affected: dine-in and takeaway orders, the kitchen boards, stock, and the edge box (it
  takes no deliveries).
- Out of scope: fees billed by an outside courier, distance-based fees, free delivery above an
  order amount.
- Before go-live: the accountant confirms the ICO treatment against the current Estatuto.
