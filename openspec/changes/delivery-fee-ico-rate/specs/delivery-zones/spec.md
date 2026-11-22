## MODIFIED Requirements

### Requirement: A delivery zone has a flat delivery fee

A delivery zone SHALL have a delivery fee: one amount charged for every delivery into that zone, in the organization's currency, given as the final price the customer pays with the ICO included. The fee SHALL be zero or positive with at most two decimals; any other value SHALL be rejected with a validation error and nothing SHALL be stored. A zone created without a fee SHALL have fee `0`, which means free delivery.

A delivery zone SHALL have an ICO rate for its fee, given as a fraction (`0.08` means 8%). The rate SHALL be at least `0` and less than `1`, with at most four decimals; any other value SHALL be rejected with a validation error and nothing SHALL be stored. A zone created without a rate SHALL have rate `0.08`.

Alongside the fee the system SHALL store and return its base and ICO amount, derived from the fee at the zone's ICO rate exactly as for a product's price (`base = fee / (1 + rate)` rounded to two decimals, `ico = fee - base`). The fee and the rate SHALL be returned on every zone read.

Admins and managers SHALL be able to set and change the fee and the rate with the zone, under the same permissions as the rest of the zone. Changing either SHALL recompute the zone's base and ICO and SHALL affect only orders created afterwards.

Zones that existed before the fee was introduced SHALL have fee `0`. Zones that existed before the rate was introduced SHALL have rate `0.08`, and their stored base and ICO SHALL be unchanged.

#### Scenario: A zone with a fee

- **WHEN** a manager creates zone "Centro" with fee `5000` and no rate
- **THEN** the zone is stored and returned with fee `5000`, ICO rate `0.08`, base `4629.63` and ICO `370.37`

#### Scenario: A zone with its own rate

- **WHEN** a manager creates zone "Norte" with fee `5000` and rate `0.19`
- **THEN** the zone is stored and returned with ICO rate `0.19`, base `4201.68` and ICO `798.32`

#### Scenario: A zone without a fee

- **WHEN** a zone is created with no fee
- **THEN** it is stored with fee `0`

#### Scenario: A negative fee

- **WHEN** a zone is created or updated with fee `-1000`
- **THEN** the request fails with a validation error and the zone is not created or changed

#### Scenario: An invalid rate

- **WHEN** a zone is created or updated with rate `-0.01`, `1` or `8`
- **THEN** the request fails with a validation error and the zone is not created or changed

#### Scenario: Changing a zone's rate

- **WHEN** zone "Centro" has fee `5000` at rate `0.08` and its rate is changed to `0.19`
- **THEN** the zone returns base `4201.68` and ICO `798.32`, and its fee stays `5000`

#### Scenario: Repricing a zone

- **WHEN** zone "Centro" has fee `5000`, a delivery order is created in it, and the zone's fee is changed to `6000` or its rate to `0.19`
- **THEN** the existing order keeps fee `5000`, its base, its ICO and rate `0.08`, and the next order in "Centro" gets the new values

#### Scenario: The agent tries to change a fee or a rate

- **WHEN** a user with only the `agent` role updates a zone's fee or rate
- **THEN** the request is refused with HTTP 403 and the zone is unchanged
