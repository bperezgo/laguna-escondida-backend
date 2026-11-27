# delivery-tracking Specification

## Purpose
Tracks a paid takeaway or delivery order from the kitchen to the customer's hands: which orders are ready, which courier has each one on the road, whether it was delivered or why it failed, so staff can manage deliveries and the customer can be told.

## Requirements

### Requirement: Takeaway and delivery orders have a delivery status

A paid, not voided `takeaway` or `delivery` order SHALL have exactly one delivery status:
- `preparing`: no delivery status has been recorded and at least one of its lines is neither completed nor cancelled;
- `ready`: no delivery status has been recorded and every line is completed or cancelled;
- `out_for_delivery`, `delivered` or `failed`: the last delivery status recorded for it.

`preparing` and `ready` SHALL be derived from the lines and not recorded. An unpaid order, a voided order and a `dine_in` order SHALL have no delivery status.

#### Scenario: Paid with lines still cooking

- **WHEN** a delivery order is paid and one of its lines is `in_progress`
- **THEN** its delivery status is `preparing`

#### Scenario: Kitchen finishes

- **WHEN** the last pending line of a paid takeaway order is completed
- **THEN** its delivery status is `ready`

#### Scenario: Dine-in is not tracked

- **WHEN** a dine-in order is paid
- **THEN** it has no delivery status and does not appear on the dashboard

### Requirement: Delivery status changes follow fixed transitions

The delivery status SHALL change only through these transitions:

| From | To | Order type |
|---|---|---|
| `preparing` or `ready` | `out_for_delivery` | `delivery` |
| `out_for_delivery` | `delivered` | `delivery` |
| `out_for_delivery` | `failed` | `delivery` |
| `failed` | `out_for_delivery` (re-dispatch) | `delivery` |
| `preparing` or `ready` | `delivered` | `takeaway` |

Any other change SHALL be refused with `409 Conflict`, code `INVALID_DELIVERY_TRANSITION`, and the order's current status, and nothing SHALL be recorded. A change on an unpaid, voided or `dine_in` order SHALL be refused the same way. `delivered` SHALL be final.

#### Scenario: Delivered without leaving

- **WHEN** a manager marks a `ready` delivery order `delivered`
- **THEN** the request fails with `409` and code `INVALID_DELIVERY_TRANSITION`, and the status is still `ready`

#### Scenario: Takeaway picked up

- **WHEN** a manager marks a `ready` takeaway order `delivered`
- **THEN** its status is `delivered`

#### Scenario: Takeaway on the road

- **WHEN** a takeaway order is set `out_for_delivery`
- **THEN** the request fails with `409` and code `INVALID_DELIVERY_TRANSITION`

#### Scenario: An unpaid order

- **WHEN** an unpaid delivery order is set `out_for_delivery`
- **THEN** the request fails with `409` and code `INVALID_DELIVERY_TRANSITION`

### Requirement: An order on the road has a courier

Setting an order `out_for_delivery` SHALL name its courier: either an active user of the organization holding the courier role, or, for a third party, a non-empty courier name of at most 80 characters. A request naming both, neither, or a user who is not an active courier of the organization SHALL be rejected with a validation error, and nothing SHALL be recorded. The courier SHALL stay with the order while it is `out_for_delivery`, and a re-dispatch SHALL name a courier again, which MAY be a different one.

#### Scenario: Assigned to a staff courier

- **WHEN** a manager sets a ready delivery order `out_for_delivery` with courier user Juan
- **THEN** the order is `out_for_delivery` with courier Juan

#### Scenario: A third-party courier

- **WHEN** a manager sets a ready delivery order `out_for_delivery` with courier name "Mensajeros Ya"
- **THEN** the order is `out_for_delivery` with courier "Mensajeros Ya" and no courier user

#### Scenario: A waiter as courier

- **WHEN** a manager sets an order `out_for_delivery` with a user who does not hold the courier role
- **THEN** the request fails with a validation error and the status is unchanged

### Requirement: A failed delivery records a reason

Setting an order `failed` SHALL require a reason, one of `customer_unreachable`, `wrong_address`, `refused` or `other`, and SHALL accept an optional note of at most 500 characters. A missing or unknown reason, or a longer note, SHALL be rejected with a validation error, and nothing SHALL be recorded. The note SHALL be visible only to the organization's staff.

#### Scenario: Customer not answering

- **WHEN** a courier marks their order `failed` with reason `customer_unreachable` and note "portería no deja subir, no contesta"
- **THEN** the order is `failed` with that reason and note

#### Scenario: Failed without a reason

- **WHEN** an order is marked `failed` with no reason
- **THEN** the request fails with a validation error and the order is still `out_for_delivery`

### Requirement: Who may change a delivery status

An admin or manager SHALL be able to make every transition, on any order of the organization.

A courier SHALL be able to:
- set a `ready` delivery order with no courier `out_for_delivery` with themselves as courier (claim it);
- set an order whose courier user is themselves from `out_for_delivery` to `delivered` or `failed`.

Any other change by a courier SHALL be refused with `403 Forbidden`, and nothing SHALL be recorded. That includes a `preparing` order, another courier's order, a re-dispatch, a takeaway order and naming another courier. Other roles SHALL NOT change delivery statuses.

#### Scenario: A courier claims an order

- **WHEN** courier Juan claims a ready delivery order
- **THEN** the order is `out_for_delivery` with courier Juan

#### Scenario: Another courier's order

- **WHEN** courier Pedro marks courier Juan's order `delivered`
- **THEN** the request fails with `403` and the order is still `out_for_delivery`

#### Scenario: A third party reports back

- **WHEN** a manager marks an order on the road with courier name "Mensajeros Ya" `delivered`
- **THEN** the order is `delivered`

#### Scenario: A courier re-dispatches

- **WHEN** a courier sets a `failed` order `out_for_delivery`
- **THEN** the request fails with `403`

### Requirement: Every status change is kept in the history

Each recorded status change SHALL be kept as a history entry with: the new status, the instant, the user who made it, the courier (user or name) when the order is on the road, and the reason and note when it failed. History entries SHALL NOT be changed or deleted. An order's history SHALL be returned in chronological order, together with the instants the order was paid and became `ready`.

#### Scenario: Failed then delivered

- **WHEN** an order goes out with Juan, fails with `wrong_address`, is re-dispatched with Pedro and is delivered
- **THEN** its history has four entries in that order, and the failed entry keeps Juan, the reason and the note

### Requirement: The dashboard lists takeaway and delivery orders by status

Admins and managers SHALL be able to list the organization's paid, not voided takeaway and delivery orders with, for each: the order identifier, order type, customer name and phone, delivery address, location and map link for deliveries, delivery status, courier, the instant it entered its current status and its last failure reason.

The list SHALL include every order that is not `delivered`, whatever its day, and orders `delivered` during the current business day. It SHALL accept an optional delivery status filter and an optional order type filter. An invalid filter value SHALL be rejected with a validation error.

Changes to a listed order's delivery status, and orders becoming `ready`, SHALL be reflected in an open dashboard without a reload.

#### Scenario: Orders on the road

- **WHEN** the list is requested with status `out_for_delivery`
- **THEN** only orders on the road are returned, each with its courier

#### Scenario: Yesterday's undelivered order

- **WHEN** a delivery order paid yesterday is still `failed`
- **THEN** it is on today's list

#### Scenario: Courier cannot read the dashboard

- **WHEN** a courier requests the dashboard list
- **THEN** the request fails with `403`

### Requirement: A courier sees what they can claim and what they carry

A courier SHALL be able to list:
- the organization's `ready` delivery orders with no courier, oldest ready first;
- their own `out_for_delivery` orders, in the order they were set out for delivery, oldest first.

Each SHALL carry the order identifier, customer name and phone, delivery address, map link and lines. Neither list SHALL include other couriers' orders or amounts beyond whether the order is paid.

#### Scenario: Juan's queue

- **WHEN** Juan claimed order A at 12:10 and order B at 12:05, and Pedro has order C on the road
- **THEN** Juan's list returns B then A, and not C

### Requirement: Orders can be printed as a bag slip

From the dashboard, staff SHALL be able to print a slip for a takeaway or delivery order with: the order identifier, order type, customer name and phone, the delivery address, a QR code of the map link for deliveries, and the lines with their quantities. The slip SHALL show that the order is paid and SHALL NOT show the amount.

#### Scenario: Printing a delivery slip

- **WHEN** a manager prints the slip of a paid delivery order
- **THEN** the slip shows the address and a QR code that opens the order's map link

### Requirement: Delivery tracking exists only in the cloud

The edge box SHALL NOT serve delivery statuses, the dashboard, or the courier lists. A request for them on the box SHALL respond `404 Not Found`.

#### Scenario: Dashboard on the box

- **WHEN** the dashboard list is requested on the edge box
- **THEN** the response is `404`
