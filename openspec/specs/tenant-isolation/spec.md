# tenant-isolation Specification

## Purpose
Guarantees that organizations sharing one deployment and one database never see or change each other's data: every request and every background job acts for exactly one organization, determined by who is calling, and the database itself refuses access outside it.

## Requirements

### Requirement: Tenant data belongs to one organization

Every row of tenant data SHALL belong to exactly one organization. Tenant data is everything except organizations themselves and roles: users and their roles, products and their ingredients and preparation responsibilities, bills and their products and owners, open bills and their products and side dishes, pending invoices, invoice sequences, stock and stock history, suppliers and supplier catalogs, purchase entries and their items, expenses and expense categories, support documents and their products, delivery zones, idempotency keys, nodes, and sync outbox, inbox and state. After the migration, every existing row SHALL belong to Laguna (`bb0cad3e-be13-4bad-9a37-bba9525bfece`).

#### Scenario: Existing data after the migration

- **WHEN** the migration has run on a database with existing data
- **THEN** every tenant row belongs to Laguna
- **AND** every endpoint returns the same data for Laguna's users as before the migration

#### Scenario: Identifiers unique only within an organization

- **WHEN** organization B creates a bill owner with the same document number as one of Laguna's, or an expense category with the same code
- **THEN** it is created in organization B
- **AND** Laguna's bill owner or expense category is unchanged

### Requirement: The caller determines the organization

Each request SHALL act for exactly one organization, taken from the caller's credential: the `org_id` of a user's token, or the organization of the calling edge node. The server's configuration SHALL NOT choose the organization in the cloud. A request whose credential names no organization SHALL NOT read or write tenant data.

#### Scenario: Two organizations on one server

- **WHEN** a user of organization A and a user of organization B call `GET /api/products` on the same cloud server
- **THEN** each receives only their own organization's products

#### Scenario: Organization in the request body is ignored

- **WHEN** a user of organization A creates a product, a supplier or an expense, and the request body contains `organization_id` set to organization B
- **THEN** the record is created in organization A

### Requirement: No access across organizations

A request acting for organization A SHALL NOT read, create, update or delete any data of organization B, through any endpoint, MCP tool, sync push or sync pull. A reference to another organization's record SHALL behave as if the record did not exist.

#### Scenario: Reading another organization's record by id

- **WHEN** a user of organization A calls `GET /api/products/:id` with the id of organization B's product
- **THEN** the response is `404 Not Found`

#### Scenario: Updating another organization's record by id

- **WHEN** a user of organization A calls `PUT` or `DELETE` on organization B's supplier by id
- **THEN** the response is `404 Not Found`
- **AND** organization B's supplier is unchanged

#### Scenario: Linking to another organization's record

- **WHEN** a user of organization A creates an order whose lines reference organization B's product ids
- **THEN** the order is rejected as referencing products that do not exist
- **AND** nothing is stored in either organization

#### Scenario: Listing users

- **WHEN** a manager of organization A calls `GET /api/admin/users`
- **THEN** only organization A's users are returned

### Requirement: The database enforces isolation

Isolation SHALL be enforced by the database in addition to the application: a database session acting for organization A SHALL be unable to read or write organization B's rows even if the application issues a query without an organization filter. A database session with no organization set SHALL see no tenant rows and be unable to write any. The application SHALL connect as a role that is subject to this enforcement, and the cloud SHALL refuse to start when connected as a role that bypasses it.

#### Scenario: Query without the application filter

- **WHEN** a query on a tenant table runs with organization A set and no organization condition
- **THEN** it returns only organization A's rows

#### Scenario: Query with no organization set

- **WHEN** a query on a tenant table runs with no organization set
- **THEN** it returns no rows, and an insert fails

#### Scenario: Cloud connected as a privileged role

- **WHEN** the cloud starts connected as the tables' owner or a role that bypasses row-level security
- **THEN** it refuses to start and logs why

### Requirement: Run mode and organization configuration are consistent

The run mode SHALL come from the binary (`cmd/cloud` or `cmd/edge`), not from `APP_MODE`. An edge box SHALL require `ORGANIZATION_ID` and SHALL refuse to start without it. The cloud SHALL NOT read `ORGANIZATION_ID` or `NODE_ID`: a leftover `ORGANIZATION_ID` SHALL NOT pin its requests to one organization. The cloud peer key Laguna's edge box keeps its sync state under SHALL NOT change.

#### Scenario: Cloud with a leftover organization

- **WHEN** the cloud starts with `ORGANIZATION_ID` set
- **THEN** it starts, and a signed-in user of a different organization still sees only their own organization's data

#### Scenario: Edge without an organization

- **WHEN** an edge box starts without `ORGANIZATION_ID`
- **THEN** it refuses to start and logs why

#### Scenario: Laguna's box keeps syncing after the release

- **WHEN** the cloud is deployed with this change and Laguna's edge box pushes and pulls with its existing configuration
- **THEN** sync continues from where it was, without re-sending or skipping changes

### Requirement: An edge box serves only its own organization

An edge box SHALL act only for the organization in its configuration. A token whose `org_id` differs from the box's organization SHALL be rejected with `401 Unauthorized`.

#### Scenario: Token from another organization on a box

- **WHEN** a request to Laguna's edge box presents a validly signed token whose `org_id` is organization B
- **THEN** the response is `401 Unauthorized`

### Requirement: Background work runs for each organization separately

Work that runs without a request SHALL act for one organization at a time:
- scheduled jobs in the cloud SHALL run once for each active organization, and a failure for one organization SHALL NOT stop or affect the others; the e-invoicing jobs SHALL run only for organizations with e-invoicing configured
- an event SHALL be handled for the organization it was raised in; an event without an organization SHALL be rejected and logged, not handled
- a sync push or pull SHALL act for the organization of the calling node; a node that is not registered SHALL be rejected
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

- **WHEN** a sync push or pull presents the sync key with a node id not registered in the cloud
- **THEN** the request is rejected with `401 Unauthorized` and nothing is applied

### Requirement: Admin-key operations name their organization

Requests authenticated only by the admin API key carry no organization. In the cloud, such a request that reads or writes one organization's tenant data SHALL name the organization explicitly and SHALL be rejected with `400 Bad Request` when it does not; a maintenance trigger that mirrors a scheduled job SHALL run for each active organization, as the job does. Platform organization endpoints are unaffected.

#### Scenario: Stock opening balances without an organization

- **WHEN** `POST /api/stock/opening-balances` is called with the admin API key in the cloud without naming an organization
- **THEN** the response is `400 Bad Request` and nothing is written

#### Scenario: Manual document-URL update

- **WHEN** `POST /api/invoices/update-missing-document-urls` is called with the admin API key in the cloud
- **THEN** missing document URLs are updated for each active organization with e-invoicing configured

### Requirement: Stored files are kept per organization

Files the system stores (purchase-entry and expense attachments, invoice and support documents) SHALL be stored under the organization they belong to, taken from the request's organization.

#### Scenario: Attachment storage location

- **WHEN** a user of organization B uploads an expense attachment
- **THEN** the file is stored under organization B's id
