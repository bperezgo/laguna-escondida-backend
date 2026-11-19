## REMOVED Requirements

### Requirement: Payment links are configured per organization through the environment

**Reason**: Per-organization Wompi keys in the environment need an edit and a restart for every
restaurant and every rotation, and depend on the slug.

**Migration**: Put each organization's keys with
`PUT /api/platform/organizations/:id/integrations/wompi` (see `organization-integrations`) and unset
`WOMPI_PRIVATE_KEY_<SLUG>` and `WOMPI_EVENTS_SECRET_<SLUG>`. `WOMPI_API_URL` and `PAYMENT_LINK_TTL`
stay in the environment.

## ADDED Requirements

### Requirement: Payment links are configured per organization through its Wompi integration

The Wompi API URL SHALL be one environment setting shared by all organizations. An organization
SHALL have payment links configured only when it has a Wompi integration whose secrets can be
decrypted (see `organization-integrations`). Its links SHALL be created and deactivated with that
integration's private key, and its Wompi events SHALL be verified with that integration's events
secret, read when they are used, so a change takes effect without a restart. Per-organization Wompi
keys in the environment SHALL NOT be read. When one is set, startup SHALL log a warning that it is
ignored, naming the variable but not its value. At startup the cloud SHALL log, for each active
organization, whether it has payment links configured. How long a link lives SHALL be a setting
defaulting to 30 minutes. The edge box SHALL NOT create links or receive Wompi events.

#### Scenario: A restaurant with Wompi

- **WHEN** Laguna has a Wompi integration
- **THEN** its agent orders get payment links created with that integration's private key

#### Scenario: Keys rotated without a restart

- **WHEN** the operator replaces Laguna's Wompi integration while the cloud is running
- **THEN** Laguna's next link is created with the new private key and its next event is verified with the new events secret

#### Scenario: No integration

- **WHEN** organization `restaurante-x` has no Wompi integration
- **THEN** it does not have payment links configured, and startup says so

#### Scenario: Leftover environment keys

- **WHEN** the cloud starts with `WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA` set and Laguna has no Wompi integration
- **THEN** startup warns that `WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA` is ignored, and Laguna does not have payment links configured
