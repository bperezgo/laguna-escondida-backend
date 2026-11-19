## ADDED Requirements

### Requirement: Payment links are configured per organization through the environment

The Wompi API URL SHALL be one environment setting shared by all organizations. Each
organization's Wompi private key and events secret SHALL be environment settings named after its
slug. An organization SHALL have payment links configured only when both are set. At startup the
cloud SHALL log, for each active organization, whether it has payment links configured. How long
a link lives SHALL be a setting defaulting to 30 minutes. The edge box SHALL NOT create links or
receive Wompi events.

#### Scenario: A restaurant with Wompi

- **WHEN** the private key and events secret are set for organization `laguna-escondida`
- **THEN** its agent orders get payment links

#### Scenario: Only one of the two keys

- **WHEN** organization `restaurante-x` has a private key but no events secret
- **THEN** it does not have payment links configured, and startup says so

### Requirement: Each version of an agent order has one active payment link

After an agent order is created, the system SHALL create a single-use Wompi payment link for the
order's total, in the organization's currency, expiring when the order expires, and SHALL store
it as the order's active link. Changing the order's lines SHALL deactivate the active link and
create a new one for the new total with the same expiry. Voiding the order SHALL deactivate its
active link. An order SHALL have at most one active link. When Wompi does not create the link,
the order SHALL remain without a link, the failure SHALL be logged, and creating or changing the
order SHALL still succeed.

#### Scenario: A link for a new order

- **WHEN** an agent order of 45000 COP is created at 12:00
- **THEN** it has an active single-use link of 4500000 cents expiring at 12:30

#### Scenario: A change replaces the link

- **WHEN** the lines of an agent order with link A are changed at 12:10
- **THEN** link A is deactivated, the order has a new active link B for the new total, and B expires at 12:30

#### Scenario: Wompi unavailable

- **WHEN** Wompi answers an error while an agent order's link is created
- **THEN** the order exists with no link, and the error is logged

### Requirement: Wompi events are accepted only when signed

The cloud SHALL accept Wompi events at an organization's webhook URL and SHALL verify each event's
checksum: the SHA-256 of the values of the fields its `signature.properties` names, in order,
followed by its `timestamp` and the organization's events secret. An event whose checksum does not
match, or for an organization without payment links configured, SHALL be refused with `401`, logged,
and SHALL change nothing. An event of a type other than `transaction.updated` SHALL be answered `200`
and ignored.

#### Scenario: A forged event

- **WHEN** an event with a valid body but a wrong checksum arrives for Laguna
- **THEN** it is refused with `401`, and no order changes

#### Scenario: Another event type

- **WHEN** a correctly signed `nequi_token.updated` arrives
- **THEN** it is answered `200` and nothing changes

### Requirement: An approved transaction pays the order of its active link

A signed `transaction.updated` with status `APPROVED` SHALL pay the order when the transaction is
for the order's active link, the order is unpaid and not voided, and the transaction's amount and
currency equal the link's. Paying SHALL be exactly what paying the order at the till does: the bill
is written, the electronic invoice queued, the lines released to the kitchen and `order.paid`
recorded. The payment code SHALL follow the transaction's payment method: card as credit card, PSE
as bank debit transfer, any other method as interbank debit transfer; the bill SHALL have no fiscal
customer.

Any other approved transaction (for a replaced or expired link, a paid or voided order, or a
different amount) SHALL NOT pay anything, SHALL be stored as unmatched and SHALL be logged as an
error. Each Wompi transaction SHALL be applied at most once: a transaction delivered again with a
status already stored SHALL be answered `200` and change nothing.

#### Scenario: Paid with the current link

- **WHEN** a signed `APPROVED` transaction of 4500000 COP cents arrives for the active link of an unpaid 45000 COP order
- **THEN** the order is paid, its lines appear on the kitchen board, and the event is answered `200`

#### Scenario: Paid twice by redelivery

- **WHEN** the same approved transaction is delivered again
- **THEN** it is answered `200`, and the order has one bill

#### Scenario: Paid with a replaced link

- **WHEN** an approved transaction arrives for a link the order's change deactivated
- **THEN** the order stays unpaid, the transaction is stored as unmatched, and an error is logged

#### Scenario: A different amount

- **WHEN** an approved transaction of 3000000 cents arrives for a link of 4500000 cents
- **THEN** the order stays unpaid and the transaction is stored as unmatched

### Requirement: Transactions that are not approved change nothing but are kept

A signed transaction with status `PENDING`, `DECLINED`, `VOIDED` or `ERROR` SHALL be stored and
answered `200`, and SHALL NOT pay or void the order. A pending transaction SHALL hold the order's
expiry until it resolves.

#### Scenario: A declined card

- **WHEN** a signed `DECLINED` transaction arrives for an order's active link
- **THEN** the order stays awaiting payment and its link stays active until it expires
