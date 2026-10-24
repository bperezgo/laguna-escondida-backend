## ADDED Requirements

### Requirement: Deliveries are taken in the cloud only
A `delivery` order SHALL be created only in the cloud. A `delivery` order sent to the edge box SHALL be rejected with `422 Unprocessable Entity` and the code `DELIVERY_CLOUD_ONLY`, before any zone check, and no order SHALL be created. `dine_in` and `takeaway` orders on the box SHALL behave as before.

#### Scenario: Delivery on the box
- **WHEN** a delivery order with a valid location is sent to the box
- **THEN** the box responds `422` with code `DELIVERY_CLOUD_ONLY` and no order exists

#### Scenario: Takeaway on the box
- **WHEN** a takeaway order with a contact is sent to the box
- **THEN** the order is created as before

#### Scenario: Delivery in the cloud
- **WHEN** a delivery order with a location inside an active zone is sent to the cloud
- **THEN** the order is created and records the zone, as before
