## Purpose

Lets a server adjust a plate's side dishes on an order line within the configured bounds, and makes stock consumption follow the actual quantities served rather than the default recipe.

## ADDED Requirements

### Requirement: Order line accepts side-dish selections
The system SHALL allow an order line for a composite product to carry a resolved quantity for each of that product's side-dish options. A line that carries no side-dish selection SHALL be treated as the default quantities for every side dish.

#### Scenario: Line with explicit selections
- **WHEN** a server adds a plate and sets salad to 0, canasta to 3, and rice to 1
- **THEN** the order line records salad 0, canasta 3, rice 1

#### Scenario: Line with no selection uses defaults
- **WHEN** a server adds a plate without adjusting any side dish
- **THEN** the order line resolves to each side dish's default quantity
- **AND** stock consumption matches the plate's default recipe (unchanged from prior behavior)

### Requirement: Side-dish selections are validated against configuration
The system SHALL reject an order line whose side-dish selection references an ingredient that is not a side-dish option of the product, or whose quantity is outside that option's `[min, max]`.

#### Scenario: Reject quantity above maximum
- **WHEN** a server sets canasta to 4 on a plate whose canasta max is 3
- **THEN** the system rejects the order with a validation error
- **AND** no order line and no stock movement are recorded

#### Scenario: Reject non-side-dish ingredient
- **WHEN** a server submits a selection for an ingredient that is not a side-dish option of the plate
- **THEN** the system rejects the order with a validation error

#### Scenario: Remove a side dish within bounds
- **WHEN** a server sets salad to 0 on a plate whose salad min is 0
- **THEN** the order is accepted and the line records salad 0

### Requirement: Stock consumption reflects side-dish selections
When an order is created, the system SHALL consume stock for side-dish ingredients according to the line's resolved side-dish quantities, and for fixed ingredients according to the default recipe.

#### Scenario: Consumption follows selection, not default
- **WHEN** a plate whose default is 1 salad and 2 canasta is ordered with salad 0 and canasta 3
- **THEN** no salad stock is consumed
- **AND** canasta stock is consumed for 3 units per plate
- **AND** the fixed ingredients are consumed at their default amounts

### Requirement: Editing side dishes adjusts stock
When an order line is updated, the system SHALL adjust stock to reflect the difference between the previous and current side-dish quantities.

#### Scenario: Increase a side dish on an existing order
- **WHEN** an order line's canasta is changed from 2 to 3
- **THEN** stock is decremented by one additional canasta per plate

#### Scenario: Decrease a side dish on an existing order
- **WHEN** an order line's canasta is changed from 3 to 1
- **THEN** stock is incremented (restored) by two canasta per plate

### Requirement: Deleting an order restores side-dish stock
When an order is deleted, the system SHALL restore stock using the side-dish quantities recorded on the line.

#### Scenario: Restore on delete
- **WHEN** an order line recorded with salad 0 and canasta 3 is deleted
- **THEN** stock is restored for 3 canasta per plate and no salad

### Requirement: Side-dish changes do not affect price
The system SHALL NOT change the plate's price based on side-dish selections; the plate is billed at its configured price regardless of side-dish quantities.

#### Scenario: Price is unchanged by a swap
- **WHEN** a plate's canasta is reduced and fries added within their bounds
- **THEN** the line's price equals the plate's configured price with no side-dish surcharge or discount

### Requirement: Preparation area sees side-dish selections
The system SHALL include a line's resolved side-dish quantities, with the side-dish names, in the notification sent to the line's preparation area (the live kitchen feed), so the preparer sees the adjustments (removed, increased, or added side dishes). The printed receipt is the customer's "cuenta" (a billing document) and does not carry side-dish selections, which have no price effect.

#### Scenario: Live kitchen notification reflects adjustments
- **WHEN** a plate is ordered with salad 0, canasta 3, and fries 1
- **THEN** the notification sent to that plate's preparation area lists the resolved side dishes with names (ensalada 0, canasta 3, fries 1)

#### Scenario: Updated order re-notifies with new selections
- **WHEN** a line's side dishes are changed on an existing order
- **THEN** the preparation area is notified again with the updated resolved side dishes
