## ADDED Requirements

### Requirement: Agent orders are paid with a payment link

A `takeaway` or `delivery` order created by the customer agent (an agent order) SHALL be paid
through its payment link (see `payment-links`) or by staff. Staff-created and `dine_in` orders
SHALL NOT get a payment link and SHALL behave as before.

#### Scenario: A WhatsApp delivery order

- **WHEN** the customer agent creates a delivery order, with or without origin channel `laguna-escondida-whatsapp`
- **THEN** it has a payment link and stays held out of the kitchen until paid

#### Scenario: A takeaway order taken at the till

- **WHEN** staff create a takeaway order
- **THEN** it has no payment link, as before

### Requirement: Agent orders need payment links configured

Creating an agent order SHALL be refused with `422 Unprocessable Entity` and the code
`PAYMENT_LINKS_NOT_CONFIGURED` when the organization does not have payment links configured, and no
order SHALL be created. The check SHALL run after the request is validated and after the ordering
availability check, before the zone check. Staff-created orders SHALL NOT be affected.

#### Scenario: An organization without Wompi

- **WHEN** an agent takeaway order is created for an organization without payment links configured
- **THEN** the request fails with `422` and code `PAYMENT_LINKS_NOT_CONFIGURED`, and no order exists

#### Scenario: Staff in the same organization

- **WHEN** staff of that organization create a takeaway order
- **THEN** the order is created as before

### Requirement: An unpaid agent order expires

An agent order SHALL be voided when its active payment link has expired and it is still unpaid,
unless a pending transaction exists for one of its links, in which case it SHALL wait until that
transaction resolves. Expiring SHALL deactivate the link and void the order exactly as staff voiding
it does (soft-deleted, stock returned, `order.cancelled` recorded), with the void reason
`payment_expired`, and SHALL record `order.expired`. Expiry SHALL run in the cloud at least once a
minute. An agent order with no payment link SHALL NOT expire.

#### Scenario: Not paid in 30 minutes

- **WHEN** an agent order created at 12:00 is still unpaid at 12:31
- **THEN** it is voided with reason `payment_expired`, its stock is returned, and `order.expired` is recorded

#### Scenario: A payment still pending

- **WHEN** an agent order's link expired and a `PENDING` transaction exists for it
- **THEN** the order is not voided until that transaction is approved (paid) or fails (voided on the next run)

#### Scenario: No link was created

- **WHEN** an agent order has no payment link and 30 minutes pass
- **THEN** the order stays open for staff to pay or void
