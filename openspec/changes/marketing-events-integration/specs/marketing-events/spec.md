## REMOVED Requirements

### Requirement: Events are configured through the environment

**Reason**: A per-organization secret in the environment needs an edit and a redeploy for every
restaurant and every rotation, and depends on the slug.

**Migration**: Put each organization's current secret with
`PUT /api/platform/organizations/:id/integrations/marketing_app` (or generate a new one and paste it
into each of its channels in marketing-app), then unset `MARKETING_APP_EVENTS_SECRET_<SLUG>`.
`MARKETING_APP_EVENTS_URL` stays in the environment.

## ADDED Requirements

### Requirement: Events are configured through the organization's marketing-app integration

The events URL SHALL be one environment setting shared by all organizations. An organization's
signing secret SHALL be the one held by its `marketing_app` integration (see
`organization-integrations`). It SHALL be read when an event is recorded and when it is sent, so a
secret put, generated or removed at runtime takes effect on the next event without a restart. An
organization SHALL count as having no secret when it has no `marketing_app` integration, when that
integration cannot be decrypted, or when the cloud has no integrations encryption key. Per-organization
secrets in the environment SHALL NOT be read. When one is set, startup SHALL log a warning that it is
ignored, naming the variable but not its value. At startup, while the URL is set, a warning SHALL be
logged for each active organization without a secret. The edge box SHALL NOT record or send events.

#### Scenario: A restaurant not yet onboarded

- **WHEN** the URL is set and organization `restaurante-x` has no `marketing_app` integration
- **THEN** a startup warning names it, and its agent orders and reservations record no events

#### Scenario: Onboarding without a restart

- **WHEN** the cloud is running and Laguna's admin generates a marketing-app secret, then a WhatsApp reservation is confirmed
- **THEN** a `reservation.confirmed` event is recorded and sent signed with the generated secret

#### Scenario: Rotating the secret

- **WHEN** an event is pending and the secret is generated again before it is sent
- **THEN** the event is sent signed with the new secret

#### Scenario: Removing the integration

- **WHEN** Laguna's `marketing_app` integration is removed and an agent order is paid
- **THEN** the order is paid and no event is recorded

#### Scenario: A leftover environment secret

- **WHEN** the cloud starts with `MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA` set
- **THEN** a warning names the variable as ignored, without its value, and Laguna's secret comes only from its integration

## MODIFIED Requirements

### Requirement: Every event is signed and delivered at least once

Each request SHALL carry `X-Signature-256: sha256=<hex HMAC-SHA256 of the exact body bytes>`, keyed by
the organization's signing secret at the time of the attempt, and the same body bytes SHALL be sent on
every retry. Each request SHALL also carry `X-Channel-Id` with the body's `channel_id`. The header is
not signed: the receiver uses it only to choose which secret to verify with. Delivery SHALL follow
the response:
- `2xx`: delivered, not sent again;
- `400`: rejected, not retried, logged as an error;
- `401`: retried at most hourly, logged as an error;
- `5xx`, `404`, a timeout or a connection error: retried with growing delays of at most 30 minutes;
- not delivered within 72 hours: expired, not retried, logged as an error.

When the organization has no secret at the time of an attempt, its pending events SHALL be left
pending and not sent. They expire after 72 hours like any undelivered event.

Events SHALL NOT be guaranteed to arrive in order.

#### Scenario: The receiver is down

- **WHEN** marketing-app answers `503` three times and then `200`
- **THEN** the event is delivered with the same `event_id` and body each time, and is not sent again

#### Scenario: The signature matches the bytes

- **WHEN** an event is sent
- **THEN** recomputing HMAC-SHA256 over the received body with the organization's secret equals the `X-Signature-256` value

#### Scenario: The channel is named in a header

- **WHEN** an event for channel `ch_123` is sent
- **THEN** the request carries `X-Channel-Id: ch_123`, equal to the body's `channel_id`

#### Scenario: A malformed event

- **WHEN** marketing-app answers `400`
- **THEN** the event is not retried and an error is logged

#### Scenario: Rotated but not yet pasted

- **WHEN** the secret is generated again and marketing-app still holds the old one
- **THEN** events are answered `401`, retried at most hourly, and delivered once the new secret is pasted in marketing-app, within 72 hours
