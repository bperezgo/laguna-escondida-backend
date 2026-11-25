## MODIFIED Requirements

### Requirement: Agent orders are paid with a payment link

A `takeaway` or `delivery` order created by the customer agent (an agent order) SHALL be paid
the way its payment strategy says (see `payment-strategies`). Under `wompi` it SHALL be paid
through its payment link or its QR (see `payment-links`), or by staff. Under `manual` it SHALL
get neither and SHALL be paid by staff. Staff-created and `dine_in` orders SHALL NOT get a
payment link, a QR or a payment strategy and SHALL behave as before.

#### Scenario: A WhatsApp delivery order

- **WHEN** the customer agent creates a delivery order for an organization on `wompi`, with or without origin channel `laguna-escondida-whatsapp`
- **THEN** it has a payment link and stays held out of the kitchen until paid

#### Scenario: A WhatsApp order for a restaurant that confirms by hand

- **WHEN** the customer agent creates a takeaway order for an organization on `manual`
- **THEN** it has no payment link and no QR, and stays held out of the kitchen until staff pay it

#### Scenario: A takeaway order taken at the till

- **WHEN** staff create a takeaway order
- **THEN** it has no payment link, as before

### Requirement: Agent orders need payment links configured

Creating an agent order for an organization on the `wompi` strategy SHALL be refused with
`422 Unprocessable Entity` and the code `PAYMENT_LINKS_NOT_CONFIGURED` when the organization does
not have payment links configured, and no order SHALL be created. The check SHALL run after the
request is validated and after the ordering availability check, before the zone check. An
organization on the `manual` strategy SHALL NOT need payment links. Staff-created orders SHALL
NOT be affected.

#### Scenario: An organization without Wompi

- **WHEN** an agent takeaway order is created for an organization on `wompi` without payment links configured
- **THEN** the request fails with `422` and code `PAYMENT_LINKS_NOT_CONFIGURED`, and no order exists

#### Scenario: A manual organization without Wompi

- **WHEN** an agent takeaway order is created for an organization on `manual` with no Wompi integration
- **THEN** the order is created

#### Scenario: Staff in the same organization

- **WHEN** staff of that organization create a takeaway order
- **THEN** the order is created as before

### Requirement: An unpaid agent order expires

An agent order under `wompi` SHALL be voided when its active payment link has expired and it is
still unpaid, unless a signed pending transaction exists for it, in which case it SHALL wait
until that transaction resolves. An agent order under `wompi` with no payment link SHALL NOT
expire.

An agent order under `manual` SHALL be voided when its payment expiry has passed and it is still
unpaid, unless staff have marked its payment as under review, in which case it SHALL wait until
it is paid, voided or the mark is cleared.

Expiring SHALL deactivate the order's link and retire its QR, void the order exactly as staff
voiding it does (soft-deleted, stock returned, `order.cancelled` recorded), with the void reason
`payment_expired`, and SHALL record `order.expired`. Expiry SHALL run in the cloud at least once
a minute.

#### Scenario: Not paid in 30 minutes

- **WHEN** a `wompi` agent order created at 12:00 is still unpaid at 12:31
- **THEN** it is voided with reason `payment_expired`, its stock is returned, and `order.expired` is recorded

#### Scenario: A payment still pending

- **WHEN** an agent order's link expired and a signed `PENDING` transaction exists for it
- **THEN** the order is not voided until that transaction is approved (paid) or fails (voided on the next run)

#### Scenario: No link was created

- **WHEN** a `wompi` agent order has no payment link and 30 minutes pass
- **THEN** the order stays open for staff to pay or void

#### Scenario: A manual order nobody paid

- **WHEN** a `manual` agent order created at 12:00 with a 60 minute window is still unpaid at 13:01
- **THEN** it is voided with reason `payment_expired`, its stock is returned, and `order.expired` is recorded

#### Scenario: Staff are checking the transfer

- **WHEN** a `manual` agent order's payment expiry passes while it is marked under review
- **THEN** the order is not voided

#### Scenario: The review is dropped after the expiry

- **WHEN** staff clear the review mark of a `manual` order whose payment expiry has passed
- **THEN** the order is voided as expired on the next run
