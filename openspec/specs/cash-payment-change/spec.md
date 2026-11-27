# cash-payment-change Specification

## Purpose
Helps the cashier give the right change when a bill is paid in cash, by computing change or the missing amount from what the customer hands over.

## Requirements

### Requirement: Cash block appears only for cash payments
The payment screen SHALL show the cash block (received amount, quick buttons, change result) only while the selected payment type is cash. Switching to another payment type or closing the payment screen SHALL clear the received amount.

#### Scenario: Cash selected
- **WHEN** the cashier selects "Efectivo" as the payment type
- **THEN** the cash block with an empty "Recibido" field is shown

#### Scenario: Non-cash selected
- **WHEN** the selected payment type is a card or transfer type
- **THEN** no cash block is shown

#### Scenario: Switching away resets the amount
- **WHEN** the cashier enters a received amount, switches to a transfer type, and then switches back to "Efectivo"
- **THEN** the "Recibido" field is empty

### Requirement: Received amount is optional
The received amount SHALL be optional. With it empty, paying in cash SHALL behave as it does without this feature, with no change shown and payment allowed.

#### Scenario: Pay without entering received amount
- **WHEN** "Efectivo" is selected and "Recibido" is left empty
- **THEN** no change or missing amount is shown and the cashier can submit payment

### Requirement: Change is computed from received amount
When the received amount is greater than or equal to the bill total, the system SHALL show the change to give back (received minus total), prominently, in whole currency units.

#### Scenario: Customer overpays
- **WHEN** the total is $47.500 and the cashier enters $100.000 received
- **THEN** the screen shows change to give back of $52.500

#### Scenario: Exact amount
- **WHEN** the received amount equals the total
- **THEN** the screen shows change of $0

### Requirement: Short payment blocks submission
When a received amount is entered and is less than the bill total, the system SHALL show the missing amount (total minus received) as a warning and SHALL NOT allow the payment to be submitted.

#### Scenario: Customer underpays
- **WHEN** the total is $47.500 and the cashier enters $45.000 received
- **THEN** the screen shows "Faltan $2.500" and the pay button is disabled

#### Scenario: Correcting the amount re-enables payment
- **WHEN** after an underpayment the cashier changes the received amount to $50.000
- **THEN** the missing warning disappears, change of $2.500 is shown, and payment can be submitted

### Requirement: Quick amount suggestions
The cash block SHALL offer an "Exacto" button that sets the received amount to the total, plus up to 3 suggested amounts. Each suggestion SHALL be the total rounded up to a multiple of one of the bill sizes 5.000, 10.000, 20.000, 50.000 or 100.000, strictly greater than the total, deduplicated and shown in ascending order, lowest first. Tapping a button SHALL set the received amount, which the cashier can still edit.

#### Scenario: Suggestions for a typical total
- **WHEN** the total is $47.500
- **THEN** the buttons are "Exacto", $50.000, $60.000 and $100.000

#### Scenario: Suggestions for a small total
- **WHEN** the total is $12.300
- **THEN** the buttons are "Exacto", $15.000, $20.000 and $50.000

#### Scenario: Tapping a suggestion
- **WHEN** the total is $47.500 and the cashier taps $50.000
- **THEN** "Recibido" shows $50.000 and change of $2.500 is shown

#### Scenario: Total already a round amount
- **WHEN** the total is $50.000
- **THEN** the buttons are "Exacto", $55.000, $60.000 and $100.000

### Requirement: Confirmation shows change
When a received amount covering the total is entered, the payment confirmation step SHALL state the change to give back alongside the total.

#### Scenario: Confirm with change
- **WHEN** the total is $47.500, received is $100.000, and the cashier proceeds to confirm payment
- **THEN** the confirmation message includes "Cambio: $52.500"

### Requirement: Calculator data stays on the client
The received amount and change SHALL NOT be sent to the backend or stored. The payment request SHALL be the same whether or not the calculator was used.

#### Scenario: Payment payload unchanged
- **WHEN** the cashier pays in cash after entering a received amount
- **THEN** the payment request contains only the order, payment type and optional customer, as before
