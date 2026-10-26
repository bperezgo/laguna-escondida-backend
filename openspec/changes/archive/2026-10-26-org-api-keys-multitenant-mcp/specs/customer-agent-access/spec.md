## ADDED Requirements

### Requirement: The customer MCP server acts for the caller's organization

With the customer toolset selected, the MCP server SHALL NOT sign in or hold a credential of its own. Every request to `/mcp` SHALL carry `Authorization: Bearer lgk_…`, an organization's API key. A request with no bearer, or with a bearer that is not an `lgk_` key (including a user's token), SHALL be refused with `401 Unauthorized` before any tool runs. Every backend call a tool makes SHALL carry the bearer of the MCP request that made it, and no other credential. The server SHALL therefore act, within one deployment, for whichever organization the calling key belongs to. Calls made for different keys SHALL NOT share credentials or results.

A key the backend refuses SHALL make the tool call fail with a tool error saying the credential was refused, without retrying under another credential.

Startup with the customer toolset SHALL NOT require `MCP_AUTH_TOKEN`, `LAGUNA_EMAIL`, `LAGUNA_USERNAME` or `LAGUNA_PASSWORD`, and SHALL ignore them. The full toolset's authentication SHALL be unchanged.

#### Scenario: Two restaurants on one server

- **WHEN** `list_menu` is called on one customer MCP server with organization A's key, and then with organization B's key
- **THEN** the first call returns only A's products and the second only B's

#### Scenario: Orders land in the caller's organization

- **WHEN** `create_delivery_order` is called with organization B's key
- **THEN** the order is created in organization B and is not visible to organization A's key

#### Scenario: No key

- **WHEN** a request to `/mcp` on the customer server has no `Authorization` header
- **THEN** the response is `401 Unauthorized` and no backend request is made

#### Scenario: A staff token instead of a key

- **WHEN** a request to `/mcp` on the customer server bears a manager's JWT
- **THEN** the response is `401 Unauthorized` and no backend request is made

#### Scenario: A revoked key

- **WHEN** `get_my_orders` is called with a key that has been revoked
- **THEN** the tool returns an error saying the credential was refused, and no other credential is tried

#### Scenario: Old configuration is not needed

- **WHEN** the server starts with `MCP_TOOLSET=customer` and none of `MCP_AUTH_TOKEN`, `LAGUNA_EMAIL`, `LAGUNA_USERNAME` or `LAGUNA_PASSWORD` set
- **THEN** it starts and serves `/mcp`
