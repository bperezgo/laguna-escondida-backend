## Purpose

Lets each organization choose how its agent orders are paid: confirmed by staff after a transfer
(`manual`), or confirmed by Wompi (`wompi`), and holds what the manual strategy needs.

## ADDED Requirements

### Requirement: Each organization has one payment strategy for agent orders

Every organization SHALL have exactly one payment strategy for agent orders: `manual` or `wompi`.
An organization that never chose one SHALL be `wompi`, so an organization that exists before this
change behaves as before. The strategy SHALL apply to orders created after it is set. Each agent
order SHALL record the strategy it was created under and SHALL keep it through its changes, its
payment and its expiry. Staff-created and `dine_in` orders SHALL have no payment strategy. Payment
strategies SHALL exist only in the cloud.

#### Scenario: An organization that never chose

- **WHEN** an organization created before this change takes an agent order
- **THEN** the order's strategy is `wompi`, and it behaves as before

#### Scenario: The strategy changes while an order waits

- **WHEN** an agent order is created under `manual` and the admin then chooses `wompi`
- **THEN** that order stays `manual`, and the next agent order is `wompi`

#### Scenario: An order taken at the till

- **WHEN** staff create a takeaway order
- **THEN** it has no payment strategy

### Requirement: The organization's admin reads and sets the payment settings

`GET /api/organization/payment-settings` SHALL return `strategy`, `manual_instructions` and
`manual_window_minutes` for the organization of the caller's token, and SHALL need
`integrations:read`. `PUT /api/organization/payment-settings` SHALL replace the three and respond
`200` with them, and SHALL need `integrations:manage`. It SHALL be refused with `400` naming the
field, and nothing stored, when:
- `strategy` is not `manual` or `wompi`
- `strategy` is `manual` and `manual_instructions` is empty after trimming
- `manual_instructions` is longer than 1000 characters
- `manual_window_minutes` is not a whole number from 10 to 1440

`manual_window_minutes` SHALL default to 60. Choosing `wompi` SHALL keep the stored instructions
and window. The routes SHALL NOT exist on an edge box.

#### Scenario: Choosing manual

- **WHEN** Laguna's admin puts `{"strategy": "manual", "manual_instructions": "Llave Bre-B 3001234567, a nombre de Laguna Escondida SAS", "manual_window_minutes": 60}`
- **THEN** the response is `200` with those values, and Laguna's next agent order is `manual`

#### Scenario: Manual without instructions

- **WHEN** the admin puts `{"strategy": "manual", "manual_instructions": "  "}`
- **THEN** the response is `400` naming `manual_instructions`, and the strategy is unchanged

#### Scenario: A manager is refused

- **WHEN** Laguna's manager puts the payment settings
- **THEN** the response is `403` and nothing changes

### Requirement: A manual agent order carries the transfer instructions and its own expiry

An agent order created under `manual` SHALL get no payment link and no QR. It SHALL record a
payment expiry of its creation time plus the organization's `manual_window_minutes` at that
moment. Changing the order SHALL NOT change that expiry. Every read of the order SHALL return
`payment_strategy: manual`, the organization's current `manual_instructions` as
`payment_instructions`, `payment_expires_at` and `payment_under_review`. The order SHALL be held
out of the kitchen until paid, like every unpaid takeaway or delivery order.

#### Scenario: A manual delivery order

- **WHEN** an agent delivery order is created at 12:00 for an organization on `manual` with a 60 minute window
- **THEN** it has no payment link, `payment_expires_at` is 13:00, and it returns the organization's instructions

#### Scenario: A change does not buy time

- **WHEN** that order's lines are changed at 12:40
- **THEN** `payment_expires_at` is still 13:00

### Requirement: Staff confirm a manual order's payment by paying it

A manual agent order SHALL be paid by staff with the same action as paying an order at the till,
with the payment method staff choose. Paying SHALL do exactly what it does today: the bill is
written, the electronic invoice queued, the lines released to the kitchen and `order.paid`
recorded, so the customer is notified as for any paid agent order.

#### Scenario: Staff saw the transfer arrive

- **WHEN** staff pay a manual agent order as a transfer
- **THEN** its lines appear on the kitchen board and `order.paid` is recorded for the customer

### Requirement: Staff can mark a manual order's payment as under review

`PUT /api/orders/:id/payment-review` SHALL mark an unpaid manual agent order as under review, and
`DELETE /api/orders/:id/payment-review` SHALL clear the mark. Both SHALL need the permission that
paying an order needs, SHALL respond with the order, and SHALL be idempotent. They SHALL be
refused with `409` for an order that is paid, voided, or not a manual agent order, and with `404`
for an unknown order. The mark SHALL change nothing else about the order.

#### Scenario: Staff start checking a receipt

- **WHEN** staff put the payment review of an unpaid manual order
- **THEN** the order reads `payment_under_review: true`

#### Scenario: A Wompi order

- **WHEN** staff put the payment review of an agent order created under `wompi`
- **THEN** the response is `409` and nothing changes

### Requirement: The ops UI sets the payment strategy and shows manual orders to confirm

The Ajustes page SHALL have a section for the payment of WhatsApp orders showing the strategy,
the manual instructions and the manual window. A user with `integrations:manage` SHALL be able to
change them; without it the section SHALL be read-only. A refused save SHALL show its reason next
to the named field. When `wompi` is chosen and the organization has no Wompi integration, the
section SHALL say that orders are refused until Wompi is configured. The section SHALL be hidden
on an edge box and for users without `integrations:read`.

An unpaid manual agent order SHALL be shown to staff as awaiting payment confirmation, with when
it expires, an action to mark or unmark it under review, and the existing action to pay it.

#### Scenario: Switching to manual

- **WHEN** Laguna's admin chooses manual, writes the instructions and saves
- **THEN** the section shows manual with those instructions and a 60 minute window

#### Scenario: Wompi chosen without keys

- **WHEN** an organization on `wompi` has no Wompi integration and its admin opens Ajustes
- **THEN** the section says WhatsApp orders are refused until Wompi is configured

#### Scenario: Reviewing a transfer

- **WHEN** staff open an unpaid manual order and choose to review its payment
- **THEN** the order shows as under review and offers to pay it or stop the review
