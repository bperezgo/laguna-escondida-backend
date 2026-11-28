## ADDED Requirements

### Requirement: A Wompi integration may carry the credentials for the QR

Setting an organization's Wompi integration, on the platform route and on the organization's
route, SHALL accept two optional values beside the private key and the events secret:
`public_key` and `integrity_secret`. They SHALL be given together or not at all. The request
SHALL be refused with `400` naming the field, and nothing stored, when:
- only one of the two is given
- the public key does not start with `pub_test_` or `pub_prod_`, or the integrity secret does not
  start with `test_integrity_` or `prod_integrity_`
- either is for a different environment than the private key

Both SHALL be stored encrypted with the integration's other secrets. Setting the integration
without them SHALL remove any stored before, as setting it replaces the whole integration. Every
read of a Wompi integration SHALL carry a hint of each (its last 4 characters) when present and
`qr_enabled`, true only when both are present. Neither SHALL be returned in plaintext or logged.

#### Scenario: Adding the QR credentials

- **WHEN** Laguna's admin puts sandbox values for the private key, the events secret, `public_key` and `integrity_secret`
- **THEN** the response is `200` with the four hints and `qr_enabled: true`

#### Scenario: Only the public key

- **WHEN** the admin puts the private key, the events secret and a `public_key` with no `integrity_secret`
- **THEN** the response is `400` naming `integrity_secret`, and nothing is stored

#### Scenario: An integration saved before this change

- **WHEN** an organization's Wompi integration holds only a private key and an events secret
- **THEN** it reads `qr_enabled: false`, and its agent orders keep getting payment links

#### Scenario: Mixed environments

- **WHEN** the admin puts `prv_test_…` with a `pub_prod_…` public key
- **THEN** the response is `400` naming `public_key`, and nothing is stored

### Requirement: The Ajustes page sets the QR credentials with the Wompi keys

The Wompi card in Ajustes SHALL offer the public key and the integrity secret as optional inputs,
SHALL show their hints and whether the QR is enabled, and SHALL say that orders are paid with the
link only while the QR is not enabled. It SHALL treat them as it treats the other keys: never
displayed once stored, cleared after saving, and a refused save's reason shown next to the named
field.

#### Scenario: Enabling the QR

- **WHEN** the admin saves the four values
- **THEN** the card shows the QR as enabled with the four hints, and the inputs are empty

#### Scenario: Wompi without the QR

- **WHEN** an organization's Wompi integration has no QR credentials
- **THEN** the card says orders are paid with the link only
