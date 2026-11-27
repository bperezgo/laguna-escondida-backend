## MODIFIED Requirements

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
