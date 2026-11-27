## 0. Before starting

- [x] 0.1 Base this work on `organization-integrations` once it has landed on `main` (it adds the table, service, handler and Ajustes page this change extends). Verify `git log` shows migration `000087` and `make test` passes from `backend/` before the first edit.

## 1. Storage and configuration

- [x] 1.1 Cloud migration `000088_organization_integrations_marketing_app` (D1): provider check widened to `('wompi', 'marketing_app')`; the down migration deletes `marketing_app` rows and restores the old check. Verify with `make migrate-up` / `migrate-down` / `migrate-up`.
- [x] 1.2 `config/marketing_events.go` (D5): keep `MARKETING_APP_EVENTS_URL`, drop the secrets map, and expose the names of leftover `MARKETING_APP_EVENTS_SECRET_*` variables. Verify with config tests (URL valid/invalid/unset, leftovers listed by name only, never value).

## 2. Domain

- [x] 2.1 DTOs and port (D1, D2): `IntegrationProviderMarketingApp`, the `signing_secret` input, the generated-secret response DTO, and `ports.MarketingEventSecrets.SecretFor(ctx, organizationID) (string, bool, error)`. Run `make generate-mocks` and verify that it builds.
- [x] 2.2 `OrganizationIntegrationService` (D2, D3): `marketing_app` accepted by `provider()`, `SetMarketingApp` (trim, ≥32 chars, no whitespace, error names `signing_secret`), `GenerateMarketingAppSecret` (32 bytes `crypto/rand`, raw URL base64, returns plaintext once), `MarketingAppSecret` reader and its `MarketingAppSecrets` adapter, hints/null environment/no webhook path in reads, generate for any other provider → not found. Verify with a unit test per scenario in `specs/organization-integrations` ADDED/MODIFIED requirements (generate, generate twice differs, put existing, too short, generate for Wompi, no cipher, key replaced logs `marketing_app` without the value).
- [x] 2.3 `MarketingEventService` and `MarketingEventDeliveryService` use `SecretFor(ctx, orgID)` (D2), and the delivery passes each event's channel id to the transport (D4). Verify existing service tests pass with the reshaped mock, plus tests for: no integration → nothing recorded; rotation before sending → new secret signs; no secret at send time → left pending.
- [x] 2.4 `CRMScheduler.warnOrganizationsWithoutSecret` reads through the port per active organization, and startup logs one warning per leftover environment variable (D5). Verify with scheduler/bootstrap tests (warning names the org; leftover warning names the variable).

## 3. HTTP and wiring

- [x] 3.1 `MarketingEventsClient.Send` sets `X-Channel-Id` (D4). Verify with the client test against `httptest` (header equals body `channel_id`, signature unchanged).
- [x] 3.2 Handler (D3): `PUT /:provider` decodes `{"signing_secret"}` for `marketing_app`; new `POST /:provider/secret` on the platform group and on the staff group behind `integrations:manage`. Verify with handler tests: 200 with `signing_secret` only on POST, GET/PUT/list responses never contain the secret, 400 naming the field, 404 for Wompi/unknown, 403 for manager and agent key, 503 without key.
- [x] 3.3 Confirm the request/response logger does not record the POST response body or the PUT body (D3). Verify by a handler or middleware test, or note the code path that guarantees it.
- [x] 3.4 `bootstrap/cloud.go`: build the integrations service before the marketing services and pass the adapter as their secrets; `routes_cloud.go` adds the routes; update `testdata/routes_cloud.golden`. Verify `go test ./internal/bootstrap/...` passes and the golden diff shows only the two POST routes.

## 4. Frontend

- [x] 4.1 Types, `lib/api` client (`generateMarketingAppSecret`, list/get/delete reuse) and the Next proxy `app/api/organization/integrations/[provider]/secret/route.ts` (POST, passthrough, no logging) (D6). Verify with `pnpm test` for the client and `pnpm lint`.
- [x] 4.2 `MarketingAppIntegrationCard` with the four states (unavailable, not configured, just generated with copy + paste note + Listo, configured with hint, Regenerar and Quitar in-page confirmations), read-only without `integrations:manage`, rendered in `IntegrationsPageClient` next to Wompi. Verify with component tests per scenario in "The Ajustes page generates the marketing-app secret and shows it once", plus `pnpm lint` and `pnpm build` from `frontend/`.

## 5. Docs

- [x] 5.1 `backend/docs/api/marketing-events.md`: configuration table (secret from the integration, leftover variable ignored), `X-Channel-Id` header, verification by channel then raw bytes, the same-secret-on-every-channel note, rotation behavior (401 retried hourly), rollout per design's Migration Plan. `backend/docs/api/organization-integrations.md` and `docs/examples/curl-examples.md`: `marketing_app` PUT, POST secret, GET, DELETE. Verify by reading them against the specs.

## 6. marketing-app (other repo)

- [ ] 6.1 In marketing-app's `backendSigner` (D4): with `X-Channel-Id`, verify only that channel's `backend_events.secret` (constant-time) over the raw body, then require `body.channel_id` to match, else 401; without the header, keep today's loop. Update its contract doc. Verify with its tests: two channels sharing one secret both accept their own events; a forged header fails; no header behaves as before. Track it as a separate PR in that repo and link it here.

## 7. Verification

- [x] 7.1 Local cloud with `INTEGRATION_SECRETS_KEY` and `MARKETING_APP_EVENTS_URL` pointed at a stand-in (or marketing-app locally): with `MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA` set, see the ignored-variable warning; PUT the same value and confirm an agent reservation event is delivered with a matching signature and `X-Channel-Id`; generate from Ajustes as admin, see the 401s until the stand-in has the new value, then delivery; remove and see no event recorded. All without restarting.
- [ ] 7.2 Run `make test` and `make lint` from `backend/`, and `pnpm lint`, `pnpm test`, `pnpm build` from `frontend/`; confirm all pass.
