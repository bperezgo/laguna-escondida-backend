## Why

After a takeaway or delivery order is paid, nothing records what happens to it: no one can see
which orders are ready, on the road, or with whom, and the agent can tell the customer only that it
was paid, never that it is on its way, delivered, or that delivery failed.

## What Changes

- Takeaway and delivery orders get a **delivery status** after payment: `preparing` and `ready`
  (derived from the kitchen lines), then the stored `out_for_delivery`, `delivered` and `failed`.
  Takeaway goes `ready -> delivered` (shown as "Recogido"). Dine-in orders are not tracked.
- Every change is kept in a **delivery history** (status, who, when, courier, reason, note).
- **Assignment**: an order on the road belongs to a courier. A courier claims a ready delivery
  order, or a manager assigns it to a courier user or to a third party by name.
- **Failed delivery** requires a reason (`customer_unreachable`, `wrong_address`, `refused`,
  `other`) and takes an optional internal note. A manager can send a failed order out again.
- New **courier role** ("Repartidor"). It signs in to the cloud, claims orders and marks its own
  delivered or failed, nothing else. Admins and managers can make every transition (third parties).
- **Dashboard** in `/deliveries` (new "Pedidos" tab, cloud only): takeaway and delivery orders by
  status, with courier and time in each status, updated without a reload, and actions to assign,
  mark and re-dispatch.
- **Courier page**: mobile-first list of the courier's orders on the road in dispatch order, with
  address, map link, call button, "Entregado" and "No se pudo entregar".
- **Bag slip**: a browser-printed label per order (number, name, phone, address, map QR, lines).
- **marketing-app notifications** for agent orders: `order.ready` (takeaway),
  `order.out_for_delivery`, `order.delivered` (with `order_type`) and `order.delivery_failed` (with
  the reason, never the note), on `/backend/notifications`.

## Capabilities

### New Capabilities
- `delivery-tracking`: delivery statuses and transitions, who may make each, assignment, failure
  reasons, history, the dashboard and courier reads, and the bag slip.

### Modified Capabilities
- `marketing-events`: four new order notifications for delivery progress.
- `organization-sign-in`: the courier role is a cloud role (needs an email, may sign in to the
  cloud).

## Impact

- **Ordering**: builds on `wompi-payment-links` (the order-notification envelope) and is archived
  after it.
- Backend: cloud migrations (delivery status and history, courier role, permissions); the order
  service and a new delivery-status endpoint set; kitchen line completion records `order.ready`;
  marketing-event recorder; role permissions and `CloudRoles`.
- Frontend: "Pedidos" tab in `/deliveries`, courier page, bag-slip print, user admin offers the
  new role. Same PR as the backend.
- Docs: a new `backend/docs/api/deliveries.md`, `marketing-events.md`, `users.md`.
- marketing-app (other repo): handle the four new notification types and their messages. Until it
  does, they are delivered and ignored.
- Not affected: dine-in orders, payment, the kitchen boards, and the edge box, which serves none
  of this (till takeaway orders reach the cloud as `dine_in` and are not tracked, accepted).
- Out of scope: route optimization, refunds after a failed delivery.
