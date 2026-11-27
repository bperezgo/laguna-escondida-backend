## Context

See proposal.md (Why). What exists today:

- `config/marketing_events.go` loads `MARKETING_APP_EVENTS_URL` and every
  `MARKETING_APP_EVENTS_SECRET_<SLUG>` into a map behind `ports.MarketingEventSecrets`
  (`SecretFor(slug) (string, bool)`). It has three callers, and each one reads the organization only
  to get its slug:
  - `MarketingEventService.organization`, the gate that decides whether to record an event, inside
    the order or reservation transaction;
  - `MarketingEventDeliveryService.DeliverDue`, which signs, per organization, in a tenant context;
  - `CRMScheduler.warnOrganizationsWithoutSecret`, the startup log.
- `organization-integrations` (not yet archived) adds `organization_integrations` (migration
  `000087`, `CHECK (provider IN ('wompi'))`, one AES-GCM ciphertext of a JSON object per row) and
  `OrganizationIntegrationService`. The service already implements `ports.PaymentLinkConfig` as
  `For(ctx, orgID)`. Platform and staff routes are `/:provider`-generic, and `:provider` is checked
  by `service.provider(name)`.
- In `bootstrap/cloud.go`, the marketing services are built (line ~56) **before** the integrations
  service (line ~60).
- marketing-app (other repo) stores a secret per channel in `channel_secret`
  (`(channel_id, 'backend_events.secret')`). `backendSigner` tries every channel's secret until one
  verifies, then requires that channel to equal the body's `channel_id`.

## Goals / Non-Goals

**Goals:**
- The second provider is added the way D1 of `organization-integrations` intended: an allowed
  value, a validator and a typed reader. No new table or route shape.
- Laguna moves over with no lost events and no change in marketing-app.

**Non-Goals:**
- Per-channel secrets on the backend. A nullable `channel_id` on the row (channel row first,
  organization row as fallback) is the path if needed. It is not built now.
- Moving `MARKETING_APP_EVENTS_URL` into the table: it depends on the deployment, not on the
  organization.
- A grace period with two valid secrets during rotation. 401 retries cover the gap (risk below).
- A field on the Ajustes page to type a secret. Importing is an API-only operation.

## Decisions

### D1. Provider `marketing_app`, secrets `{"signing_secret": …}`, environment NULL

Cloud migration `000088` drops and re-adds the check as `provider IN ('wompi', 'marketing_app')`.
`dto.IntegrationProviderMarketingApp` is added. The hint key is `signing_secret_hint`, in the same
`secret_hints` JSONB. The read DTO already allows a null environment. The webhook path is emitted
for Wompi only.

### D2. The typed reader is a separate method, adapted to the port

`ports.MarketingEventSecrets` becomes:

```go
type MarketingEventSecrets interface {
    // SecretFor reports configured=false when the organization has no usable marketing-app integration.
    SecretFor(ctx context.Context, organizationID string) (secret string, configured bool, err error)
}
```

`OrganizationIntegrationService.MarketingAppSecret(ctx, orgID)` implements it under that signature
through a thin adapter (`service.MarketingAppSecrets{svc}`). The method can't be named `For`, which
already belongs to `PaymentLinkConfig`. It follows `For` exactly: nil cipher → `false, nil`;
missing row → `false, nil`; decrypt failure → error log (organization, provider) and `false, nil`.
It reads on every call and keeps no cache. The three callers pass the context's organization id
and stop reading the organization for its slug, except where they still need it (timezone in the
recorder, the list in the startup warning).

*Alternative:* a separate `MarketingAppSecretService`. It would duplicate the cipher and
repository plumbing for one method.

### D3. Generate and put are two operations on the same row

- `POST /:id/integrations/:provider/secret` (platform) and `POST /:provider/secret` (staff,
  `integrations:manage`) call `GenerateMarketingAppSecret`. It reads 32 bytes from `crypto/rand`,
  encodes them with `base64.RawURLEncoding` (43 characters, no prefix: marketing-app treats the
  secret as opaque), encrypts, upserts, and returns the read DTO plus the plaintext. Any provider
  other than `marketing_app` is `404`.
- `PUT /:provider` with `marketing_app` validates `signing_secret` (trimmed, at least 32 characters,
  no whitespace) and upserts. Errors name the field, never the value.
- The generate handler builds its response from a dedicated DTO
  (`GeneratedMarketingAppSecretResponse`), so the standard read DTO still has no secret field. The
  handler test asserts the secret appears only in this response.
- The request logger already must not log PUT bodies (`organization-integrations` 3.3). The POST
  has no body, but its response must not be logged either. Check that the response logger doesn't
  capture bodies.

*Alternative:* a PUT without a body meaning "generate". That overloads one verb with two behaviors
and makes an accidental empty PUT rotate the secret.

### D4. `X-Channel-Id` on every request; marketing-app picks the secret by it

`MarketingEventTransport.Send` takes the channel id and sets `X-Channel-Id`. The outbox has no
`channel_id` column; the delivery reads `channel_id` from the stored body (which this backend wrote,
so reading it before marketing-app verifies is safe). Retries of rows recorded before this deploy
therefore also get the header, with no migration. In
marketing-app, `backendSigner`:
1. with `X-Channel-Id`, loads that channel's `backend_events.secret`. An unknown channel or no
   secret → `401`. It verifies HMAC over the raw bytes with a constant-time compare, then parses
   and requires `body.channel_id == X-Channel-Id`, or `401`;
2. without it, keeps today's trial loop, so the two repos can deploy in either order.

This keeps "verify before parsing". The header is unsigned but only selects a key. A forged header
can only make verification fail.

*Alternatives:* parse `channel_id` out of the body before verifying. That parses unauthenticated
JSON and breaks the documented rule. Or keep the loop but continue on a channel mismatch instead
of answering 401. That works, but it costs O(channels) HMACs per request and stays ambiguous.

### D5. Environment secrets are ignored, loudly, with no fallback

`config/marketing_events.go` keeps the URL and drops the map. At startup it scans `os.Environ()`
for `MARKETING_APP_EVENTS_SECRET_` and warns once per variable name (as in Wompi's D6). No
environment → table fallback: the PUT import (D3) closes the migration gap without keeping two
sources of truth.

### D6. Ajustes card

`frontend/components/settings/MarketingAppIntegrationCard.tsx`, next to `WompiIntegrationCard` in
`IntegrationsPageClient`. Its states:

- **Unavailable:** 503 `INTEGRATIONS_UNAVAILABLE`, the same message as Wompi.
- **Not configured:** the "Generar secreto" button.
- **Just generated:** the secret in a read-only monospace field with "Copiar", a note to paste it
  in each channel under Canal → Ajustes → "Secreto de eventos del sistema de pedidos", and "Listo"
  to dismiss. The secret is held only in component state, never in the URL, storage or logs.
- **Configured:** `••••abcd`, the last update, "Regenerar" (in-page confirmation with the 401
  warning) and "Quitar" (in-page confirmation).

The Next proxy gets `app/api/organization/integrations/[provider]/secret/route.ts` (POST, response
passed through, nothing logged).

## Risks / Trade-offs

- [Rotation gap: marketing-app answers 401 until the secret is pasted on every channel] → The
  backend retries 401s at most hourly, for up to 72h, so events arrive late but are not lost. The
  card warns before regenerating.
- [Deploy gap: from the deploy until the import, Laguna records no events, and those are lost] →
  The migration plan puts the secret right after the deploy. The window is minutes, at Laguna's
  volume.
- [The extra read in the recording transaction] → One indexed row and an AES-GCM open per agent
  order or reservation change. Negligible, and the same trade Wompi made.
- [Generated secret leaks through the browser] → Shown once, in component state only, never logged
  by the proxy. The UI offers regenerating as the remedy.
- [Two repos] → D4's fallback lets backend and marketing-app deploy in either order. Only onboarding
  a two-channel organization waits for marketing-app.
- [Ordering with `organization-integrations`] → This change edits code and requirements that change
  adds. Merge it after that change lands, and archive it second.

## Migration Plan

1. Deploy the backend, with `INTEGRATION_SECRETS_KEY` already set. The startup log warns that
   `MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA` is ignored, and that Laguna has no secret.
2. Right away: `PUT /api/platform/organizations/<laguna>/integrations/marketing_app` with the current
   value. Confirm the next agent event is delivered (2xx).
3. Remove `MARKETING_APP_EVENTS_SECRET_*` from the deployment.
4. Deploy marketing-app's `X-Channel-Id` verification (any time).
5. Optional: regenerate from Ajustes and paste the new secret on Laguna's channel, to prove rotation.

Rollback: revert the backend and restore the environment variable. Integration rows are harmless to
old code, which ignores them.

## Open Questions

- Laguna's current secret must be at least 32 characters for the import. If it's shorter, step 2
  becomes generate + paste instead. Check when you run it.

## Implementation map

| File | Change |
|---|---|
| `backend/internal/platform/postgres/migrations/cloud/000088_organization_integrations_marketing_app.{up,down}.sql` | D1 |
| `backend/internal/domain/dto/organization_integration.go` | provider, `MarketingAppSecret` input, generated response |
| `backend/internal/domain/ports/marketing_events.go` (+ regenerated mock) | D2 port |
| `backend/internal/domain/service/organization_integration_service.go` (+ test) | D2 reader, D3 put/generate, provider list |
| `backend/internal/domain/service/marketing_event_service.go`, `marketing_event_delivery_service.go` (+ tests) | D2 callers, D4 channel to transport |
| `backend/internal/platform/cron/crm_scheduler.go` (+ test) | D2 startup warning |
| `backend/internal/platform/httpclient/marketing_events_client.go` (+ test) | D4 header |
| `backend/internal/platform/config/marketing_events.go` (+ test) | D5 |
| `backend/internal/platform/handler/organization_integration_handler.go` (+ test) | D3 generate handlers, PUT body for `marketing_app` |
| `backend/internal/bootstrap/cloud.go`, `routes_cloud.go`, `testdata/routes_cloud.golden` | build integrations before marketing services; new routes |
| `backend/docs/api/marketing-events.md`, `backend/docs/api/organization-integrations.md`, `backend/docs/examples/curl-examples.md` | config, header, verification, rollout |
| `frontend/types/organizationIntegrations.ts`, `frontend/lib/api/organizationIntegrations.ts` | D6 client |
| `frontend/app/api/organization/integrations/[provider]/secret/route.ts` | D6 proxy |
| `frontend/components/settings/MarketingAppIntegrationCard.tsx`, `IntegrationsPageClient.tsx` (+ tests) | D6 |
| marketing-app: `backend_events.go` (`backendSigner`) + its contract doc | D4 (other repo) |
