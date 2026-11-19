## Context

See proposal.md (Why). What exists today:

- `internal/platform/config/payment_links.go` loads `WOMPI_PRIVATE_KEY_<SLUG>` /
  `WOMPI_EVENTS_SECRET_<SLUG>` into maps behind `ports.PaymentLinkConfig`
  (`ConfiguredFor`, `PrivateKeyFor`, `EventsSecretFor`, all by slug, with no context and no error).
  The callers are `PaymentLinkService.Configured`/`CreateForOrder`/deactivation (by `org.Slug` after
  reading the organization) and `PaymentWebhookService` (by the path's organization id, then slug).
  `bootstrap/cloud.go:logPaymentLinks` reports per active organization at startup.
- Staff notifications already encrypt per-organization secrets: `ports.SecretCipher`,
  `platform/crypto/aes_gcm_cipher.go` (`0x01|nonce|ciphertext`), with the key from
  `NOTIFICATION_SECRETS_KEY` (32 bytes, base64, `config/staff_notifications.go`). The
  `notification_channels` table stores `secret BYTEA` + `secret_hint`.
- Platform organization routes (`/api/platform/organizations`, `routes_cloud.go`) sit behind
  `AdminAPIKeyMiddleware`. `APIKeyService.Issue` shows the pattern for a cross-org write:
  `requireOrganization`, then `tenant.WithOrg(ctx, orgID)`.
- `wompi-payment-links` is implemented on `feat/wompi-payment-links` but not archived (0.1, 3.1,
  5.2 and 5.3 open). No production Wompi keys exist yet.

## Goals / Non-Goals

**Goals:**
- One table and one service for provider credentials, so the next provider is a new allowed value,
  a validator and a typed reader, not a new table.
- Credentials changed at runtime take effect on the next use, without a restart or a cache to
  invalidate.

**Non-Goals:**
- Managers changing keys: they get neither integration permission.
- Other settings on the new Ajustes page (branding, ordering pause): it starts with integrations only.
- A full webhook URL on the page: the backend doesn't know its public address, so the page shows the
  path to copy.
- Moving e-invoicing credentials (`ELECTRONIC_INVOICE_*`) into the table.
- Wompi's public key and integrity secret: payment links and webhooks need neither.
- Rotating `INTEGRATION_SECRETS_KEY` (re-encrypting rows). Losing it means putting the keys again.

## Decisions

### D1. One row per (organization, provider), with all of a provider's secrets in one ciphertext

`organization_integrations(id, organization_id → organizations, provider CHECK IN ('wompi'),
environment VARCHAR(16) NULL, secrets BYTEA, secret_hints JSONB, created_at, updated_at)`, with a
unique index on `(organization_id, provider)`. `secrets` encrypts a JSON object
(`{"private_key": …, "events_secret": …}`), so a provider with a different set of secrets needs no
schema change. Hints and the environment stay in plaintext because they are what reads return.
Cloud migration `000087`, with RLS like the other tenant tables (as `payment_links` in 000086).

*Alternatives:* a row per secret (`provider`, `name`, `ciphertext`): this needs a multi-row upsert to
keep a pair consistent, and Wompi's two values are only useful together. Columns per provider: the
table would change with every provider.

### D2. Its own key, `INTEGRATION_SECRETS_KEY`, through the existing cipher

`config/integrations.go` loads it like `NOTIFICATION_SECRETS_KEY` (base64, 32 bytes, a malformed
value fails startup, missing means off). Bootstrap builds a second `crypto.NewAESGCMCipher` for it.

*Alternative:* reuse `NOTIFICATION_SECRETS_KEY`. That would be one less setting, but it ties
payment credentials to a feature flag named after notifications and makes their rotations one event.
A shared, renamed key can come later.

### D3. Write-only secrets through the platform routes

`OrganizationIntegrationHandler` on the existing `organizations` group:
`GET /:id/integrations`, `GET|PUT|DELETE /:id/integrations/:provider`. `:provider` outside the
allowed list is `404`. `OrganizationIntegrationService` validates per provider (D4), encrypts, and
upserts with `tenant.WithOrg`. Responses are a DTO without `secrets`. When the cipher is absent (no
key), the service returns `ErrIntegrationsUnavailable`, mapped to `503 INTEGRATIONS_UNAVAILABLE`.
Secrets are never logged: request logging must not record the PUT body (check the HTTP request
logger), and validation errors name the field, not the value.

### D4. Wompi validation from the key prefixes

The private key must start with `prv_test_` or `prv_prod_`, and the events secret with
`test_events_` or `prod_events_`. Both must be the same environment (`test` → `sandbox`,
`prod` → `production`). Against `WOMPI_API_URL`: host `sandbox.wompi.co` requires `sandbox` and
host `production.wompi.co` requires `production`. Any other host (an `httptest` fake or a local
stand-in) accepts either. This catches the common mistake (production keys on the sandbox, or a
public key pasted) at save time instead of at the first order.

### D5. `PaymentLinkConfig` becomes a per-organization lookup

```go
type PaymentLinkConfig interface {
    // For reports configured=false when the organization has no usable Wompi integration.
    For(ctx context.Context, organizationID string) (creds dto.WompiCredentials, configured bool, err error)
}
```

It is implemented by `OrganizationIntegrationService` (read the row, decrypt, unmarshal). It reads on
every use, without a cache: one indexed row and an AES-GCM open per order, link change or webhook,
which is negligible at this volume. Callers stop needing the slug. `PaymentLinkService` uses the
context's organization id, and the webhook uses the path's id (it still reads the organization to
404 an unknown one and to run in its context). A decrypt failure logs an error (organization,
provider, no value) and returns `configured=false, err=nil`, so an order is refused with
`PAYMENT_LINKS_NOT_CONFIGURED` rather than a 500 (spec: treated as absent).

*Alternative:* load everything at startup into a map. That is what the environment did, and a
rotation would then need a restart or an invalidation path.

### D6. Environment keys are ignored, loudly

`config/payment_links.go` keeps `WOMPI_API_URL` and `PAYMENT_LINK_TTL` and drops the per-slug maps.
At startup it scans `os.Environ()` for the two old prefixes and warns once per variable name. There
is no fallback from environment to table: no production keys exist yet, so supporting both sources
would only add a second source of truth.

### D7. Staff routes on the same service

A cloud-only group `/api/organization/integrations` behind `JWTAuthMiddleware`:
`GET ""` and `GET /:provider` need `RequirePermission(integrations:read)`, `PUT /:provider` and
`DELETE /:provider` need `integrations:manage`. The handler passes the token's organization
(`tenant.RequireOrg`) to the existing service methods, so validation, encryption and errors are the
platform routes' (including `503 INTEGRATIONS_UNAVAILABLE`). The admin role gets both permissions
through `AllPermissions()`; no other role gets either, so an agent API key (agent role) is `403`.
The platform routes keep the admin API key for onboarding.

*Alternative:* staff-only routes. Onboarding would then need a restaurant admin to log in before the
operator can test anything.

### D8. The Ajustes page

`app/(main)/ajustes` with an "Integraciones" section holding a Wompi card. Cloud-only: both
permissions are in the frontend's `CLOUD_ONLY_PERMISSIONS`, so on the edge the nav entry and page are
hidden. The card has three states:

- **Unavailable** (`503 INTEGRATIONS_UNAVAILABLE`): "Las integraciones no están habilitadas en este servidor."
- **Not configured**: a form with two `type="password"`, `autocomplete="off"` fields, never prefilled
  and cleared after submit. A `400` with `field` is shown under that field; one without is shown as
  the form's message (environment mismatch).
- **Configured**: environment badge (Sandbox / Producción), hints as `••••wxyz`, last update, the
  webhook path with a copy button, "Reemplazar llaves" (opens the form) and "Quitar" (in-page
  confirmation, no `window.confirm`).

Without `integrations:manage` the card is read-only. The Next proxy routes pass the body through and
never log it.

## Risks / Trade-offs

- [Key loss: `INTEGRATION_SECRETS_KEY` changed or lost] → Every organization stops taking agent
  orders (refused, not broken). The log names each organization, and the operator puts the keys
  again. This is documented in `docs/api/organization-integrations.md`.
- [Secrets in logs through the request logger] → Task 3.3 verifies the PUT body is not logged.
  The handler test asserts no response contains either secret.
- [Keys typed in the browser] → Password fields, never echoed back by any response, never logged
  by the Next proxy or the backend request logger; handler tests assert no response carries a secret.
- [Wompi prefixes change or differ by account] → The prefix check is the only coupling to Wompi's
  key format. If sandbox keys don't match (task 1.1), relax D4 to a warning instead of guessing.
- [Ordering with `wompi-payment-links`] → This change edits code that change added and replaces its
  D7. Merge it onto `feat/wompi-payment-links` (or after it lands), and archive it second.

## Migration Plan

1. Deploy with `INTEGRATION_SECRETS_KEY` set and no Wompi integrations: every organization refuses
   agent orders, as before.
2. Put Laguna's sandbox keys with the PUT. Run `wompi-payment-links` 5.2/5.3 against it.
3. Put Laguna's production keys on the production cloud. Configure the webhook path the GET returns
   in Wompi's dashboard.
4. Remove any `WOMPI_PRIVATE_KEY_*` / `WOMPI_EVENTS_SECRET_*` from the deployment (the startup
   warning lists them).

Rollback: delete the integration (agent orders refused again). Reverting the code needs the old
environment keys back.

## Open Questions

- Wompi's exact prefixes for the events secret (`test_events_` / `prod_events_`) and the private key
  (`prv_test_` / `prv_prod_`): confirm against Laguna's sandbox account in task 1.1. Only D4's
  constants depend on it.

## Implementation map

| File | Change |
|---|---|
| `internal/platform/postgres/migrations/cloud/000087_create_organization_integrations.{up,down}.sql` | D1 |
| `internal/platform/config/integrations.go` (+ test), `config/cloud.go` | D2 key |
| `internal/platform/config/payment_links.go` (+ test) | D6 |
| `internal/domain/dto/organization_integration.go` | integration, `WompiCredentials`, provider/environment enums |
| `internal/domain/error/` (integration errors) | not found, unavailable, validation |
| `internal/domain/ports/organization_integrations.go`, `ports/payment_links.go` | repository port, D5 |
| `internal/platform/postgres/repository/organization_integration_repository.go` (+ test) | D1 |
| `internal/domain/service/organization_integration_service.go` (+ test) | D3, D4, D5 |
| `internal/domain/service/payment_link_service.go`, `payment_webhook_service.go` (+ tests) | D5 callers |
| `internal/platform/handler/organization_integration_handler.go` (+ test) | D3 |
| `internal/bootstrap/cloud.go`, `routes_cloud.go`, `testdata/routes_cloud.golden` | wiring, routes, startup log |
| `docs/api/organization-integrations.md`, `docs/api/wompi-webhook.md`, `docs/api/orders.md`, `docs/examples/curl-examples.md` | docs |
| `internal/domain/permissions/permissions.go` (+ test) | D7 permissions |
| `internal/platform/handler/organization_integration_handler.go` (+ test), `bootstrap/routes_cloud.go`, golden | D7 staff routes |
| `frontend/types/organizationIntegrations.ts`, `frontend/lib/api/organizationIntegrations.ts` | D8 client |
| `frontend/app/api/organization/integrations/route.ts`, `…/[provider]/route.ts` | D8 proxy |
| `frontend/lib/permissions/constants.ts`, `edge.ts` (+ test), `frontend/app/home/page.tsx` | D8 permissions, nav |
| `frontend/app/(main)/ajustes/page.tsx`, `frontend/components/settings/` | D8 page |
