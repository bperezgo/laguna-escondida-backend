## ADDED Requirements

### Requirement: A Wompi agent order gets a QR when the organization can offer one

An organization SHALL be able to offer a QR when its Wompi integration holds a public key and an
integrity secret (see `organization-integrations`). After an agent order under the `wompi`
strategy gets its payment link, and only when the organization can offer a QR and the order was
created with the customer's acceptance of Wompi's terms (see `customer-agent-access`), the system
SHALL create a Wompi transaction with payment method `BANCOLOMBIA_QR` for the order's total, in
the organization's currency, with a reference unique to that version of the order, the
organization's current acceptance tokens and the integrity signature. It SHALL wait up to a
configured time (default 5 seconds) for Wompi to return the QR, and SHALL store the QR as Wompi
returned it with the transaction. An order SHALL have at most one active QR.

The QR SHALL never be the only way to pay: the link SHALL be created first, as today. When the
transaction is refused, the QR does not arrive in time, or it cannot be stored, creating or
changing the order SHALL still succeed with its link, and the failure SHALL be logged. An order
whose active QR has not arrived yet SHALL have the QR read from Wompi once on each read of the
order, until it arrives.

#### Scenario: A QR for a new order

- **WHEN** an agent order of 45000 COP is created with the terms accepted, for an organization that can offer a QR
- **THEN** it has an active link and an active QR of 4500000 cents with its own reference

#### Scenario: The integration has no QR credentials

- **WHEN** an agent order is created for an organization whose Wompi integration has only a private key and an events secret
- **THEN** it has a link and no QR, and no transaction is created at Wompi

#### Scenario: The terms were not accepted

- **WHEN** an agent order is created without the customer's acceptance of Wompi's terms
- **THEN** it has a link and no QR

#### Scenario: Wompi refuses the QR

- **WHEN** Wompi answers an error while the QR transaction is created
- **THEN** the order exists with its link and no QR, and the error is logged

#### Scenario: The QR arrives late

- **WHEN** Wompi returns the QR 8 seconds after the transaction was created
- **THEN** the order is created with its link only, and the next read of the order returns the QR

### Requirement: A QR lives as long as its version of the order

Changing the order's lines SHALL retire its active QR and, when a QR can still be offered, create
a new one for the new total with a new reference. Paying, voiding or expiring the order SHALL
retire its active QR. Retiring SHALL ask Wompi to cancel the pending transaction when Wompi
allows it, and SHALL always stop the QR from paying the order and from being served. A failure
to cancel at Wompi SHALL be logged and SHALL NOT fail the change, payment, void or expiry. The
order's own QR transaction, pending since it was created, SHALL NOT hold the order's expiry.

#### Scenario: A change replaces the QR

- **WHEN** the lines of an agent order with QR A are changed
- **THEN** QR A is retired, and the order has a new active QR B for the new total with another reference

#### Scenario: Paid with the link

- **WHEN** an order with an active link and an active QR is paid through its link
- **THEN** its QR is retired

#### Scenario: An unpaid QR does not keep the order alive

- **WHEN** an agent order's link expired, its QR was never paid, and no signed pending transaction exists
- **THEN** the order is voided as expired and its QR is retired

### Requirement: The QR is served as a PNG at an unguessable public URL

Each QR SHALL have a public URL carrying a random token of at least 128 bits and no identifier
of the order or the organization. `GET` on that URL SHALL need no credentials and SHALL answer
`200` with the QR as `image/png`, rendered from the stored QR, while the QR is active and has
arrived. It SHALL answer `404` for an unknown token, a retired QR or a QR that has not arrived.
The system SHALL NOT keep image files. The URL SHALL be served only by the cloud.

#### Scenario: WhatsApp fetches the image

- **WHEN** the URL of an order's active QR is requested without credentials
- **THEN** the response is `200` with a PNG of that QR

#### Scenario: The order was changed

- **WHEN** the URL of a QR retired by a change is requested
- **THEN** the response is `404`

#### Scenario: A guessed token

- **WHEN** a URL with a token that was never issued is requested
- **THEN** the response is `404`

## MODIFIED Requirements

### Requirement: An approved transaction pays the order of its active link

A signed `transaction.updated` with status `APPROVED` SHALL pay the order when the transaction is
for the order's active link or carries the reference of the order's active QR, the order is
unpaid and not voided, and the transaction's amount and currency equal the link's or the QR's. A
transaction with a payment link SHALL be matched by that link; one without SHALL be matched by
its reference. Paying SHALL be exactly what paying the order at the till does: the bill is
written, the electronic invoice queued, the lines released to the kitchen and `order.paid`
recorded. The payment code SHALL follow the transaction's payment method: card as credit card,
PSE as bank debit transfer, any other method (the QR included) as interbank debit transfer; the
bill SHALL have no fiscal customer. Paying SHALL deactivate the order's link and retire its QR.

Any other approved transaction (for a replaced or expired link, a retired QR, an unknown
reference, a paid or voided order, or a different amount) SHALL NOT pay anything, SHALL be stored
as unmatched and SHALL be logged as an error. Each Wompi transaction SHALL be applied at most
once: a transaction delivered again with a status already stored SHALL be answered `200` and
change nothing.

#### Scenario: Paid with the current link

- **WHEN** a signed `APPROVED` transaction of 4500000 COP cents arrives for the active link of an unpaid 45000 COP order
- **THEN** the order is paid, its lines appear on the kitchen board, and the event is answered `200`

#### Scenario: Paid with the current QR

- **WHEN** a signed `APPROVED` transaction of 4500000 COP cents arrives with no payment link and the reference of the active QR of an unpaid 45000 COP order
- **THEN** the order is paid, its lines appear on the kitchen board, and its link is deactivated

#### Scenario: Paid twice by redelivery

- **WHEN** the same approved transaction is delivered again
- **THEN** it is answered `200`, and the order has one bill

#### Scenario: Paid with the link and then with the QR

- **WHEN** an order was paid through its link and an approved transaction then arrives with the reference of its QR
- **THEN** the order has one bill, the second transaction is stored as unmatched, and an error is logged

#### Scenario: Paid with a replaced link

- **WHEN** an approved transaction arrives for a link the order's change deactivated
- **THEN** the order stays unpaid, the transaction is stored as unmatched, and an error is logged

#### Scenario: Paid with a retired QR

- **WHEN** an approved transaction arrives with the reference of a QR the order's change retired
- **THEN** the order stays unpaid, the transaction is stored as unmatched, and an error is logged

#### Scenario: A different amount

- **WHEN** an approved transaction of 3000000 cents arrives for a link of 4500000 cents
- **THEN** the order stays unpaid and the transaction is stored as unmatched

### Requirement: Transactions that are not approved change nothing but are kept

A signed transaction with status `PENDING`, `DECLINED`, `VOIDED` or `ERROR` SHALL be stored and
answered `200`, and SHALL NOT pay or void the order. A signed pending transaction SHALL hold the
order's expiry until it resolves. A signed `DECLINED`, `VOIDED` or `ERROR` transaction carrying
the reference of the order's active QR SHALL retire that QR; the order SHALL keep its link.

#### Scenario: A declined card

- **WHEN** a signed `DECLINED` transaction arrives for an order's active link
- **THEN** the order stays awaiting payment and its link stays active until it expires

#### Scenario: A QR payment that failed

- **WHEN** a signed `ERROR` transaction arrives with the reference of an order's active QR
- **THEN** the order stays awaiting payment with its link, and its QR is retired
