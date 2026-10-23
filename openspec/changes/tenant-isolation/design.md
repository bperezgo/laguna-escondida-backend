## Context

- No business table has `organization_id`; only `invoice_sequences` and `organization_einvoicing` (migration 67) do. `organizations` (migrations 33, 68) is global and already has an immutable, unique `slug` (Laguna: `laguna-escondida`).
- `DBSessionMiddleware(db, cfg.OrganizationID)` is registered globally **before** authentication (`cmd/main.go:308`) and pins a connection with `app.org_id` set to the env value for every request (`postgres/session.go`).
- `GetTxOrDB` uses the request's transaction, then the pinned session, then the plain pool. A query on the plain pool has no `app.org_id`.
- `pos_app` (migration 65) is a NOLOGIN, NOBYPASSRLS group role with default privileges on future tables. Production still connects as the owner.
- `config.Load` requires `ORGANIZATION_ID` in both modes and derives `NODE_ID` and `CLOUD_NODE_ID` from it (`config.go:175-206`).
- Services receive the org as a constructor argument: `PurchaseEntryService`, `ExpenseService`, `SupportDocumentService`, `InvoiceSubmissionService`, `SupportDocumentRepository`, `UserHandler`.
- `users.username` is globally `UNIQUE` (migration 15). The JWT carries the user id and role ids, not the org. Sign-in is `POST /api/auth/signin`.
- Background work runs without a request: four crons (`cron/scheduler.go`), Watermill subscribers (`cmd/event_subscriptions.go`, given a `Detach`ed context), and the cloud's sync push appliers and pull. On the edge, the push and pull loops run against the box's own database.
- Requirements: `specs/tenant-isolation/spec.md`, `specs/organization-sign-in/spec.md`. Motivation: `proposal.md`.

## Goals / Non-Goals

**Goals:**
- No code path reads or writes tenant data without an org from a trusted source (a token, the box's config, an org record looped over), and the database refuses it if the code forgets.
- One code path in both modes: the edge differs only in where the org is checked.
- Laguna keeps working through every intermediate release.

**Non-Goals:**
- Making a second organization fully operable. Per-org e-invoicing credentials, per-node sync keys, the SSE hub keyed by org and org API keys are separate changes (proposal, "Out of scope"). This change only guarantees those gaps fail closed or stay Laguna-only, never leak.

## Decisions

**1. The org comes from the principal, through a domain package.**
- `domain/tenant` exposes `WithOrg(ctx, orgID) context.Context` and `OrgFrom(ctx) (string, bool)`. Services already take `ctx` first, so the domain needs no platform import.
- Auth middleware validates the JWT, rejects it without `org_id`, and calls `tenant.WithOrg`.
- `DBSessionMiddleware(db)` drops its `orgID` parameter, reads `tenant.OrgFrom`, and opens a session only when an org is present. It is attached inside the authenticated route groups, after auth, instead of globally. SSE routes still skip it.
- Rejected: keeping the global middleware and overwriting `app.org_id` later in the request. Any statement issued between the two would run as the wrong org.

**2. Mode comes from `APP_MODE`, never from whether `ORGANIZATION_ID` is set.**
- Edge: `ORGANIZATION_ID` required. Auth additionally rejects a token whose `org_id` differs from it.
- Cloud: boot fails if `ORGANIZATION_ID` is set, so a leftover setting can't silently pin every request to one org. Boot also fails if `NODE_ID` is unset, since it can no longer be derived.
- The cloud's `NODE_ID` is set to the value derived today, `SHA1("laguna-escondida/node/bb0cad3e-…/cloud")`, which is also the edge's default `CLOUD_NODE_ID`, so the box's `sync_state` stays valid. The edge keeps deriving its ids from its org.
- Rejected: inferring the mode from the env var's presence. A misconfigured cloud would become a leak instead of a boot failure.

**3. One organization per user; sign-in resolves the slug first.**
- `users.organization_id NOT NULL`, `UNIQUE(organization_id, username)`. `user_roles` keeps its shape plus the column. The plan's `memberships` tables are not built.
- Sign-in: look up `organizations` by slug (global, no RLS) → reject unknown or `suspended` with the same opaque `401` as a bad password → open a session for that org → query `users` under normal RLS → issue a JWT with `org_id`.
- On the edge, `organization` is optional; when present it must equal the box's slug. One frontend build works in both places.
- Because the org is known before the user is read, `users` takes the same RLS policy as every other table. Rejected: a `SECURITY DEFINER` lookup function, which option A (global usernames) would have needed.

**4. Schema in one migration, RLS in a later one.**
- Migration A, per table: add `organization_id` with `DEFAULT` Laguna's id → `SET NOT NULL`, keeping the default so the Release 1 code, which doesn't set the column, can still insert; swap the uniques; add composite unique `(organization_id, id)` on parents and composite foreign keys `(organization_id, parent_id)` on the main relations (plan §2). Indexes lead with `organization_id`.
- Release 2 drops the defaults in its own migration, shipped with the repositories that stamp the org from the context.
- Migration B: `ENABLE` and `FORCE ROW LEVEL SECURITY` with the policy `organization_id = current_setting('app.org_id', true)::uuid` for `USING` and `WITH CHECK`, on every tenant table. `organizations` and `roles` stay global.
- They ship in separate releases (see Migration Plan), so RLS is enabled only after the leak suite passes with the app scopes.
- Rejected: a per-table rolling migration. More releases with no extra safety, since nothing reads the column until the scopes land.

**5. Repositories: an explicit scope, and creates stamped from the context.**
- A helper in `platform/postgres`, e.g. `tenantDB(ctx, db) (*gorm.DB, error)`, returns `GetTxOrDB(ctx, db).Where("organization_id = ?", org)` and errors with a domain error when the context has no org. Every tenant repository method goes through it.
- Models gain `OrganizationID`; creates set it from `tenant.OrgFrom`, never from a DTO. DTOs do not gain the field.
- Raw SQL (e.g. `claimConsecutive`, the sync appliers' upserts) takes the org from the context the same way.
- Rejected: a GORM callback that injects the filter globally. It's invisible at the call site and easy to bypass with `Raw`; the explicit scope plus RLS gives both readability and a backstop.

**6. Background work runs per org.** Under RLS, a query without `app.org_id` returns no rows, so every path without a request must set an org or it silently stops working for Laguna.
- **Crons (cloud):** each job lists the active organizations (global table) and runs once per org with `tenant.WithOrg` and a session for it; one org's failure is logged and doesn't stop the others. The e-invoicing jobs only run for orgs with an `organization_einvoicing` row. The provider credentials stay global until the credentials change, so only Laguna may have that row in production until then.
- **Event subscribers:** payloads gain `organization_id`; the subscriber restores it with `tenant.WithOrg` on its detached context and opens its own session. A payload without it is rejected and logged.
- **Sync push and pull (cloud):** the node authenticated by `NODE_SYNC_KEY` sends its node id, and the cloud resolves it through `nodes.organization_id`; appliers and the pull run in that org's session, so the pull's users list is filtered by RLS. An unknown node is rejected. The shared key stays until the per-node keys change, which is why only Laguna's box may be registered until then.
- **Edge:** background work uses the box's `ORGANIZATION_ID`. That is the one legitimate use of the configured org.
- Rejected: running background work as the table owner (RLS bypass). It brings back "no org means everything".

**7. Services read the org from the context.**
- Constructors lose their `organizationID` argument; methods call `tenant.OrgFrom(ctx)` and return a domain error when it's missing (S3 keys, e-invoicing settings, consecutives, support documents).
- `/auth/me` returns the token's organization instead of `cfg.OrganizationID`; the response shape is unchanged (decision of 2026-10-22).

**8. Leak suite.**
- `test/acceptance/tenancy`, on testcontainers, seeds orgs A and B with mirrored data and, acting as A, asserts that no endpoint, sync push or pull, cron run or event handler returns or changes anything of B.
- It runs twice: once as built, and once against repository integration tests with the app scope disabled and RLS on, to prove the backstop works on its own.
- It connects as a `pos_app` login role, not as the owner, or RLS would be silently skipped.

## Risks / Trade-offs

- [A repository call site is missed by the scope] → RLS returns no rows instead of another org's; the leak suite covers every endpoint; a lint or grep check in CI flags `GetTxOrDB` on tenant models outside `tenantDB`.
- [A background path is missed and stops working for Laguna once RLS is on] → Decision 6 enumerates them; the RLS release goes to staging with a production snapshot first, and the crons and the edge sync are watched before it reaches production.
- [Production still connects as the owner, which bypasses RLS] → the RLS release is gated on `DB_USER` pointing at a `pos_app` login user; boot logs and fails in cloud mode if the current role has `BYPASSRLS` or owns the tables.
- [Usernames can collide across orgs] → intended; sign-in always includes the org. Existing usernames are unique, so the backfill can't violate the new constraint.
- [Everyone is signed out at release] → announced; the release goes out outside service hours.
- [Composite foreign keys lock rows during the migration on large tables (`historic_stock`, `bill_products`)] → rehearse on a production snapshot and time it; add the constraints `NOT VALID` and `VALIDATE` them separately if the lock is too long.
- [Only Laguna may have e-invoicing or an edge box until the follow-up changes] → stated in the proposal; onboarding a second org with either is blocked by those changes, not by this one.

## Migration Plan

1. **Release 1, schema:** migration A. The backfill makes every row Laguna's and the default keeps new rows Laguna's; nothing reads the column. Rehearsed on a production snapshot first.
2. **Release 2, auth, context and scopes, together with the frontend:** the new sign-in body; the JWT with `org_id`; `DBSessionMiddleware` after auth; boot checks; the cloud task definition drops `ORGANIZATION_ID` and sets `NODE_ID`; repositories, services and background work take the org from the context; a migration drops the Laguna defaults; the leak suite gates the release. Everyone signs in again.
3. **Release 3, RLS:** ops creates the `pos_app` login user and points `DB_USER` at it (`DB_MIGRATION_USER` at the owner); migration B enables RLS. Staging first, with the suite also run with a scope removed on purpose.
4. **Rollback:** release 3 is undone by migration B's down (disable RLS) without touching data. Release 2 is undone by redeploying release 1 with `ORGANIZATION_ID` restored; tokens issued in between are rejected by the old code, so everyone signs in again. Migration A's down drops the columns; it is safe only while one org exists.
