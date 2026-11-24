## MODIFIED Requirements

### Requirement: Background work runs for each organization separately

Work that runs without a request SHALL act for one organization at a time:
- scheduled jobs in the cloud SHALL run once for each active organization, and a failure for one organization SHALL NOT stop or affect the others; the e-invoicing jobs SHALL run only for organizations with e-invoicing configured
- an event SHALL be handled for the organization it was raised in; an event without an organization SHALL be rejected and logged, not handled
- a sync push or pull SHALL act for the organization of the calling node; a node that is not registered, or that does not present the key issued to it, SHALL be rejected
- on an edge box, background work SHALL act for the box's organization

#### Scenario: Scheduled job over two organizations

- **WHEN** the stock reconciliation job runs in the cloud with organizations A and B active
- **THEN** it reconciles A's stock using only A's data and B's stock using only B's data

#### Scenario: One organization's job fails

- **WHEN** a scheduled job fails for organization A
- **THEN** the failure is logged with A's id
- **AND** the job still runs for organization B

#### Scenario: Suspended organization skipped

- **WHEN** a scheduled job runs and organization B is `suspended`
- **THEN** the job does not run for organization B

#### Scenario: Stock event after a purchase entry

- **WHEN** a purchase entry is created in organization A and its event updates stock
- **THEN** only organization A's stock changes

#### Scenario: Sync pull for a node

- **WHEN** Laguna's edge node pulls reference data from the cloud while organization B exists
- **THEN** the pull contains only Laguna's products, users and other reference data

#### Scenario: Unregistered node

- **WHEN** a sync push or pull names a node id not registered in the cloud
- **THEN** the request is rejected with `401 Unauthorized` and nothing is applied

#### Scenario: Another node's key

- **WHEN** a sync push names organization B's node and presents the key issued to organization A's node
- **THEN** the request is rejected with `401 Unauthorized` and nothing is applied for B
