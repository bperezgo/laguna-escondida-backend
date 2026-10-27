## ADDED Requirements

### Requirement: Takeaway and delivery orders are taken only while available

Creating a `takeaway` or `delivery` order SHALL be refused while that order type is unavailable for the organization at the moment of the request (see `ordering-availability`), and no order SHALL be created. The rule SHALL apply to every creator, staff and agent alike.
- A paused type SHALL be refused with `422 Unprocessable Entity`, code `ORDERING_PAUSED`, the order type and the `paused_reason`.
- A type outside its hours SHALL be refused with `422 Unprocessable Entity`, code `OUTSIDE_ORDERING_HOURS`, the order type and `next_opens_at`.

The check SHALL run after the request is validated and, on the box, after the delivery cloud-only refusal. For a delivery, it SHALL run before the zone check.

`dine_in` orders SHALL NOT be affected. Updating, voiding or paying an order that already exists SHALL NOT be affected.

#### Scenario: Delivery while paused

- **WHEN** delivery is paused and a valid delivery order inside an active zone is created
- **THEN** the request fails with `422` and code `ORDERING_PAUSED`, and no order exists

#### Scenario: Takeaway after closing

- **WHEN** the takeaway schedule has only Monday 11:30–22:30 and a takeaway order is created on Monday at 23:00
- **THEN** the request fails with `422`, code `OUTSIDE_ORDERING_HOURS` and `next_opens_at` the next Monday 11:30

#### Scenario: Delivery paused but takeaway open

- **WHEN** delivery is paused, takeaway is available, and a takeaway order is created
- **THEN** the takeaway order is created

#### Scenario: Dine-in is never gated

- **WHEN** both delivery and takeaway are paused and a dine-in order is created
- **THEN** the order is created as before

#### Scenario: An existing order during a pause

- **WHEN** an unpaid delivery order exists and delivery is then paused
- **THEN** its lines can still be changed and it can still be voided or paid

#### Scenario: Nothing configured

- **WHEN** an organization has no ordering settings and a takeaway order is created at any hour
- **THEN** the order is created as before
