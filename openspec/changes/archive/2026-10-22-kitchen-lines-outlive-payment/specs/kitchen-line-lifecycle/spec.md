## Purpose

Keeps the kitchen working from what was ordered, not from whether it was paid: a line's preparation state continues after its order is paid, paid and voided orders are told apart, and every line records when it started and finished so prep times are real.

## ADDED Requirements

### Requirement: A closed order is either paid or voided

Paying an order SHALL close it and record the instant it was paid. Voiding (deleting) an order SHALL close it without a paid instant. An order closed before this behavior existed SHALL be treated as voided for every kitchen purpose.

#### Scenario: Paying records the payment

- **WHEN** an order is paid
- **THEN** it leaves the active order list, a bill exists for it, and the order records the instant it was paid

#### Scenario: Voiding records no payment

- **WHEN** an active order is deleted
- **THEN** it leaves the active order list and records no paid instant

#### Scenario: Historic closed orders

- **WHEN** the kitchen board is loaded after this change is deployed
- **THEN** no line of an order closed before the deployment appears on it

### Requirement: Kitchen lines continue after their order is paid

A line of a paid order SHALL stay on the kitchen board of its area under the same rules as a line of an open order, until every line of that order in that area is completed or cancelled. A line of a voided order SHALL NOT appear on the kitchen board.

The completed review for an area SHALL include completed lines of paid orders, under the same rules as open orders.

#### Scenario: A table pays before its food is ready

- **WHEN** an order has a line `in_progress` in the kitchen area and the order is paid
- **THEN** the line is still on the kitchen board with state `in_progress`

#### Scenario: The last line is finished after payment

- **WHEN** the last unfinished line of a paid order in an area is completed
- **THEN** the order's lines leave that area's board and appear in that area's completed review

#### Scenario: A voided order

- **WHEN** an order with unfinished lines is deleted
- **THEN** none of its lines appear on the kitchen board

### Requirement: Only forward kitchen transitions are allowed on a paid order

On a paid order's line, setting it in progress, completing it and uncompleting it SHALL behave exactly as on an open order. Cancelling a line of a paid order SHALL be refused with a conflict error, and the line SHALL be unchanged.

On a voided order's line, every transition SHALL fail as not found.

#### Scenario: Completing a line after payment

- **WHEN** a line of a paid order is completed
- **THEN** the request succeeds and the line is `completed`

#### Scenario: Undoing a completion after payment

- **WHEN** a completed line of a paid order is uncompleted
- **THEN** the request succeeds and the line is back on the board as `created`

#### Scenario: Cancelling a line after payment

- **WHEN** a line of a paid order is cancelled
- **THEN** the request fails with a conflict error and the line keeps its state

#### Scenario: A voided order's line

- **WHEN** a line of a voided order is completed
- **THEN** the request fails as not found

### Requirement: A paid order stays closed

Changing a paid order's line state SHALL NOT reopen the order: it SHALL NOT reappear in the active order list, and it SHALL NOT become editable, payable again or deletable. It SHALL keep appearing in the closed orders of its business day.

#### Scenario: Line activity on a paid order

- **WHEN** a paid order's line is completed
- **THEN** the order is still absent from the active order list and still present in the closed orders of its day

#### Scenario: Paying twice

- **WHEN** pay is requested for an order that is already paid
- **THEN** the request fails as not found and no second bill exists

### Requirement: Lines record when preparation started and finished

A line SHALL record the instant it was first set in progress and the instant it was completed. Completing a line that was never set in progress SHALL record the completion and leave the start empty. Uncompleting a line SHALL clear its completion. Moving an in-progress line back to `created` SHALL clear its start. These instants SHALL NOT change for any other write to the line or its order.

The completion time reported by the kitchen board and the completed review SHALL be the recorded completion instant.

#### Scenario: Normal preparation

- **WHEN** a line is set in progress at 12:00 and completed at 12:14
- **THEN** it records a start of 12:00 and a completion of 12:14

#### Scenario: Completed directly

- **WHEN** a `created` line is completed at 12:10
- **THEN** it records a completion of 12:10 and no start

#### Scenario: Undo a completion

- **WHEN** a completed line is uncompleted
- **THEN** it has no completion instant and keeps its start

#### Scenario: A later edit does not move the completion

- **WHEN** a line completed at 12:14 is touched by a later write to its order at 12:30
- **THEN** its completion is still 12:14

### Requirement: Replication preserves payment, closure and line timing

When an order is replicated to a peer, the peer SHALL reproduce whether it is open, paid or voided, its paid instant, and each line's start and completion instants. Replicating a change to a paid order's lines SHALL NOT reopen the order on the peer. A replicated change from a node that does not send these fields SHALL be applied as before this change.

#### Scenario: A line finished on the edge after payment

- **WHEN** the restaurant completes a line of an already-paid order and the change replicates to the cloud
- **THEN** the cloud shows the line completed with its completion instant, and the order is still closed and paid in the cloud

#### Scenario: A node that predates the change

- **WHEN** the cloud receives an order snapshot without closure or timing fields
- **THEN** it applies the snapshot exactly as it did before this change
