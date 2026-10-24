# delivery-orders Specification

## Purpose
Lets an order say how it leaves the restaurant (eaten in, picked up, or delivered) and carry what is needed to hand it over, so takeaway and delivery orders can be taken from customers outside the restaurant, are only accepted where the rider can go, are only prepared once paid, and can be found by the customer's phone and shown apart from dine-in orders.

## Requirements

### Requirement: Every order has an order type

Every order SHALL have exactly one order type: `dine_in`, `takeaway` or `delivery`. An order created without an order type SHALL be `dine_in`. An order type outside these three values SHALL be rejected with a validation error, and no order SHALL be created.

The order type SHALL be returned on every order read (the active list, a single order, the closed list and a single closed order).

#### Scenario: Creating an order without a type

- **WHEN** an order is created with no order type
- **THEN** the order is created as `dine_in` and reads return `order_type: "dine_in"`

#### Scenario: An unknown order type

- **WHEN** an order is created with `order_type: "pickup"`
- **THEN** the request fails with a validation error and no order exists

### Requirement: Takeaway and delivery orders carry a contact

An order MAY carry a customer name and a customer phone. A `takeaway` order SHALL require a customer phone. A `delivery` order SHALL require a customer phone, a non-empty delivery address and a delivery location (see the next requirement). A `dine_in` order SHALL NOT require any of these.

The delivery address is free text for the rider (street, building, apartment, door) and SHALL NOT be used to locate the order.

A request missing a required field SHALL be rejected with a validation error naming it, and no order SHALL be created.

The contact SHALL be stored on the order itself and SHALL NOT create, change or depend on a fiscal customer (bill owner). Choosing the fiscal customer for an invoice SHALL keep happening at payment time, unchanged.

#### Scenario: Takeaway without a phone

- **WHEN** an order is created with `order_type: "takeaway"` and no phone
- **THEN** the request fails with a validation error and no order exists

#### Scenario: Delivery without an address

- **WHEN** an order is created with `order_type: "delivery"`, a phone, a location and no address
- **THEN** the request fails with a validation error and no order exists

#### Scenario: Dine-in with no contact

- **WHEN** an order is created with `order_type: "dine_in"` and no contact fields
- **THEN** the order is created exactly as orders are created today

#### Scenario: A delivery contact is not a fiscal customer

- **WHEN** a delivery order is created for a phone that matches no bill owner
- **THEN** the order is created and no bill owner is created or modified

### Requirement: A delivery is accepted only inside a delivery zone

A `delivery` order SHALL carry a location as latitude and longitude in decimal degrees (latitude in [-90, 90], longitude in [-180, 180]). It SHALL be created only if that point lies inside an active delivery zone (see `delivery-zones`), and the order SHALL record which zone. If the point lies in no active zone, or no zone is active, the request SHALL be rejected with an "outside delivery zone" error, and no order SHALL be created.

The location SHALL be returned on order reads together with a map link built from it, so the rider can navigate to it.

`takeaway` and `dine_in` orders SHALL NOT require a location; one sent with them SHALL be ignored.

#### Scenario: Inside a zone

- **WHEN** a delivery order is created with a location inside the active zone "Centro"
- **THEN** the order is created, records zone "Centro", and reads return the location and a map link

#### Scenario: Outside every zone

- **WHEN** a delivery order is created with a location outside all active zones
- **THEN** the request fails with an "outside delivery zone" error and no order exists

#### Scenario: No zone configured

- **WHEN** a delivery order is created while no delivery zone is active
- **THEN** the request fails with an "outside delivery zone" error and no order exists

#### Scenario: Delivery without a location

- **WHEN** a delivery order is created with a phone and an address but no location
- **THEN** the request fails with a validation error and no order exists

### Requirement: Customer phones are stored in one canonical form

A customer phone SHALL be stored in E.164 form (`+` followed by country code and number), and phones from any country SHALL be accepted. Spaces, dashes, dots and parentheses SHALL be ignored. Then:

- a number starting with `+` SHALL be read as an international number;
- a number without `+` SHALL be read as a number of the restaurant's configured home region, which also accepts that region's numbers written with their country code but no `+`.

The result SHALL be stored only if it is a valid number for its country. Any other value SHALL be rejected with a validation error. The same normalization SHALL be applied to a phone used as a search filter, so a search matches however either number was typed.

A phone that comes from the messaging channel's verified sender is always international (country code included, no `+`) and SHALL be read as international whatever the home region.

#### Scenario: A local number with home region Colombia

- **WHEN** the home region is Colombia and an order is created with phone `300 555 1234`
- **THEN** the stored phone is `+573005551234`

#### Scenario: A foreign number

- **WHEN** an order is created with phone `+1 415 555 0132`
- **THEN** the stored phone is `+14155550132`

#### Scenario: A WhatsApp sender from another country

- **WHEN** the channel supplies sender `34612345678` (Spain) while the home region is Colombia
- **THEN** the stored phone is `+34612345678`

#### Scenario: A malformed phone

- **WHEN** an order is created with phone `12345`
- **THEN** the request fails with a validation error and no order exists

#### Scenario: Searching with a differently formatted phone

- **WHEN** an order was stored with `+573005551234` and the active list is filtered by `300-555-1234`
- **THEN** that order is returned

### Requirement: Unpaid takeaway and delivery orders are held out of the kitchen

The lines of a `takeaway` or `delivery` order SHALL NOT appear on any kitchen board, and SHALL NOT be set in progress or completed, until the order is paid. An attempt to set such a line in progress or complete it SHALL be refused with an "awaiting payment" conflict error, whoever makes it. `dine_in` orders SHALL behave as before.

While held, the order SHALL stay editable and voidable like any open order, and its stock SHALL be reserved exactly as for any order at creation (voiding returns it).

#### Scenario: A delivery order before payment

- **WHEN** a delivery order is created with a kitchen line
- **THEN** the line is not on the kitchen board

#### Scenario: Starting a held line

- **WHEN** staff set a line of an unpaid takeaway order in progress
- **THEN** the request fails with an "awaiting payment" error and the line is unchanged

#### Scenario: Dine-in is not held

- **WHEN** a dine-in order is created with a kitchen line
- **THEN** the line is on the kitchen board immediately, as today

### Requirement: Paying a held order releases it to the kitchen

When a `takeaway` or `delivery` order is paid, its lines SHALL appear on the kitchen boards of their areas, live, without a reload, and SHALL then follow the kitchen line lifecycle of paid orders (`kitchen-line-lifecycle` in `kitchen-lines-outlive-payment`). The kitchen's waiting time for those lines SHALL be measured from the payment instant, not from when the order was created.

#### Scenario: Payment arrives

- **WHEN** a delivery order created at 12:00 is paid at 12:20
- **THEN** its lines appear on the kitchen board with a waiting time starting at 12:20

#### Scenario: Prepared after payment

- **WHEN** a released line is completed
- **THEN** the request succeeds and the order remains closed and paid

### Requirement: The order lists can be filtered by type and phone

The active order list and the closed order list SHALL each accept an optional order type filter and an optional customer phone filter. When both are given an order SHALL match both. With no filter each list SHALL return exactly what it returns today. An invalid order type or phone in a filter SHALL be rejected with a validation error.

#### Scenario: Only delivery orders

- **WHEN** the active list is requested with `order_type=delivery` while two dine-in and one delivery order are active
- **THEN** only the delivery order is returned

#### Scenario: Paid deliveries being prepared

- **WHEN** the closed list is requested with `order_type=delivery`
- **THEN** only delivery orders closed during the business day are returned

#### Scenario: One customer's orders

- **WHEN** the active list is requested with a customer phone
- **THEN** only active orders stored with that phone are returned, of any type

#### Scenario: No filter

- **WHEN** either list is requested with no filter
- **THEN** it returns the same orders as before this change

### Requirement: Orders replicated from the restaurant stay dine-in

An order replicated to the cloud from the restaurant SHALL be stored as `dine_in` with no contact or location, whatever its content, and replication SHALL keep working with nodes that know nothing about order types. Replicating an existing order again SHALL NOT change its order type, contact or location.

#### Scenario: A pushed restaurant order

- **WHEN** the restaurant pushes a new order to the cloud
- **THEN** the cloud stores it as `dine_in` and it does not appear in the `order_type=delivery` list

### Requirement: Changing an order keeps its type, contact and location

Updating an order's products or details SHALL NOT change its order type, contact, location or zone.

#### Scenario: Changing the lines of a delivery order

- **WHEN** an unpaid delivery order's lines are replaced
- **THEN** the order is still `delivery` with the same phone, address, location and zone

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
