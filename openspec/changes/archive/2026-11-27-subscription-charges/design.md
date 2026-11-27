## Context

See `proposal.md` for the motivation. This design assumes `signup-card-step` is implemented. What shapes the approach:

- **`organization_billing`** holds each organization's period and Wompi payment source. `plans.Price` is the only source of an amount. The `BillingProvider` port has `Terms` and `SavePaymentSource`.
- **Access is decided at read time.** `OrganizationAccessService.Check` refuses suspended, then pending, then expired, then plan, from fields read through a cache that every replica refreshes within 60 seconds. No job changes a status when a trial ends.
- **The cloud runs on several replicas**, and each runs its schedulers (`internal/platform/cron`, gocron in singleton mode per process). Nothing today stops two replicas from doing the same work in the same second.
- **Wompi charges a saved source with `POST /v1/transactions`** carrying `payment_source_id`, the amount in cents, a reference and an integrity signature. The answer is usually `PENDING`, and the final status arrives as a `transaction.updated` event or by reading the transaction. Wompi retries an unanswered event after 30 minutes, 3 hours and 24 hours.
- **A webhook for Wompi exists**, per organization, for the restaurants' own payment links (`/api/webhooks/wompi/:organization_id`, `backend/docs/api/wompi-webhook.md`). Its checksum check is reusable; its route and secrets are not, since billing events come from the platform's own Wompi account.
- **A stopped organization's users have no session**, as a pending one's did. The setup token of `signup-card-step` (`aud: "billing-setup"`) is the credential for that case too.
- **The cloud sends no email.**

## Goals / Non-Goals

**Goals:**
- No job is needed for access to be right. Charges change two fields, and access reads them.
- A charge cannot be duplicated by replicas, retries or a crash.
- Everything is testable against the fake Wompi with an injected clock.

**Non-Goals:**
- Plan or period changes by the admin after the trial starts, prorating, refunds.
- Telling the owner about a charge outside the app.
- The electronic invoice for each charge.
- Removing a payment source at Wompi.

## Decisions

### 1. Two new fields decide everything

`organization_billing` gains `paid_until` (null until the first approved charge), `cancelled_at`, and `attempts` for the current due date. The **due date** is derived: `paid_until` when set, else the organization's `trial_ends_at`.

`OrganizationAccessService` reads `paid_until`, `cancelled_at` and whether a payment source exists, next to status and trial end, through the same cache. With `grace = 72h`:

- trial: expired from `trial_ends_at + grace` with a live subscription, from `trial_ends_at` otherwise → `ErrTrialExpired`.
- active with `paid_until`: unpaid from `paid_until + grace` when not cancelled, from `paid_until` when cancelled → new `ErrPaymentRequired`, mapped to `403 payment_required` wherever `trial_expired` is mapped, and to `suspended` for a box.
- active without `paid_until`: as today.

*Alternative considered:* a `past_due` status flipped by the job. Rejected: access would then depend on the job running, and the edge's status constraint would need a release.

### 2. `billing_charges`

Cloud migration: `id`, `organization_id`, `due_on` (the due date it pays for), `attempt` (1 to 3, or 0 for a recovery or new-card charge), `reference` (unique), `plan`, `period`, `amount`, `currency`, `status` (`created | pending | approved | declined | voided | error`), `wompi_transaction_id` (unique when set), `covers_from`, `covers_until`, timestamps.

Two partial unique indexes carry the "never twice" rule: one on `(organization_id, due_on)` where `status = 'approved'`, and one on `(organization_id)` where `status in ('created', 'pending')`.

The reference is `kb-<organization id>-<due_on as yyyymmdd>-<attempt or a short random for attempt 0>`.

### 3. Charging is insert, then call, then mark

`ChargeService.ChargeDue(ctx)` runs every 10 minutes on every replica:

1. Select organizations whose due date has passed, with a payment source, not cancelled, not suspended, with `attempts < 3`, whose retry time has come (due date, +24h, +72h), using `FOR UPDATE SKIP LOCKED` on the billing row.
2. In that transaction, insert the charge as `created` with its reference and increment `attempts`. The partial index refuses a second open charge. Commit.
3. Call Wompi with the reference. On an answer, store the transaction id and set `pending`, or settle at once if the status is final.

A crash between 2 and 3 leaves a `created` row. The lookup job (decision 5) finds it by reference at Wompi: a transaction that exists is adopted, and one that does not after 10 minutes marks the row `error`, which frees the next attempt.

The amount is `plans.Price(plan, period)` read at step 2 and stored on the row, so settlement compares against what was asked, not against a price that changed meanwhile.

### 4. Settlement is one function

`ChargeService.Settle(ctx, transaction)` is called by the webhook and by the lookup. In one database transaction it locks the charge by reference, returns if it is already final, checks amount and currency for an approval, sets the status, and for an approval:

- trial → `active` through `OrganizationService`, with `covers_from = trial_ends_at`.
- active → `covers_from` is the previous `paid_until`.
- recovery charge (attempt 0 on a stopped organization) → `active`, `covers_from = now`, `cancelled_at` cleared.
- `paid_until = covers_until = covers_from + one month or year`, `attempts = 0`.

Then it drops the organization from the access cache on this replica; the others follow within the minute the spec allows.

### 5. Webhook and lookup

`POST /api/webhooks/wompi-billing` verifies the checksum with `BILLING_WOMPI_EVENTS_SECRET`, using the checksum function extracted from the existing webhook handler, and calls `Settle`. It is registered only when the secret is set.

`ChargeService.Reconcile(ctx)` runs every 5 minutes: it reads from Wompi every charge that is `created` or `pending` and older than 10 minutes, by transaction id or by reference, and calls `Settle`.

`BillingProvider` gains `Charge(ctx, req)`, `Transaction(ctx, id)` and `TransactionByReference(ctx, reference)`. The integrity signature uses `BILLING_WOMPI_INTEGRITY_SECRET`. Charging is on only when the public key, private key, events secret and integrity secret are all set; a partial set fails startup.

### 6. The setup token serves recovery

The token is unchanged. `SetupAuthMiddleware` takes the state it requires: `pending` for `/api/billing/setup`, and stopped (expired or unpaid) for `/api/billing/recovery`. `completeSignIn` returns the token with `trial_expired` and `payment_required` for an admin, as it does with `pending_approval`.

Recovery pay reuses `SaveCard`'s payment-source step and `ChargeDue`'s insert-call-mark with attempt 0. The sign-in body is returned by the recovery state read after approval, so the page's polling is also what signs the person in.

### 7. Admin routes use a new permission

`billing:manage`, held by the admin role only, guards `/api/billing`. These are core routes on every plan. Changing the card calls the same service method as the first save, minus `StartTrial`, plus an immediate charge when the due date has passed.

### 8. Frontend

`app/ajustes/suscripcion` with `SubscriptionPageClient`, reusing `CardForm`, `PeriodToggle` and `ChargeSummary` from `signup-card-step`. `app/recuperar` on `PublicLayout` for recovery. `SubscriptionNotice` replaces `TrialBanner`, driven by a pure function in `lib/billing/notice.ts` over what `/auth/me` returns, which gains the billing fields the notice needs. The nav item is filtered by the permission and hidden on a box, like the other cloud-only items.

## Risks / Trade-offs

- **The owner works on the box and may never open the cloud app** → A failed charge is only visible in the cloud notice, and three days later the box stops syncing. Until an email or WhatsApp message exists, the operator watches a log line (`subscription charge failed`) and calls. This is the weakest part of the change and the first follow-up.
- **We owe an electronic invoice for each charge** → Out of scope. The operator invoices by hand from the charges in the platform view. The owner confirms the obligation and the no-IVA rule with the accountant before production keys are set.
- **A price change between attempts** → Each attempt reads the price again, so a retry may ask a different amount than the first try. Acceptable: prices change by release, rarely.
- **Wompi accepts a transaction and never reports it** → The lookup settles it, and a charge still open after 24 hours is logged and counted as failed.
- **A wrong integrity signature or reference format is only found with live sandbox keys** → Task 5.1 checks both before the scheduler is turned on anywhere.
- **Three days of grace is three free days for a card that will never work** → Accepted, to keep a paying restaurant from being locked out by a bank's bad afternoon.

## Migration Plan

1. Cloud migration: new columns on `organization_billing`, the `billing_charges` table and its indexes. Existing organizations have no billing row or no `paid_until`, so nothing changes for them. The down migration drops both.
2. Ship one `cloud/v*` release without the events and integrity secrets: the pages work, nothing is charged.
3. In a test environment set the four sandbox values and Wompi's events address, and run tasks 5.1 and 9.1.
4. Set the production values. From then on every trial with a saved card is charged when it ends. Check before that which trials already have one.
5. Rollback: remove the events or integrity secret, which turns charging off and leaves access rules running on stored dates. Organizations in grace then stop at its end; the operator activates them by hand.

## Implementation map

- **Migration**: `backend/internal/platform/postgres/migrations/cloud/000099_add_billing_charges.{up,down}.sql`
- **Domain**: `internal/domain/dto/billing.go`, `internal/domain/error/billing_error.go`, `access_error.go`, `internal/domain/permissions`, `internal/domain/ports/billing_provider.go`, `billing_repository.go`, `organization_access_reader.go`, `internal/domain/service/charge_service.go` (new), `billing_service.go` (new: read, card, cancel, resume, recovery), `billing_setup_service.go`, `organization_access_service.go`, `organization_service.go`, `user_service.go`, `node_service.go`
- **Adapters**: `internal/platform/httpclient/wompi_client.go`, `internal/platform/postgres/repository/billing_repository.go`, `organization_access_reader.go`, `internal/platform/cache/organization_access_reader.go`, `internal/platform/handler/billing_handler.go` and `billing_webhook_handler.go` (new), the existing Wompi webhook handler (checksum extracted), `middleware.go`, `user_handler.go`, `signup_handler.go`, `internal/platform/cron/charge_scheduler.go` (new), `internal/platform/config/billing.go`, `internal/bootstrap/app.go`, `routes_cloud.go` + `testdata/routes_cloud.golden`
- **Tests**: next to each file; acceptance in `test/acceptance/` with `testsupport/wompi.go` extended for transactions and events
- **API docs**: `backend/docs/api/billing.md`, `auth.md`, `organizations.md`, new `wompi-billing-webhook.md`
- **Frontend**: `app/ajustes/suscripcion/page.tsx`, `app/recuperar/page.tsx`, `app/api/billing/**`, `components/billing/*` (new), `components/layout/TrialBanner.tsx` (replaced by `SubscriptionNotice`), `components/auth/SignInForm.tsx`, `lib/signIn.ts`, `lib/trial.ts`, `lib/billing/*`, `lib/home/navItems.ts`, `lib/permissions/types.ts`, `app/api/auth/me/route.ts`

## Open Questions

- The exact Spanish wording of each screen and notice.
- Whether the log line for a failed charge should also raise a Grafana alert. It needs no code.

## Implementation notes

Decisions made while implementing that the sections above do not state. Each needs a yes or a change before archive.

- **`billing_charges.recovery_sign_in_at`** was added. It is what makes "the sign-in body returned once" true: the recovery state claims it with one conditional update, and only for a charge approved in the last hour.
- **Customer email of a scheduled charge** is the organization's oldest active admin with an email, read in the due query. Recovery and card-change charges use the acting admin's email.
- **Who the job charges:** a `trial` (due on `trial_ends_at`) or an `active` organization that already has `paid_until`. An organization activated by hand is never charged, even with a saved card.
- **Recovery or not** is decided from the organization's state when the charge was created: an attempt-0 charge created while stopped pays from the approval and removes the cancellation; any other pays from its due date.
- **A period's end** keeps the day of the month and stops at the month's last day (31 January → 28 February).
- **Secrets** must start with `test_events_`/`prod_events_` and `test_integrity_`/`prod_integrity_`, of the keys' environment, or startup fails.
- **Charge body** sends `payment_method.installments: 1`, as Wompi's docs show for card sources. `recurrent` is not sent.
- **A refused charge request (4xx)** is stored as `error` at once and counts as an attempt. A failed lookup by reference, a 404 included, never frees a charge; only an answered empty list does.
- **Recovery `card_entry`** is true only when charging is on, and `POST /api/billing/recovery/pay` exists only then.
- **Suscripción in the app** is a settings section, so the sidebar shows it from the same list as the other Ajustes pages.

## Open questions from implementation

- **Extending a trial after failed attempts** does not reset `attempts`, so the new due date is not charged. Reset it when the operator changes `trial_ends_at`?
- **Lookup by reference** (`GET /v1/transactions?reference=`) is not in Wompi's public docs. Task 5.1 must confirm it and its answer shape before the scheduler is enabled.
- **A trial that ended without a card, then pays with a new card within three days,** counts as in grace once the card is saved, so its period runs from the trial's end rather than from the approval. Intended?
