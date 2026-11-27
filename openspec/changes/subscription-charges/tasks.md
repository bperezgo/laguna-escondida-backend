Backend commands run from `backend/`, frontend commands from `frontend/`. Read only the files in design "Implementation map". `signup-card-step` must be implemented and archived first. Acceptance tests need the `DB_HOST=` prefix to use testcontainers. Every time-dependent test uses the injected clock.

## 1. Storage

- [x] 1.1 Add migration `000099_add_billing_charges` (design decisions 1 and 2). Verify with an acceptance test: a second `approved` charge for the same organization and due date is refused, a second open charge for one organization is refused, a duplicate reference is refused, and the down migration removes the table and columns.
- [x] 1.2 Extend the billing repository: due organizations with `FOR UPDATE SKIP LOCKED`, insert and update of charges, read by reference and by transaction id, open charges older than a duration. Verify: an acceptance test runs two concurrent claims of one due organization and asserts only one gets it.

## 2. Access rules

- [x] 2.1 Carry `paid_until`, `cancelled_at` and the saved-card flag through the access reader and its cache. Verify: the reader's tests return them, and a change is seen after the cache's refresh.
- [x] 2.2 Apply design decision 1 in `OrganizationAccessService`, add `ErrPaymentRequired`, and map it to `403 payment_required` in the middleware and to `suspended` in `node_service.go`. Verify with unit tests for every scenario in the `organization-plans` delta, including an active organization without `paid_until` that never stops.
- [x] 2.3 Make the operator's `active` clear `paid_until` and cancel the subscription for a stopped organization. Verify: a service test for the "Operator reactivates a lapsed organization" scenario.
- [x] 2.4 Return the setup token with `trial_expired` and `payment_required` for an admin, and let `SetupAuthMiddleware` require a state. Verify: tests for the `organization-sign-in` delta, and middleware tests that a stopped organization's token reaches `/api/billing/recovery` and gets `409 not_pending` on `/api/billing/setup`, and the reverse for a pending one with `409 not_stopped`.

## 3. Provider

- [x] 3.1 Extend `config/billing.go` with the events and integrity secrets. Verify with unit tests: all four values → charging on, public and private only → card entry on and charging off, any other partial set → startup error.
- [x] 3.2 Add `Charge`, `Transaction` and `TransactionByReference` to `BillingProvider` and the Wompi adapter. Verify with `httptest` tests: the request carries the payment source, amount in cents, `COP`, reference, email and a signature that matches the integrity secret; `PENDING`, `APPROVED` and `DECLINED` answers map to their statuses; an unknown reference returns not found.

## 4. Charging and settling

- [x] 4.1 Add `ChargeService.Settle` (design decision 4). Verify with unit tests for every scenario of "A charge is settled by what Wompi reports", "The first charge is made when the trial ends" and "A paid period is renewed on its last day", including a renewal approved late and a second report of the same transaction.
- [x] 4.2 Add `ChargeService.ChargeDue` (design decision 3) with the retry times. Verify with unit tests over a fake provider and clock: charges on the due date, +1 day and +3 days; none while one is open; none after three failures; none for a cancelled, suspended or card-less organization; the amount stored is the price at that moment.
- [x] 4.3 Add `ChargeService.Reconcile` (design decision 5). Verify with unit tests: a `pending` charge older than 10 minutes is settled from the lookup, a `created` row with a transaction at Wompi is adopted, one without is marked `error`, and one open for 24 hours is logged and counted as failed.
- [x] 4.4 Add the scheduler for both jobs and wire it in `app.go`, started only when charging is on. Verify: a scheduler test like the existing ones, and the app starts with and without the secrets.
- [x] 4.5 Extract the checksum function from the existing Wompi webhook handler and add `POST /api/webhooks/wompi-billing`. Verify with handler tests: a signed approval settles, a wrong checksum answers `401`, another event type and an unknown reference answer `200` unchanged, the route is absent without the secret, and the existing webhook's tests still pass.
- [x] 4.6 Extend `testsupport/wompi.go` with transactions and events, and add acceptance tests: trial ends → charge → event → active and paid; declined three times → stopped with `trial_expired` after grace; two replicas' `ChargeDue` at once → one transaction at the fake. Verify: they pass with the acceptance target.

## 5. Wompi sandbox check (needs sandbox keys)

- [ ] 5.1 With the four sandbox values, charge a saved test source by hand and receive its event on a public address. Record in design.md under "Sandbox findings": the exact transaction body and signature that worked, the statuses seen and how long they took, the event's `signature.properties`, and whether a recurring flag or installments field is required. Verify: the findings are written, and any that contradict design decisions 3 to 5 have updated them before the scheduler is enabled anywhere.

## 6. Admin and recovery routes

- [x] 6.1 Add the `billing:manage` permission for the admin role, `BillingService.Read` and `GET /api/billing`. Verify: handler tests for the two scenarios of "An admin reads the subscription", and that no payment source identifier is in the body.
- [x] 6.2 Add `GET /api/billing/card-setup` and `PUT /api/billing/card`, with the immediate charge when the due date has passed. Verify: tests for both scenarios of "An admin changes the card" and for the shared limit of 5 attempts.
- [x] 6.3 Add cancel and resume. Verify: tests for both scenarios, `409 subscription_lapsed` and `409 no_subscription`.
- [x] 6.4 Add the three recovery routes (design decision 6). Verify: tests for every scenario of "A stopped account can pay its way back", including `409 charge_pending` and the sign-in body returned once after approval.
- [x] 6.5 Add the billing fields the notice needs to `/auth/me`. Verify: a handler test for an admin and for a manager, who gets only what the trial banner uses today.
- [x] 6.6 Update `routes_cloud.golden`, then run `make lint` and `make test`. Verify: both pass.

## 7. API docs

- [x] 7.1 Update `billing.md`, `auth.md` and `organizations.md`, and write `wompi-billing-webhook.md`. Verify: every route, status code, error code and body field in the four delta specs appears in the docs, and the webhook doc says where to set the address in Wompi's dashboard.

## 8. Frontend

- [x] 8.1 Add `lib/billing/notice.ts` and `SubscriptionNotice`, replacing `TrialBanner`. Verify: unit tests for the four rows of the notice table, the paid-and-fine case, a non-admin during a trial and an edge box.
- [x] 8.2 Add the `app/api/billing/**` handlers for the admin and recovery routes. Verify: tests of their decision helpers, including that the recovery state's sign-in body sets `access_token`, clears `setup_token` and never reaches the page.
- [ ] 8.3 Build `/ajustes/suscripcion` with its nav item. Verify: unit tests for the state-in-words function over trial, paid, cancelled, failed and no-card inputs; by hand, cancel asks for confirmation with the right date and resume undoes it.
- [x] 8.4 Send `trial_expired` and `payment_required` with a setup token to `/recuperar`, and build that page with its polling. Verify: unit tests for the sign-in outcome function and for what the page does on pending, approved and declined; `pnpm test` passes.
- [x] 8.5 Run `pnpm lint`, `pnpm test` and `pnpm build`. Verify: all pass.

## 9. End to end

- [ ] 9.1 On the local rig with the four sandbox values and a short trial set in the database: save `4242 4242 4242 4242`, move the trial's end to the past, wait for the job, and see the organization `active` with one charge in the Suscripción page and one transaction in Wompi's sandbox. Verify each step's screen and status.
- [ ] 9.2 Same with a source that declines, per the sandbox findings: see the failed-payment notice, move the dates past grace, be refused at sign-in, land on `/recuperar`, pay with `4242 4242 4242 4242` and enter the app. Verify each step.
- [ ] 9.3 Cancel during a trial, move the end to the past, and confirm no transaction is created and sign-in leads to `/recuperar`. Verify in Wompi's sandbox and in the app.
- [x] 9.4 Run `openspec validate subscription-charges --strict`. Verify: it passes.
