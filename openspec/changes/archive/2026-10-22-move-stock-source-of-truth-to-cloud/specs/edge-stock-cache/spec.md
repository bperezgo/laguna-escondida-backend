## Purpose

Keeps the restaurant selling while the cloud owns stock: the edge decrements on-hand locally so a sale never depends on connectivity, sends every change as a signed movement, and treats the numbers it shows as a daily copy of the cloud's rather than a truth of its own.

## ADDED Requirements

### Requirement: A sale changes stock without consulting the cloud

Selling SHALL never depend on reaching the cloud. The edge SHALL apply a sale's stock effect to its local amounts and SHALL record each effect as a signed movement queued for replication, whether or not the cloud is reachable.

A sale SHALL NOT be blocked, delayed or altered by the state of stock: no on-hand value, and no failure to write one, changes what is sold.

#### Scenario: Selling while offline

- **WHEN** an order is placed while the cloud is unreachable
- **THEN** the sale completes, local on-hand decreases, and a movement is queued for the cloud

#### Scenario: Stock is never a gate

- **WHEN** an order is placed for a product whose local on-hand is zero or negative
- **THEN** the sale completes unchanged

#### Scenario: Queued movements drain in order

- **WHEN** the cloud becomes reachable after an offline period
- **THEN** every queued movement is delivered once, in the order it was produced

### Requirement: The edge sends changes, never amounts

The edge SHALL communicate stock only as signed movements. It SHALL NOT publish its on-hand amount as a value for another node to adopt.

#### Scenario: What replication carries

- **WHEN** the edge's stock changes for any reason
- **THEN** what reaches the cloud is the signed change, and no absolute amount that the cloud could assign

### Requirement: The edge's stock numbers are a daily copy of the cloud's

The edge SHALL refresh its stock amounts from the cloud on a daily schedule, replacing what it holds, so each day opens with the cloud's numbers including whatever the office recorded. Between refreshes the edge's numbers SHALL be understood as a local view that reflects the restaurant's own movements and not the office's.

A failed refresh SHALL be logged, SHALL leave the previous numbers in place, and SHALL NOT affect any other edge function.

#### Scenario: Opening the day

- **WHEN** the daily refresh runs after the office recorded a purchase
- **THEN** the edge's on-hand for that product matches the cloud's

#### Scenario: Drifting within the day

- **WHEN** the office records a correction after the day's refresh
- **THEN** the edge continues with its current numbers and adopts the correction at the next refresh

#### Scenario: Refresh cannot reach the cloud

- **WHEN** the daily refresh fails
- **THEN** the failure is logged, the previous amounts remain readable, and sales are unaffected

#### Scenario: Refresh does not lose queued movements

- **WHEN** a refresh replaces the edge's amounts while movements are still queued for the cloud
- **THEN** those movements are still delivered and still applied by the cloud

### Requirement: Stock is not authored at the restaurant

The edge SHALL NOT offer any way to author an on-hand amount: creating a stock row, adjusting an amount, deleting one and recording a batch count are cloud operations. The edge's only stock writes SHALL be consequences of business events recorded there.

#### Scenario: No authoring endpoints

- **WHEN** a stock write endpoint is called on the edge
- **THEN** the response is `404`

#### Scenario: A delivery recorded at the restaurant

- **WHEN** a purchase entry is recorded on the edge
- **THEN** local on-hand increases and the increase replicates as a signed movement the cloud folds into its own amount

### Requirement: Movement history is read in the cloud

The edge SHALL retain only what it needs to produce and replicate its own movements. Movement history SHALL be a cloud concern; no edge feature SHALL depend on reading past movements.

#### Scenario: Asking the restaurant for history

- **WHEN** a product's movement history is needed
- **THEN** it is read from the cloud, which holds movements from every node
