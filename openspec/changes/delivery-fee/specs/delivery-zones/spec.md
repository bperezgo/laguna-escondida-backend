## ADDED Requirements

### Requirement: A delivery zone has a flat delivery fee

A delivery zone SHALL have a delivery fee: one amount charged for every delivery into that zone, in the organization's currency, given as the final price the customer pays with the ICO included. The fee SHALL be zero or positive with at most two decimals; any other value SHALL be rejected with a validation error and nothing SHALL be stored. A zone created without a fee SHALL have fee `0`, which means free delivery.

Alongside the fee the system SHALL store and return its base and ICO amount, derived from the fee at the ICO rate of 8% exactly as for a product's price (`base = fee / 1.08` rounded to two decimals, `ico = fee - base`). The fee SHALL be returned on every zone read.

Admins and managers SHALL be able to set and change the fee with the zone, under the same permissions as the rest of the zone. Changing a zone's fee SHALL affect only orders created afterwards.

Zones that existed before this change SHALL have fee `0`.

#### Scenario: A zone with a fee

- **WHEN** a manager creates zone "Centro" with fee `5000`
- **THEN** the zone is stored and returned with fee `5000`, base `4629.63` and ICO `370.37`

#### Scenario: A zone without a fee

- **WHEN** a zone is created with no fee
- **THEN** it is stored with fee `0`

#### Scenario: A negative fee

- **WHEN** a zone is created or updated with fee `-1000`
- **THEN** the request fails with a validation error and the zone is not created or changed

#### Scenario: Repricing a zone

- **WHEN** zone "Centro" has fee `5000`, a delivery order is created in it, and the zone's fee is changed to `6000`
- **THEN** the existing order keeps fee `5000` and the next order in "Centro" gets `6000`

#### Scenario: The agent tries to change a fee

- **WHEN** a user with only the `agent` role updates a zone's fee
- **THEN** the request is refused with HTTP 403 and the zone is unchanged

## MODIFIED Requirements

### Requirement: A point can be checked without creating an order

The system SHALL answer, for a latitude and longitude, whether it lies in an active zone and which one, and, when it does, that zone's delivery fee, without creating anything. The zone and fee reported SHALL be the ones an order created at that point at that moment would record.

#### Scenario: Checking a point

- **WHEN** a point inside zone "Centro" (fee `5000`) is checked
- **THEN** the answer is deliverable, zone "Centro", delivery fee `5000`

#### Scenario: Checking a point in a free zone

- **WHEN** a point inside a zone with fee `0` is checked
- **THEN** the answer is deliverable with delivery fee `0`

#### Scenario: Checking a point outside every zone

- **WHEN** a point outside every active zone is checked
- **THEN** the answer is not deliverable, with no zone and no fee
