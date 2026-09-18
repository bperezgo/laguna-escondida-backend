## Purpose

Lets an operator declare, per composite product, which of its ingredients are customer-configurable side dishes and the quantity bounds each may take, so servers can adjust a plate's sides within controlled limits.

## ADDED Requirements

### Requirement: Mark a composite ingredient as a side dish
The system SHALL allow an operator to designate an ingredient of a composite product as a side dish, with a default quantity, a minimum quantity, and a maximum quantity. Ingredients not so designated remain fixed and are not customer-configurable.

#### Scenario: Designate an existing ingredient as a side dish
- **WHEN** an operator marks the "salad" ingredient of a plate as a side dish with default 1, min 0, max 2
- **THEN** the plate reports "salad" as a side-dish option with default 1, min 0, max 2
- **AND** the plate's other ingredients remain fixed and are not reported as side-dish options

#### Scenario: Fixed ingredient is unaffected
- **WHEN** a plate has a fixed ingredient (e.g. the protein) that is not marked as a side dish
- **THEN** that ingredient is not listed among the plate's side-dish options
- **AND** its recipe quantity is unchanged

### Requirement: Side-dish quantity bounds are valid
The system SHALL reject a side-dish configuration unless `0 <= min <= default <= max`.

#### Scenario: Reject max below default
- **WHEN** an operator configures a side dish with default 2 and max 1
- **THEN** the system rejects the configuration with a validation error
- **AND** no change is persisted

#### Scenario: Reject negative minimum
- **WHEN** an operator configures a side dish with min -1
- **THEN** the system rejects the configuration with a validation error

### Requirement: Offered alternative side dish
The system SHALL support a side dish whose default quantity is 0, representing an alternative that is not part of the standard plate but may be added within its maximum.

#### Scenario: Configure an add-on alternative
- **WHEN** an operator marks "fries" as a side dish of a plate with default 0, min 0, max 2
- **THEN** the plate lists "fries" as a side-dish option
- **AND** a plate ordered with no changes consumes zero "fries"

### Requirement: List a product's side-dish options
The system SHALL expose, for a composite product, the set of its side-dish options with each option's default, min, and max, so a client can render the +/- controls and enforce bounds.

#### Scenario: Read side-dish options for a plate
- **WHEN** a client requests the side-dish configuration for a plate
- **THEN** the system returns each side-dish option with its ingredient, default, min, and max
