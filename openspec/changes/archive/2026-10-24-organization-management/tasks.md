## 1. Schema

- [x] 1.1 Add migration `000068_extend_organizations` per design decision 2:
  - required settings `NOT NULL` with Colombia defaults, plus the `status` CHECK
  - optional columns nullable
  - `slug` backfilled (`laguna-escondida` for Laguna, `'org-' || id` otherwise), then `NOT NULL UNIQUE`
  - Laguna's `display_name` = `Laguna Escondida`
  - a down migration that reverses all of it

  Verify `make migrate-up` then `make migrate-down` then `make migrate-up` run clean. Verify Laguna reads back with the backfilled values, and an insert with `(id, name, slug)` gets the Colombia defaults.
- [x] 1.2 Leave the timestamps alone: migration 49 already made them `TIMESTAMPTZ` (this replaces the planned conversion and its production-snapshot check). Verify in the migration test that Laguna's `created_at` is unchanged by up and down and stays `timestamp with time zone`.
- [x] 1.3 `einvoicing_integration_test.go` seeds organizations with `(id, name)`, which now fails on `slug NOT NULL`. **Ask before editing this existing test helper**, then add a slug. Verify `DB_HOST= go test ./internal/platform/postgres/repository/ -run EInvoicing`.

## 2. Domain

- [x] 2.1 Extend `dto.Organization` with every setting and `EInvoicing *EInvoicingSettings`. Add:
  - `CreateOrganizationRequest`, with an optional prefixes block
  - `UpdateOrganizationRequest`, all `*string`
  - `SetEInvoicingRequest`

  Verify `go build ./...`.
- [x] 2.2 Add `domain/error/organization_error.go`:
  - `ErrOrganizationNotFound`
  - `ErrOrganizationSlugTaken`
  - a validation error that carries the field name and reason, a concrete type with no `any`

  Verify `go build ./...`.
- [x] 2.3 Add port `OrganizationRepository` (`Create`, `Get`, `List`, `Update`) and `Upsert` on `EInvoicingSettingsRepository`. `Get`/`List` return organizations with their e-invoicing settings attached. Run `make generate-mocks` and verify the new mocks exist and `go build ./...` passes.
- [x] 2.4 Add `OrganizationService` with `Create`, `List`, `Get`, `Update`, `SetEInvoicing` and the validation functions from the spec. It takes the two repositories and `UnitOfWork`, and generates UUID v7 ids. Confirm the production image has tzdata for `time.LoadLocation`. Verify `go build ./...`.
- [x] 2.5 Add `organization_service_test.go`, table-driven, named `TestOrganizationService_{Method}_{Scenario}`. Cover:
  - **Create:** a minimal create gets the defaults; create with prefixes calls `Upsert` inside the unit of work; equal or lowercase prefixes fail before any write; slug taken; every invalid-field case in the spec (name, slug shapes, status, timezone `America/Bogata`, country, locale, currency, `logo_url` not https, color `blue`, length limits)
  - **Update:** a partial update touches only the given fields; an empty optional field clears it; an empty required field fails; not found
  - **SetEInvoicing:** first configuration; replace; not found; invalid prefixes
  - **List and Get:** pass through, and `Get` not found

  Verify `go test ./internal/domain/service/ -run Organization` passes with ≥90% coverage on the service methods.

## 3. Repository

- [x] 3.1 Add `organization_repository.go` (GORM model ↔ DTO, `GetTxOrDB`):
  - `Create` maps unique violation `23505` on the slug to `ErrOrganizationSlugTaken`
  - `Update` writes only the changed fields and maps cleared ones to NULL
  - `Get` returns `ErrOrganizationNotFound`
  - `List` is ordered by name and loads e-invoicing rows in one extra query

  The constructor returns the port type. Verify with an integration test (testcontainers): create, duplicate slug, partial update, clear a field, get/list with and without e-invoicing.
- [x] 3.2 Implement `Upsert` in `einvoicing_settings_repository.go` (`ON CONFLICT (organization_id) DO UPDATE`). Verify with an integration test that a replace keeps the old prefix's `invoice_sequences` row, and that it works on the request's pinned connection.

## 4. HTTP

- [x] 4.1 Add `organization_handler.go`. It binds the request (the PATCH binding struct includes `Slug *string`, only to reject it), calls the service, and maps errors: validation → 400 with the field, not found → 404, slug taken → 409. Responses: 201 on create, 200 otherwise. Verify with handler tests for each status code.
- [x] 4.2 Register the five routes under `/api/platform/organizations` in `registerCloudRoutes`, behind `AdminAPIKeyMiddleware`, and wire the repository, service and handler in `cmd/main.go` (cloud mode only). Verify `go build ./...` and that the app boots in cloud and edge mode.
- [x] 4.3 Extend the route-guard tests in `cmd/` to assert:
  - the platform routes return 401 with an admin JWT and with the agent JWT
  - they return 401 without `X-API-Key`
  - they are not registered in edge mode
  - `DELETE /api/platform/organizations/:id` is 404

  Verify `go test ./cmd/...`.

## 5. Documentation

- [x] 5.1 Write `docs/api/organizations.md` in the format of `docs/api/products.md`:
  - endpoint table, then per-endpoint request, response and errors with realistic JSON
  - a warning that new organizations can't be used until tenancy steps 2–4 land
  - a warning that changing prefixes restarts numbering

  Add working curl examples to `docs/examples/curl-examples.md`. Verify the examples against a local cloud run.
- [x] 5.2 Update `docs/multi-tenancy-plan.md`:
  - a decisions-log entry: org settings as columns, `/api/platform` routes behind `ADMIN_API_KEY` until `platform_admin`, no delete, immutable slug
  - mark this part of step 9 as done in §8
  - in §4, point the rows for timezone, country, business info and branding at the new columns

  Verify the doc reads consistently.

## 6. Final checks

- [x] 6.1 Run `make lint`, `go build ./...`, `go test ./...` and `DB_HOST= make test-acceptance`, and fix any findings. Verify all pass.
- [ ] 6.2 After deploy, `PATCH` Laguna's business info, receipt text and branding from the current edge env values and frontend constants. Verify that `GET /api/platform/organizations/bb0cad3e-be13-4bad-9a37-bba9525bfece` returns them.

## 7. Organization on `/auth/me`

- [x] 7.1 Add `dto.CurrentOrganization` (the tenant-facing fields: no status, e-invoicing or timestamps) and `organization` on `CurrentUserResponse`. Add `OrganizationService.GetCurrent(ctx, id)` mapping from the stored organization, with service tests (found, not found).
- [x] 7.2 `GetCurrentUserHandler` adds the configured organization (`ORGANIZATION_ID` until step 3); any failure is a `500` without details. Build the organization service in both modes. Handler tests for both outcomes.
- [x] 7.3 Update `docs/api/auth.md`, the `/auth/me` curl example, and `docs/multi-tenancy-plan.md` (decisions log, §6). Verify `/auth/me` against a local cloud and edge run.

  Then rerun 6.1's checks.
