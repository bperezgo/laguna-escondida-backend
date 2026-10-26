## MODIFIED Requirements

### Requirement: The caller determines the organization

Each request SHALL act for exactly one organization, taken from the caller's credential: the `org_id` of a user's token, the organization of an organization API key, or the organization of the calling edge node. The server's configuration SHALL NOT choose the organization in the cloud. A request whose credential names no organization SHALL NOT read or write tenant data.

#### Scenario: Two organizations on one server

- **WHEN** a user of organization A and a user of organization B call `GET /api/products` on the same cloud server
- **THEN** each receives only their own organization's products

#### Scenario: Organization in the request body is ignored

- **WHEN** a user of organization A creates a product, a supplier or an expense, and the request body contains `organization_id` set to organization B
- **THEN** the record is created in organization A

#### Scenario: An API key names its organization

- **WHEN** organization A's API key calls `GET /api/products` on a cloud server that also serves organization B
- **THEN** only organization A's products are returned

### Requirement: An edge box serves only its own organization

An edge box SHALL act only for the organization in its configuration. A token whose `org_id` differs from the box's organization SHALL be rejected with `401 Unauthorized`. An organization API key SHALL be rejected with `401 Unauthorized` on every edge box, including one of the key's own organization.

#### Scenario: Token from another organization on a box

- **WHEN** a request to Laguna's edge box presents a validly signed token whose `org_id` is organization B
- **THEN** the response is `401 Unauthorized`

#### Scenario: An API key on a box

- **WHEN** a request to Laguna's edge box presents Laguna's API key
- **THEN** the response is `401 Unauthorized`
