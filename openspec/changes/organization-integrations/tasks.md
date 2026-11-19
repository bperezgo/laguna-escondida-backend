## 0. Before starting

- [x] 0.1 Base this work on `feat/wompi-payment-links` (it changes code that branch adds). Verify `git log` shows the Wompi commits and `make test` passes before the first edit.

## 1. Configuration and storage

- [ ] 1.1 Confirm Wompi's key prefixes against Laguna's sandbox account (private key `prv_test_`, events secret `test_events_`). Note the answer in design's Open Questions and adjust D4's constants if they differ.
- [x] 1.2 `config/integrations.go`: `INTEGRATION_SECRETS_KEY` (base64, 32 bytes; malformed fails, missing means off), loaded from `config/cloud.go`. Verify with config tests: valid, missing, not base64, wrong length.
- [x] 1.3 `config/payment_links.go`: drop the per-slug maps and keep `WOMPI_API_URL` and `PAYMENT_LINK_TTL`. Expose the names of leftover `WOMPI_PRIVATE_KEY_*` / `WOMPI_EVENTS_SECRET_*` variables for the startup warning (D6). Verify with config tests (leftovers listed by name only).
- [x] 1.4 Cloud migration `000087_create_organization_integrations` (D1): unique `(organization_id, provider)`, `provider` check `wompi`, RLS like `payment_links`. Verify with `make migrate-up` / `migrate-down` / `migrate-up`.

## 2. Domain

- [x] 2.1 DTOs (`OrganizationIntegration` without secrets, `WompiCredentials`, provider and environment), errors (not found, unavailable, validation naming the field), the repository port, and `PaymentLinkConfig.For(ctx, organizationID)` (D5). Run `make generate-mocks` and verify that it builds.
- [x] 2.2 `OrganizationIntegrationRepository` (upsert, get, list, delete, by organization under `tenant.WithOrg`). Verify with repository tests: upsert replaces, list ordered by provider, another organization's row is invisible.
- [x] 2.3 `OrganizationIntegrationService`: Wompi validation (D4), encrypt and upsert, read and list with hints and webhook path, delete, `For` (decrypt, and on failure log without the value and return not configured), `ErrIntegrationsUnavailable` without a cipher. Verify with a unit test per scenario in `specs/organization-integrations` (sandbox keys, production on sandbox, mixed, public key, no key, key replaced, local-URL accepts either).
- [x] 2.4 `PaymentLinkService` and `PaymentWebhookService` use `For` with the organization id instead of the slug methods. Verify that the existing payment link and webhook tests pass with the mock reshaped, plus a test that a rotation is used on the next call.

## 3. HTTP and wiring

- [x] 3.1 `OrganizationIntegrationHandler`: `GET /:id/integrations`, `GET|PUT|DELETE /:id/integrations/:provider` on the platform organizations group. Map errors to 400/404/503 `INTEGRATIONS_UNAVAILABLE`. Verify with handler tests, including one asserting that no response body contains either secret.
- [x] 3.2 Bootstrap: a cipher from `INTEGRATION_SECRETS_KEY` when set, the service as `PaymentLinkConfig`, routes registered cloud-only. `logPaymentLinks` reads through `For`, plus the leftover-env warning. Update `testdata/routes_cloud.golden` and verify that the edge routes have none of the integration routes.
- [x] 3.3 Check the HTTP request logger (`handler/middleware.go`) never records request bodies. If it does, exclude the integration routes. Verify by putting keys locally and grepping the logs for the secret.

## 4. Docs

- [x] 4.1 `docs/api/organization-integrations.md` (endpoints, validation, key loss), with configuration updated in `docs/api/wompi-webhook.md` and `docs/api/orders.md` (`PAYMENT_LINKS_NOT_CONFIGURED` now means no Wompi integration), and a curl example for the PUT in `docs/examples/curl-examples.md`. Verify the docs list no `WOMPI_PRIVATE_KEY_` / `WOMPI_EVENTS_SECRET_` except as ignored.

## 4b. Staff routes and the Ajustes page

- [x] 4b.1 Permissions `integrations:read` / `integrations:manage` (D7), granted to admin through `AllPermissions`. Verify with a permissions test that manager, accountant, server, cooker and agent lack both.
- [x] 4b.2 Staff handler methods and the cloud-only `/api/organization/integrations` group (D7). Update `testdata/routes_cloud.golden` and verify the edge has none. Verify with handler tests: admin `200`, manager `403`, agent key `403`, `503` without a key, and no response containing a secret.
- [x] 4b.3 Document the staff routes in `docs/api/organization-integrations.md` (permissions, same bodies and errors) and add a curl example with a JWT.
- [x] 4b.4 Frontend plumbing (D8): `types/organizationIntegrations.ts`, `lib/api/organizationIntegrations.ts` (keeps `status`, `code`, `field`), proxy routes `app/api/organization/integrations/route.ts` and `[provider]/route.ts` that never log bodies, both permissions in `lib/permissions/constants.ts` and `CLOUD_ONLY_PERMISSIONS` (with `edge.test.ts` cases), and the home-nav entry gated by `integrations:read`.
- [x] 4b.5 Frontend page (D8): `app/(main)/ajustes/page.tsx` and the Wompi card in `components/settings/` (unavailable, not configured, configured; read-only without `integrations:manage`; field errors; in-page delete confirmation; inputs cleared after save). Unit-test the state and error-mapping helpers. Run `pnpm exec tsc --noEmit`, `pnpm test` and `pnpm lint` from `frontend/`.

## 5. End to end

- [ ] 5.1 Local cloud with `INTEGRATION_SECRETS_KEY` and Wompi's sandbox URL: put Laguna's sandbox keys, see startup report Laguna configured, create an agent order through the MCP customer toolset and get a `payment_url`. Then replace the keys without a restart and see the next order use them. Then delete the integration and see `PAYMENT_LINKS_NOT_CONFIGURED`. Repeat put → replace → remove from the Ajustes page as Laguna's admin, and check that a manager sees the status but can't change it.
- [ ] 5.2 Run `make test` and `make lint` from `backend/` and confirm both pass. Then do `wompi-payment-links` 5.2/5.3 with keys put through the endpoint instead of the environment.
