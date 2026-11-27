## Purpose

Defines how a paid bill's line amounts, taxes and header totals are computed for the fiscal provider (DIAN e-invoicing), so every invoice is accepted and still adds up to what the customer paid, and how a rejected pending invoice is rebuilt and re-sent.

## ADDED Requirements

### Requirement: Line tax is computed once per line from the tax-inclusive line total

For each invoice line, the system SHALL compute the line's tax-inclusive total as the product's tax-inclusive unit price × quantity. It SHALL split that total into a base and one amount per tax (VAT, ICO). Each tax amount is the line total × the tax's rate / (1 + the sum of the line's rates), rounded once to 2 decimals. The base is the line total minus the line's tax amounts. The line's `total` SHALL be the base, each tax's `taxAmount` SHALL be that line amount, and each tax's `percent` SHALL be its rate × 100 with 2 decimals.

A line's tax amount SHALL NOT be computed as a per-unit tax amount rounded first and then multiplied by the quantity.

#### Scenario: Product sold by the gram with a large quantity

- **WHEN** a bill has 3352 units of a product priced $32.00 including 8% ICO and no VAT
- **THEN** the invoice line has `total` `99318.52`, ICO `taxAmount` `7945.48` and `percent` `8.00`
- **AND** `total` + ICO `taxAmount` equals `107264.00`, the amount charged for the line

#### Scenario: Line with one unit

- **WHEN** a bill has 1 unit of a product priced $8,000.00 including 8% ICO
- **THEN** the line's ICO `taxAmount` is `592.59` and its `total` is `7407.41`

#### Scenario: Line with both VAT and ICO

- **WHEN** a line's product carries VAT 19% and ICO 8%
- **THEN** each tax amount is the line total × its own rate / 1.27, rounded to 2 decimals
- **AND** the base plus both tax amounts equals the line's tax-inclusive total exactly

#### Scenario: Line tax matches the provider's recomputation

- **WHEN** any invoice line is sent
- **THEN** each tax's `taxAmount` equals the line `total` × that tax's rate, rounded to 2 decimals, within at most 0.01

### Requirement: Item unit price is derived from the line base

Each invoice line's `unitPrice` SHALL be the line base / quantity, rounded to 2 decimals, and its `quantity` SHALL be the sold quantity. If the provider rejects a line because `unitPrice` × `quantity` differs from `total`, the system SHALL keep the line's `total` and tax amounts as computed and carry the remainder on that line in a form the provider accepts. The invoice SHALL still add up to the amount charged.

#### Scenario: Unit price of a weighed product

- **WHEN** a line has `total` `99318.52` and quantity 3352
- **THEN** its `unitPrice` is `29.63`

### Requirement: Invoice header amounts equal the sum of its lines

The invoice request's `taxAmount` and `totalAmount` SHALL be computed from its lines, including the delivery-fee line when present:
- `taxAmount` is the sum of all line tax amounts.
- `totalAmount` is the sum of line `total`s.

`payAmount` and `discountAmount` SHALL stay the bill's stored `pay_amount` and `discount_amount`, as sent today. The amount charged doesn't change.

This SHALL hold for bills stored before this change, whose stored VAT/ICO/net totals used per-unit rounding. The request SHALL NOT copy those stored tax or net totals.

#### Scenario: Bill stored with per-unit rounded totals

- **WHEN** a bill stored with `ico` `7944.24` and `total_amount` `99319.76` (one line of 3352 × $32.00 at 8% ICO) is submitted
- **THEN** the request has `taxAmount` `7945.48`, `totalAmount` `99318.52` and `payAmount` `107264.00`

#### Scenario: Bill with a delivery fee

- **WHEN** a bill has product lines and a delivery fee
- **THEN** the delivery-fee line's base and ICO are included in `totalAmount` and `taxAmount`
- **AND** `payAmount` equals the bill's stored `pay_amount`

### Requirement: Newly paid bills store totals computed per line

When a bill is paid on the edge or in the cloud, its stored VAT, ICO and net totals SHALL be the sums of the per-line amounts defined above. The stored net + VAT + ICO − discount + tip SHALL equal the stored `pay_amount`.

#### Scenario: Paying a bill with a weighed product

- **WHEN** a bill with 3352 units of a $32.00 (8% ICO) product is paid
- **THEN** the stored `ico` is `7945.48`, the stored `total_amount` is `99318.52` and `pay_amount` is `107264.00`

### Requirement: A pending invoice can be rebuilt without losing its number

A `pending` invoice whose cached request has been cleared SHALL be rebuilt from the bill with the current computation on the next submission run. It SHALL keep its already assigned prefix and consecutive. An invoice that was already submitted SHALL NOT be rebuilt or re-sent.

#### Scenario: Re-sending a rejected invoice after the fix

- **WHEN** a `pending` invoice with consecutive `LAG-365` was rejected for a line-tax mismatch, and its cached request is cleared
- **THEN** the next submission run sends a request built with per-line taxes under the same prefix `LAG` and consecutive `365`
- **AND** on provider acceptance the invoice becomes `submitted`

#### Scenario: Submitted invoices are untouched

- **WHEN** the cached request of a `submitted` invoice is cleared
- **THEN** no submission run sends it again
