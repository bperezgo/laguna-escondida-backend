# edge-sync-refusal-backoff Specification

## Purpose
Keeps edge boxes the cloud refuses (suspended or expired organizations, revoked nodes, wrong keys) from costing the cloud anything that grows with their number, and keeps the boxes the cloud accepts from all calling at the same instant.

## Requirements

### Requirement: A node refusal tells the box when to come back

Every response in which the cloud refuses a node on a node-authenticated route (`/api/sync/*` and `/api/telemetry/*`) SHALL carry a `Retry-After` header with a whole number of seconds. This covers invalid node credentials (`401`), a revoked node (`401`, `node_revoked`) and a suspended organization (`403`, `organization_suspended`).
- The value SHALL come from cloud configuration and default to `21600` (6 hours).
- On `/api/sync/*`, status codes and bodies SHALL stay as they are today.
- On `/api/telemetry/*`, every refusal SHALL answer `429 Too Many Requests` with the same body it carries today, so the box's telemetry collector treats it as retryable and waits.
- The reason SHALL still be told only to a caller holding the node's correct key.

#### Scenario: Suspended organization

- **WHEN** a box of a suspended organization calls `GET /api/sync/pull` with its correct node key
- **THEN** the response is `403` with code `organization_suspended`
- **AND** it carries `Retry-After: 21600`

#### Scenario: Wrong key

- **WHEN** a request names an existing node with a wrong key
- **THEN** the response is `401` with `{"error": "invalid node credentials"}` and a `Retry-After` header
- **AND** it does not say whether the node is revoked or its organization suspended

#### Scenario: Refused telemetry

- **WHEN** a box of a suspended organization posts to `/api/telemetry/v1/metrics` with its correct node key
- **THEN** the response is `429` with code `organization_suspended` and `Retry-After: 21600`

#### Scenario: Operator changes the delay

- **WHEN** the cloud is configured with a retry delay of 1 hour
- **THEN** every node refusal carries `Retry-After: 3600`

#### Scenario: Accepted node

- **WHEN** a box of an active organization calls a sync route with its correct key
- **THEN** the response carries no `Retry-After` header

### Requirement: Repeated refusals do not reach the database

After the cloud refuses a node, it SHALL answer that node's next requests on node-authenticated routes from memory, without a database query, for up to 5 minutes.
- An answer from memory SHALL be the same refusal the database would produce for the same key. A request with a different key SHALL still get the opaque invalid-credentials answer.
- Requests naming a node id that does not exist SHALL also be answered from memory once seen. The number of remembered nodes SHALL be bounded, so random node ids cannot grow the cloud's memory without limit.
- A change to a remembered node or its organization (reactivation, suspension, revocation, a new enrollment) SHALL therefore take effect on node authentication within 5 minutes.

#### Scenario: Expired box keeps calling

- **WHEN** a refused box sends 100 requests to `POST /api/telemetry/v1/logs` within 5 minutes
- **THEN** each is refused with the same status, body and `Retry-After`
- **AND** at most the first one queries the database

#### Scenario: Organization reactivated

- **WHEN** the operator sets a suspended organization back to `active`
- **THEN** its box's requests are accepted again no later than 5 minutes afterwards

#### Scenario: Flood of unknown node ids

- **WHEN** requests arrive naming a large number of distinct, non-existent node ids
- **THEN** each is refused with `401`
- **AND** the cloud's memory for remembered refusals stays within its fixed bound

### Requirement: A refused box pauses its sync

When the cloud refuses the box on any sync call (push, pull or stock pull) with invalid credentials, `node_revoked` or `organization_suspended`, the box SHALL stop making sync calls until the pause ends.
- The pause SHALL last the refusal's `Retry-After`, bounded to between 1 minute and 24 hours.
- A missing or unreadable `Retry-After` SHALL be treated as 6 hours.
- When the pause ends, the box SHALL make one sync call. A new refusal starts a new pause, and a success resumes the normal schedule.
- The pause SHALL be held in memory only. After the edge service restarts, the box makes its first scheduled call as usual.
- Failures that are not refusals (network errors, timeouts, `5xx`) SHALL NOT pause the box. The box keeps its current retry behavior for them.

#### Scenario: Expired box

- **WHEN** the box's pull is refused with `403 organization_suspended` and `Retry-After: 21600`
- **THEN** the box makes no push, pull or stock-pull call for the next 6 hours
- **AND** it makes one call when the 6 hours have passed

#### Scenario: Excessive delay is bounded

- **WHEN** a refusal carries `Retry-After: 999999`
- **THEN** the box pauses for 24 hours

#### Scenario: Refusal without a delay

- **WHEN** a refusal carries no `Retry-After`
- **THEN** the box pauses for 6 hours

#### Scenario: Restart during a pause

- **WHEN** the edge service restarts while paused
- **THEN** the box makes its next scheduled sync call
- **AND** if refused, it pauses again

#### Scenario: Cloud unreachable

- **WHEN** a pull fails because the cloud cannot be reached
- **THEN** the box is not paused and tries again on its next scheduled run

### Requirement: A refused box's telemetry waits too

When the cloud refuses the box's telemetry with `429` and `Retry-After`, the box's telemetry collector SHALL send no further telemetry request until that delay has passed, and SHALL then retry. Telemetry produced in the meantime SHALL stay within the existing bounded local buffer, and what exceeds it is discarded as today.

#### Scenario: Expired box's telemetry

- **WHEN** the cloud answers the box's telemetry with `429` and `Retry-After: 21600`
- **THEN** the box sends no telemetry request for the next 6 hours
- **AND** its telemetry buffer does not grow past its configured cap

### Requirement: A paused box loses nothing

While paused, the box SHALL keep selling. It SHALL keep every unsynced change queued locally, and the POS SHALL keep showing the blocked status it shows today. When a later sync call succeeds, the queued changes SHALL be pushed as usual.

#### Scenario: Sales during a pause, then reactivation

- **WHEN** a paused box records sales, and later its organization is reactivated and its next call succeeds
- **THEN** the sales recorded during the pause are pushed to the cloud

### Requirement: Boxes spread their sync calls

Each box SHALL start its scheduled sync calls at an offset of its own, instead of at the exact scheduled second.
- The offset SHALL be stable for a given node across restarts.
- For loops that run every minute, the offset SHALL be under 45 seconds.
- For the daily stock pull, the offset SHALL be under 15 minutes.
- Offsets SHALL be spread across these ranges, so different boxes generally get different offsets.

#### Scenario: Many boxes on the default schedule

- **WHEN** many boxes run the default every-minute pull
- **THEN** their pull calls are spread over the first 45 seconds of each minute rather than all arriving at second 0

#### Scenario: Same box after a restart

- **WHEN** a box restarts
- **THEN** its sync calls keep the same offset as before the restart
