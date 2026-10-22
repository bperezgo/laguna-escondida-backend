## Purpose

Makes the cloud the single owner of on-hand stock by folding every replicated movement into the amount it already holds, so the office can record purchases, corrections and counts directly while the restaurant keeps selling offline.

## ADDED Requirements

### Requirement: The cloud is the only writer of on-hand stock

The cloud SHALL be the sole authority for a product's on-hand amount. It SHALL NOT accept an on-hand amount reported by a peer node: a replicated absolute is never assigned to the amount, whatever value it carries and however recently it was produced.

Every change to on-hand SHALL come either from a movement the cloud records itself or from a movement replicated to it as a signed change.

#### Scenario: A peer reports an absolute amount

- **WHEN** a node replicates a stock snapshot stating a product's on-hand amount
- **THEN** the cloud's on-hand amount for that product is unchanged

#### Scenario: A peer's absolute cannot erase a local increase

- **WHEN** the cloud records a purchase that raises a product's on-hand, and afterwards receives a peer snapshot produced before that purchase
- **THEN** the purchase's effect remains and no value is lost

### Requirement: Replicated movements fold into on-hand exactly once

The cloud SHALL apply each replicated movement by adding its signed change to the product's current on-hand amount, in the same transaction that records the movement as received, so a movement is never counted twice and never applied without being recorded.

A movement naming a product with no stock row SHALL create one whose amount is the movement's change. A movement that would make on-hand negative SHALL still be applied, because on-hand is an accounting result and a negative value is a real signal, not an error.

#### Scenario: A sale replicated from the restaurant

- **WHEN** a movement with a change of `-3` arrives for a product whose cloud on-hand is 30
- **THEN** the cloud's on-hand becomes 27 and the movement is recorded in the history

#### Scenario: The same movement arrives twice

- **WHEN** a movement that has already been applied is delivered again
- **THEN** the on-hand amount does not change a second time and no duplicate history entry is recorded

#### Scenario: Movements arrive in any order

- **WHEN** the same set of movements for a product is applied in a different order
- **THEN** the resulting on-hand amount is identical

#### Scenario: First movement for a product

- **WHEN** a movement arrives for a product that has no stock row
- **THEN** a stock row is created holding that movement's change

#### Scenario: Result goes below zero

- **WHEN** applying a movement would take on-hand below zero
- **THEN** the movement is applied and the negative amount is readable

### Requirement: Stock is authored in the cloud, not at the restaurant

Creating a stock row, adjusting an amount, deleting a stock row and recording a batch count SHALL be available only on a node running in cloud mode, and SHALL be absent on a node running in edge mode. Reading stock SHALL remain available on both.

Each of these operations SHALL record a movement describing the change it made, so no authored write bypasses the history.

#### Scenario: Authoring on the cloud

- **WHEN** an authorized user adjusts a product's on-hand in the cloud
- **THEN** the amount changes and one movement recording that change is written

#### Scenario: Authoring against the restaurant

- **WHEN** a stock write endpoint is called on a node running in edge mode
- **THEN** the response is `404`

### Requirement: A batch count is recorded as a delta against the cloud's own amount

A batch count SHALL be converted into a signed change computed as `counted amount - the cloud's current on-hand`, and SHALL be recorded as a movement like any other. A counted amount equal to the current on-hand SHALL produce no movement.

Because the count is a delta and not an absolute, a sale replicated after the count SHALL still take effect on top of it.

#### Scenario: Counting a shelf

- **WHEN** an operator submits a counted amount of 20 for a product whose cloud on-hand is 30
- **THEN** a movement with a change of `-10` is recorded and on-hand becomes 20

#### Scenario: A sale lands after the count

- **WHEN** a movement with a change of `-1` is replicated after a count set on-hand to 20
- **THEN** on-hand becomes 19

#### Scenario: Nothing to record

- **WHEN** the counted amount equals the cloud's current on-hand
- **THEN** no movement is written and on-hand is unchanged

### Requirement: A count states how current the cloud's view of the restaurant is

A counted amount is only as good as the movements the cloud has already received: sales made at the restaurant but not yet replicated will be applied after the count and subtract twice. The cloud SHALL therefore expose how recently it last applied a movement from the restaurant, so the counting screen can warn that a count taken now will be wrong by whatever the restaurant has not yet sent.

#### Scenario: Reading the freshness signal

- **WHEN** the batch count screen is opened
- **THEN** it can read when the cloud last applied a movement replicated from the restaurant

### Requirement: On-hand equals the sum of the movements behind it

For every product, the cloud's on-hand amount SHALL equal the sum of the changes of all its recorded movements. This invariant SHALL be checkable without stopping the system, and a product whose amount and movement sum disagree SHALL be reportable.

Because movement history predating replication was never sent to the cloud, adopting ownership SHALL write one reconciling opening-balance movement per product so the invariant holds from that point forward.

Every movement SHALL record what kind of event produced it, so the history distinguishes a sale from a purchase, a correction, a count, and an opening balance.

#### Scenario: Rebuilding an amount from history

- **WHEN** the movements recorded for a product are summed
- **THEN** the total equals that product's on-hand amount

#### Scenario: Divergence is reportable

- **WHEN** a product's on-hand amount does not equal the sum of its movements
- **THEN** that product is reported as diverged, with both values

#### Scenario: Adopting ownership

- **WHEN** the cloud takes ownership of on-hand
- **THEN** each product carries an opening-balance movement that makes its movement sum equal its amount

#### Scenario: Reading the history

- **WHEN** a product's movement history is read
- **THEN** each entry states whether it came from a sale, a purchase, an adjustment, a count, or an opening balance
