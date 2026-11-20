## ADDED Requirements

### Requirement: Delivery progress is notified

For an agent order, the system SHALL record a notification on `/backend/notifications`:
- `order.ready` when a `takeaway` order becomes `ready`;
- `order.out_for_delivery` when an order is set `out_for_delivery`, including on a re-dispatch;
- `order.delivered` when an order is set `delivered`;
- `order.delivery_failed` when an order is set `failed`.

Each SHALL be recorded in the same transaction as the change it reports, and SHALL follow the same recording, signing and delivery rules as every other event. A `delivery` order becoming `ready` SHALL NOT be notified. Orders without a `channel_id` SHALL record nothing.

#### Scenario: A WhatsApp delivery goes out

- **WHEN** courier Juan claims a ready agent delivery order
- **THEN** one `order.out_for_delivery` notification is recorded

#### Scenario: A takeaway is ready

- **WHEN** the last pending line of a paid agent takeaway order is completed
- **THEN** one `order.ready` notification is recorded

#### Scenario: The change rolls back

- **WHEN** setting an agent order `delivered` fails after the history entry was written, and the change rolls back
- **THEN** no `order.delivered` notification exists

#### Scenario: An order taken at the till

- **WHEN** a staff-created delivery order with no channel is delivered
- **THEN** no notification is recorded

### Requirement: Delivery notifications carry what the agent needs to tell the customer

Delivery notifications SHALL use the notifications envelope `{event_id, type, occurred_at, channel_id, conversation_id, customer_phone, data}`, where `data` SHALL hold `order_id` and `order_type`, and also:
- for `order.out_for_delivery`: `courier_name`, the courier user's display name or the third-party name;
- for `order.delivery_failed`: `reason`, one of `customer_unreachable`, `wrong_address`, `refused` or `other`.

The internal failure note SHALL NOT be sent.

#### Scenario: A picked-up takeaway

- **WHEN** an agent takeaway order is set `delivered`
- **THEN** `/backend/notifications` receives `type` `order.delivered` whose `data` has the order id and `order_type` `takeaway`

#### Scenario: A failed delivery

- **WHEN** an agent delivery order is set `failed` with reason `wrong_address` and a note
- **THEN** `/backend/notifications` receives `type` `order.delivery_failed` whose `data` has `reason` `wrong_address` and no note
