# delivery-fee-invoicing Specification

## Purpose
Carries a delivery order's recorded fee into its bill, its electronic invoice and the money reports, taxed as part of the restaurant service so the customer's invoice and the restaurant's accounts match what was charged.

## Requirements

### Requirement: The bill carries the delivery fee

When a delivery order with a fee greater than `0` is paid, its bill SHALL carry the fee, its base and its ICO amount. The bill's net amount SHALL include the fee's base, its ICO SHALL include the fee's ICO, and its paid amount SHALL include the whole fee, so that the paid amount still equals net plus VAT plus ICO minus discount plus tip. The tip SHALL NOT be calculated on or include the fee.

#### Scenario: Paying a delivery with a fee

- **WHEN** a delivery order with lines worth `32000` (ICO included) and fee `5000` is paid with no tip and no discount
- **THEN** its bill's paid amount is `37000`, and its ICO is the lines' ICO plus `370.37`

#### Scenario: Paying a free delivery

- **WHEN** a delivery order with fee `0` is paid
- **THEN** its bill is the same as for the same lines before this change

### Requirement: The electronic invoice has a delivery line taxed with ICO

The electronic invoice of a bill that carries a delivery fee SHALL include one extra item for the delivery: description "Domicilio", quantity 1, unit price the fee's base, and one ICO tax at the rate the order recorded with its fee, equal to the fee's ICO amount. The rate SHALL be the one recorded when the order was created, not the zone's current rate. It SHALL be the only change to the invoice; the product items and the tip SHALL be invoiced as before. A bill with no delivery fee, or fee `0`, SHALL have no delivery item.

A delivery order SHALL record its zone's ICO rate together with the fee, its base and its ICO amount, and its bill SHALL carry the same rate. Orders and bills that recorded a fee before the rate was introduced SHALL have rate `0.08`.

#### Scenario: Invoicing a delivery

- **WHEN** the invoice of a paid delivery order with fee `5000` recorded at rate `0.08` is submitted
- **THEN** it contains the product items and one "Domicilio" item with unit price `4629.63` and ICO 8% of `370.37`

#### Scenario: Invoicing after the zone's rate changed

- **WHEN** a delivery order is created in a zone at rate `0.08`, the zone's rate is changed to `0.19`, and the order is then paid and its invoice submitted
- **THEN** the "Domicilio" item has ICO 8% of `370.37`

#### Scenario: Invoicing a takeaway

- **WHEN** the invoice of a paid takeaway order is submitted
- **THEN** it contains no "Domicilio" item

### Requirement: Reports show delivery fees on their own

The daily close and the financial summary SHALL report, for their period, the delivery fees collected: the gross total, its base and its ICO. Their existing totals (collected, net, ICO, discount, tip and the per-method breakdown) SHALL include the fees, so they keep reconciling as before. With no delivery fee in the period the delivery figures SHALL be `0`.

#### Scenario: A day with deliveries

- **WHEN** two delivery orders with fees `5000` and `8000` are paid during a business day
- **THEN** that day's close reports delivery fees `13000`, and its total collected includes them

#### Scenario: A day without deliveries

- **WHEN** no delivery order is paid during a business day
- **THEN** that day's close reports delivery fees `0` and its other totals are the same as before this change
