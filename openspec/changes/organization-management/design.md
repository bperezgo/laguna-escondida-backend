## Context

- `organizations` was created by migration 33 with only `id`, `name`, `created_at` and `updated_at` (`TIMESTAMP`). It holds one row, Laguna (`bb0cad3e-…`). There is a `dto.Organization`, but no port, repository, service or endpoint for organizations.
- `organization_einvoicing` (migration 67) stores per-org prefixes. Its port `EInvoicingSettingsRepository` only has `Get`, and the table is written only by migration 67's seed.
- `ADMIN_API_KEY` (`AdminAPIKeyMiddleware`, header `X-API-Key`) is the only credential that isn't tied to a user. It already guards the bootstrap `POST /api/users` and the cloud maintenance routes.
- `UnitOfWork.Do` runs on the request's pinned connection (`DBSessionMiddleware`) and nests as a savepoint.
- `einvoicing_integration_test.go` seeds organizations with `INSERT INTO organizations (id, name)`.
- Requirements are in `specs/organization-management/spec.md`; the motivation is in `proposal.md`.

## Goals / Non-Goals

**Goals:**
- The organization model and endpoints are the ones onboarding (tenancy step 9) will build on, so later steps add to them rather than rework them.
- Nothing that runs today changes behavior.

**Non-Goals:**
- Making any running code read the new columns, except `/auth/me` returning them (decision below). See proposal, "Stored, not yet enforced".
- Row-level security on `organizations`, which stays global per the tenancy plan §2.

## Decisions

**1. Settings are columns on `organizations`, not a separate table or a JSON column.**
- They are one-to-one with the organization and will be read on every `/auth/me` once tenancy step 3 lands.
- Explicit columns keep the repository's types concrete, as AGENTS.md requires (no `any`), and let the database check `NOT NULL`.
- Rejected: an `organization_settings` table (a join for no benefit) and a `branding jsonb` column (untyped).
- E-invoicing stays in its own table because step 5 adds encrypted credentials there.

**2. Migration `000068_extend_organizations`:**
- Add `slug`, the four required settings and the optional columns. Optional columns are nullable `VARCHAR`; the ticket texts are `VARCHAR(1000)`.
- `status`, `timezone`, `country`, `locale` and `currency` are `NOT NULL` with database defaults matching the service defaults (`active`, `America/Bogota`, `CO`, `es-CO`, `COP`). Rows inserted with only `(id, name)`, like migration 33's, still pass.
- Add a `CHECK` on `status IN ('active','suspended')`.
- `slug`:
  - added nullable
  - backfilled: `laguna-escondida` for Laguna's id, `'org-' || id` for any other row (36-char UUID + 4 = 40, valid)
  - then `SET NOT NULL` and `UNIQUE`
  - no default: a slug is a deliberate choice
- Laguna's `display_name` is set to `Laguna Escondida`. Its logo, colors and business info stay NULL and are set with `PATCH` after deploy, because the real values live in edge env vars and frontend constants that the migration can't read.
- `created_at` and `updated_at` become `TIMESTAMPTZ` (`USING … AT TIME ZONE 'UTC'`), matching the newer tables.
- The down migration drops the added columns and constraints and restores `TIMESTAMP`.

**3. A unique-violation error from the database means "slug taken", instead of checking first.**
- The repository maps Postgres `23505` on the slug constraint to `ErrOrganizationSlugTaken`.
- A check-then-insert would race. One insert either succeeds or reports the conflict.

**4. Ids are UUID v7, generated in the service.** This follows the AGENTS.md convention. Laguna's existing v4 id is untouched.

**5. Create runs in one `UnitOfWork`.**
- `OrganizationService.Create` validates everything, including the optional prefixes, before writing anything.
- It then inserts the organization and, if prefixes were given, calls `EInvoicingSettingsRepository.Upsert` inside `UnitOfWork.Do`.
- `PUT …/einvoicing` loads the organization first, so an unknown id returns `404` instead of a foreign-key error, then upserts:

  ```sql
  INSERT … ON CONFLICT (organization_id) DO UPDATE SET invoice_prefix, support_document_prefix, updated_at
  ```

**6. PATCH semantics:**
- The service's `UpdateOrganizationRequest` uses `*string` fields: `nil` means unchanged.
- An empty string clears an optional field (stored as NULL) and is a validation error on a required field.
- The handler's binding struct also has a `Slug *string`, only to reject the field with a clear message. Without it, Gin would silently ignore `slug` and the caller would think it changed.
- Rejected: JSON Merge Patch with explicit `null`. Telling "absent" from `null` apart needs custom unmarshalling, and the empty string already covers clearing.

**7. Validation lives in the service**, as pure functions over the request. This keeps it in the domain and lets the service tests cover every rule.
- Timezone uses `time.LoadLocation`. The production image already embeds tzdata, since `America/Bogota` is loaded today; the implementation checks this rather than assuming it.
- The other fields use the regexes in the spec. ISO country and currency are checked by shape only (`^[A-Z]{2}$` and `^[A-Z]{3}$`), not against a list. Rejected: a full ISO table, overkill for an operator-only endpoint with three tenants.
- Prefixes are checked in the service (`^[A-Z0-9]{1,10}$`, the two must differ) before the database's `CHECK` would catch the "must differ" rule, so the caller gets a `400`, not a `500`.

**8. Routes are under `/api/platform/organizations`, in `registerCloudRoutes`, behind `AdminAPIKeyMiddleware`.**
- The `/platform` prefix marks cross-org routes. Tenancy step 3 swaps the middleware for a `platform_admin` check on the same paths, and adds a tenant-scoped `/api/organization`, without breaking clients.
- The route-guard test in `cmd/agent_routes_test.go` builds the real router, so it must show these routes are unreachable with a JWT, including the agent's.

**9. Response shape:** one JSON object per organization, with every field and `einvoicing: {invoice_prefix, support_document_prefix} | null`.
- The list loads the organizations and then their e-invoicing rows in a second query keyed by id, not one query per organization.
- No pagination: there are a handful of rows.


**10. `/auth/me` returns the running organization** (added after implementation, so the frontend can build its branding and formatting against the real contract):
- A separate `CurrentOrganization` DTO, not `Organization`: the frontend gets settings, not the status, the e-invoicing prefixes or the timestamps.
- The handler asks `OrganizationService.GetCurrent` for the configured `ORGANIZATION_ID`; step 3 swaps that for the token's `org_id` without changing the response.
- A missing organization is a `500`, not `organization: null`: every deployment has its row (migration 33), so a miss is a misconfiguration, and the frontend never has to handle a missing object.
- The edge serves it too, from its own row; its optional settings stay `null` until step 6 syncs them, and the frontend falls back to today's values.

## Risks / Trade-offs

- [Organizations created now can't be used, and someone may try to onboard a restaurant on them] → `docs/api/organizations.md` states that tenancy steps 2–4 must land first. Nothing can sign in to a new organization anyway: there are no memberships yet.
- [`ADMIN_API_KEY` is one shared secret that can now create organizations] → It already creates users, so the exposure is equivalent. It is replaced by `platform_admin` in step 3, on the same paths.
- [Changing prefixes mid-resolution makes invoice numbering restart at 1 for the new prefix, and pending invoices take the new prefix at submission] → Documented in the API doc as a "change only for a new DIAN resolution" warning. This matches how prefixes behave today.
- [Converting the timestamps reinterprets existing values as UTC] → One row, and the RDS server runs in UTC. Verified against a production snapshot as part of the migration task.
- [Status `suspended` is stored but not enforced] → Spec'd explicitly ("Stored settings do not yet change runtime behavior"), so it is visible rather than a surprise. Enforced in step 3.

## Migration Plan

1. Deploy: migration 68 runs at boot, on edge and cloud. It only adds columns, and no application code reads `organizations` today; it is referenced only by foreign keys.
2. After deploy, `PATCH` Laguna's business info and branding from the current edge env values and frontend constants.
3. Rollback: the down migration drops the columns. The endpoints are new and have no dependents.
