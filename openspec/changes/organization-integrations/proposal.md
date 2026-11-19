## Why

`wompi-payment-links` reads each organization's Wompi private key and events secret from
`WOMPI_PRIVATE_KEY_<SLUG>` and `WOMPI_EVENTS_SECRET_<SLUG>` (its D7, a stopgap). Every new restaurant,
and every key rotation, then means editing the cloud's environment and restarting it, and the setup
is tied to the slug. That doesn't hold for a platform that onboards restaurants. It has to be fixed
before agent orders are deployed, because the first production keys would otherwise go into the
environment and need moving later.

## What Changes

- New cloud table `organization_integrations`: one row per organization and provider, holding the
  provider's secrets encrypted with AES-256-GCM (the existing `crypto` cipher). `wompi` is the only
  provider for now. Other providers (e-invoicing credentials, today global `ELECTRONIC_INVOICE_*`)
  are added later by extending the provider list. They don't need a new table.
- The platform operator configures the integration through the organization endpoints, behind the
  admin API key like API keys and e-invoicing prefixes:
  `PUT|GET|DELETE /api/platform/organizations/:id/integrations/wompi` and
  `GET /api/platform/organizations/:id/integrations`. Secrets are write-only: reads return a hint
  (the last characters), the Wompi environment and when the integration was last changed.
- The organization's own **admin** can set, replace and remove its Wompi integration from a new
  cloud-only **Ajustes** page in the ops UI, through staff routes `GET /api/organization/integrations`
  and `GET|PUT|DELETE /api/organization/integrations/:provider` (the organization comes from the
  token). New permissions `integrations:read` and `integrations:manage`, granted to the admin role
  only. The platform routes stay for onboarding by curl or MCP; both call the same service.
- A Wompi private key and events secret are validated on save: their `test`/`prod` prefixes must
  agree with each other and with the cloud's `WOMPI_API_URL` (sandbox or production).
- Payment links read the organization's credentials from the table, keyed by organization id
  rather than slug. "Payment links configured" now means the organization has a Wompi integration.
- **BREAKING (config)**: `WOMPI_PRIVATE_KEY_<SLUG>` and `WOMPI_EVENTS_SECRET_<SLUG>` are no longer
  read. If one is still set, startup warns that it is ignored. A new `INTEGRATION_SECRETS_KEY`
  (32 bytes, base64) encrypts the secrets. Without it, integrations cannot be saved or read, and agent
  orders are refused, as they are today without keys. `WOMPI_API_URL` and `PAYMENT_LINK_TTL` are
  unchanged.
- The startup log keeps reporting, for each active organization, whether payment links are
  configured, now from the table.

## Capabilities

### New Capabilities
- `organization-integrations`: per-organization credentials for third-party providers, stored
  encrypted, set and removed by the platform operator or the organization's admin, never returned
  in plaintext.

### Modified Capabilities
- `payment-links` (introduced by `wompi-payment-links`, not yet archived): the requirement
  "Payment links are configured per organization through the environment" is replaced. The
  configuration comes from the organization's Wompi integration.

## Impact

- **Ordering**: this change builds on `wompi-payment-links` and must be archived after it. Its
  delta replaces a requirement that only exists once `wompi-payment-links` is archived.
- Backend: a cloud migration, the `crypto` cipher reused, a new repository/service/handler,
  `ports.PaymentLinkConfig` reshaped (by organization id, with a context and an error),
  `PaymentLinkService` and `PaymentWebhookService` updated, `config/payment_links.go` reduced to
  the shared settings, bootstrap wiring and routes, and the startup log.
- Backend (staff access): the two permissions, a staff route group on the same handler and service.
- Frontend: an `/ajustes` page with an Integraciones section, Next API proxy routes, a `lib/api`
  client and types, the two permissions in the constants and in the edge's cloud-only list, and a
  home-nav entry. Backend and frontend ship as one PR (repo rule).
- Docs: `docs/api/wompi-webhook.md` (configuration), `docs/api/orders.md`, a new
  `docs/api/organization-integrations.md` (platform and staff routes), and the curl examples.
- Not affected: the edge box (cloud only; the page is hidden there), and
  marketing-app (it never sees Wompi keys; its L2 contract setup moves from env vars to the new
  endpoint).
