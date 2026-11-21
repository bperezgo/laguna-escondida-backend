## ADDED Requirements

### Requirement: A delivery order records its zone's fee

When a `delivery` order is created, it SHALL record the delivery fee (with its base and ICO amount) of the zone it is accepted in, as that zone's fee is at that moment. `dine_in` and `takeaway` orders SHALL have no delivery fee. A delivery fee SHALL NOT be accepted from the request: one sent by the caller SHALL be ignored.

The order's total SHALL be the sum of its lines plus its delivery fee, and the amount charged for the order (the payment link and the paid amount) SHALL be that total. The delivery fee SHALL NOT be a line of the order: it SHALL NOT appear on any kitchen board, reserve or consume stock, or take a discount.

Every order read (the active list, a single order, the closed list and a single closed order) SHALL return the delivery fee apart from the lines, together with the total that includes it.

#### Scenario: Delivery into a zone with a fee

- **WHEN** a delivery order with lines worth `32000` is created with a pin inside zone "Centro" (fee `5000`)
- **THEN** the order records delivery fee `5000`, its total is `37000`, and its payment link charges `37000`

#### Scenario: Delivery into a free zone

- **WHEN** a delivery order with lines worth `32000` is created inside a zone with fee `0`
- **THEN** the order records delivery fee `0` and its total is `32000`

#### Scenario: Takeaway has no fee

- **WHEN** a takeaway order is created
- **THEN** it has no delivery fee and its total is the sum of its lines, as before

#### Scenario: A fee sent by the caller

- **WHEN** a delivery order is created inside zone "Centro" (fee `5000`) with a request that also says `delivery_fee: 0`
- **THEN** the order records delivery fee `5000`

#### Scenario: The fee is not cooked

- **WHEN** a delivery order with a fee is paid and released to the kitchen
- **THEN** only its product lines appear on the kitchen boards

## MODIFIED Requirements

### Requirement: Changing an order keeps its type, contact and location

Updating an order's products or details SHALL NOT change its order type, contact, location, zone or delivery fee. The order's total after the update SHALL be the sum of its new lines plus its unchanged delivery fee.

#### Scenario: Changing the lines of a delivery order

- **WHEN** an unpaid delivery order's lines are replaced
- **THEN** the order is still `delivery` with the same phone, address, location and zone

#### Scenario: Changing lines after the zone was repriced

- **WHEN** an unpaid delivery order with fee `5000` has its lines replaced by lines worth `20000` after its zone's fee changed to `6000`
- **THEN** the order keeps fee `5000` and its total is `25000`
