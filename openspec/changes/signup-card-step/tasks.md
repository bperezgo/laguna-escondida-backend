Backend commands run from `backend/`, frontend commands from `frontend/`, landing commands from `landing/`. Read only the files in design "Implementation map". Archive `signup-approval-gate` and `landing-pricing-and-product-proof` before archiving this change. Acceptance tests need the `DB_HOST=` prefix to use testcontainers.

## 1. Prices

- [x] 1.1 Add `Period`, `Price(plan, period)` and the monthly table to `plans.go`. Verify: a unit test asserts the nine values of the `plan-prices` table and that annual is ten times monthly.
- [x] 1.2 Add `GET /api/plans` (handler, cloud route, golden file). Verify: a handler test asserts the body in the spec, and the edge route list does not gain the route.

## 2. Setup token

- [x] 2.1 Add `GenerateSetupToken` and `ValidateSetupToken` to `JWTService`, and make `ValidateToken` refuse a token with an audience (design decision 1). Verify with unit tests: a setup token fails `ValidateToken`, a sign-in token fails `ValidateSetupToken`, and a setup token 61 minutes old is expired.
- [x] 2.2 Add `SetupAuthMiddleware`. Verify with tests: no token and a sign-in token answer `401`, a setup token of a trial organization answers `409 not_pending`, and a setup token of a pending one reaches the handler with the organization and user set.
- [x] 2.3 Return `setup_token` from signup's `201`, and from `pending_approval` for an admin on both sign-in routes (design decision 2). Verify: service and handler tests for the `self-serve-signup` and `organization-sign-in` deltas, including a pending manager who gets none.

## 3. Billing storage and selection

- [x] 3.1 Add migration `000098_add_organization_billing` and the repository. Verify: an acceptance test upserts a row, reads it back, rejects period `weekly`, and the down migration drops the table.
- [x] 3.2 Add `BillingSetupService.State` and `Select`, with `GET /api/billing/setup` and `PUT /api/billing/setup/selection` behind `SetupAuthMiddleware`. Verify with tests: the state with card entry off, a selection that changes plan and period and returns the table's amount, `400` with `field: period`, and the organization still `pending`.
- [x] 3.3 Add the billing period and the saved-card flag to the platform organization responses. Verify: a handler test shows both and shows no payment source identifier.

## 4. Billing provider

- [x] 4.1 Add `config/billing.go` (design decision 5). Verify with unit tests: no keys → off, both → on, one alone → error, mixed `test` and `prod` prefixes → error.
- [x] 4.2 Add the `BillingProvider` port and its Wompi adapter with `SavePaymentSource`. Verify with `httptest` tests like `wompi_client_test.go`: the request carries the private key, token, email and both acceptances; a `201` with `AVAILABLE` returns the source; a `422` and a non-available status return `ErrCardRefused`; a timeout returns the unavailable error.
- [x] 4.3 Make `State` return the Wompi fields when the provider is set. Verify: a test with a fake provider asserts the public key, both addresses and both tokens, and that no private key appears in the body.

## 5. Saving the card

- [x] 5.1 Add `BillingSetupService.SaveCard` (design decision 6) and `POST /api/billing/setup/card`, registered only when the provider is set. Verify with unit tests: success stores the card and starts the trial 14 days after the injected clock and returns a sign-in token; a refusal stores nothing and leaves `pending`; a database failure after Wompi succeeded leaves `pending`.
- [x] 5.2 Limit attempts to 5 per organization per hour. Verify: a test sends six and asserts the sixth is `429` with `Retry-After` and that the fake provider was called five times.
- [x] 5.3 Extend `testsupport/wompi.go` with payment sources and add an acceptance test: signup → state → selection → card → `GET /auth/me` with the returned token shows a trial. Add a second one for two concurrent saves: one `200`, one `409`. Verify: both pass with `DB_HOST= make test-acceptance` (or the repo's acceptance target).
- [x] 5.4 Run `make lint` and `make test`. Verify: both pass.

## 6. Wompi sandbox check (needs sandbox keys)

- [ ] 6.1 With sandbox keys, save the test card `4242 4242 4242 4242` by hand through the three calls. Record in design.md: the tokenization key route and payload that worked, whether a 3D Secure step is asked for, what a declined card answers at save time, and whether the widget can save a card without charging. Verify: the findings are written under a "Sandbox findings" heading, and any that contradict a decision have updated that decision before group 8 starts.

## 7. API docs

- [x] 7.1 Write `billing.md` and `plans.md`, and update `signup.md`, `auth.md` and `organizations.md`. Verify: every route, status code, error code and body field in the `plan-prices`, `signup-billing-setup`, `self-serve-signup` and `organization-sign-in` specs appears in the docs.

## 8. Frontend

- [ ] 8.1 Add `PublicLayout` and `Stepper`, move `/registro` and the sign-in page onto them, and remove `PublicShell`. Verify: `pnpm lint` and `pnpm build` pass, and at 360 and 1280 pixels wide the two pages match the `signup-flow` layout requirement, in the cloud and with the edge context.
- [x] 8.2 Add `app/api/plans/route.ts`, and in `lib/plans.ts` the recommended plan, the plan from a query value and the peso format. Verify with unit tests: `full` → full, `gold` and absent → `einvoicing`, 400000 → `$400.000`.
- [ ] 8.3 Rebuild step 1 with `PlanSummary` and `PlanPicker`, the Google button always visible, and prices that may be missing. Verify: unit tests for the outcome function (`201` and `pending_approval` → step 2, `200` → app), and by hand that Google with an empty name shows the field error.
- [x] 8.4 Store `setup_token` in its cookie from the signup and sign-in handlers, and add the `app/api/billing/setup/` handlers (design decision 8). Verify with tests of the decision helpers: the token is never in a response body, the card handler's `200` sets `access_token` and clears `setup_token`.
- [x] 8.5 Add `lib/billing/card.ts`. Verify with unit tests: grouping, Visa and Mastercard brands, a wrong check digit, an expired date and a past month of this year.
- [x] 8.6 Add `lib/billing/wompiToken.ts` per the sandbox findings. Verify: a unit test decrypts the payload with a test key pair and finds the five card fields, and a test with a fake `fetch` asserts the only address called is Wompi's.
- [x] 8.7 Build `/registro/pago`: `PeriodToggle`, `ChargeSummary`, `CardForm` with the two acceptances, each error message, and `RequestStatus` when card entry is off. Redirect `/registro/pendiente`. Verify: unit tests for the first-charge date and for the mapping of `card_refused`, `429`, `502`, `401` and `409` to what the page does; `pnpm test` passes.
- [ ] 8.8 Build `/registro/listo`. Verify: by hand with a trial session it shows the end date, the first charge and the card; without a session it leads to sign-in.
- [ ] 8.9 Add the content security policy for the public pages. Verify: on the local rig the Google button and the Wompi calls work and the console shows no violation.
- [x] 8.10 Run `pnpm lint`, `pnpm test` and `pnpm build`. Verify: all pass.

## 9. Landing

- [x] 9.1 Make each plan's trial action carry its plan, and remove the IVA note from the pricing section, the FAQ and the `plans.ts` comment. Remove "sin tarjeta" from `terminos.astro`. Verify: `actions.test.ts` and `plans.test.ts` cover the plan in the address and the plain address elsewhere; `pnpm check`, `pnpm test` and `pnpm build` pass; a search of `landing/src` for "IVA" and "sin tarjeta" finds nothing.
- [x] 9.2 Add the price check script and run it in `release-landing.yml`. Verify: a test runs it against a fixture with one wrong price and asserts it fails naming the plan, and against a matching fixture and asserts it passes.

## 10. End to end

- [ ] 10.1 On the local rig without billing keys: open `/registro?plan=full`, sign up, land on step 2 as a status page, choose annual, start the trial with the platform key, sign in and reach the app. Verify each screen and status code.
- [ ] 10.2 On the local rig with sandbox keys: sign up, save `4242 4242 4242 4242`, land on step 3 signed in, and see the payment source in Wompi's sandbox dashboard with no transaction. Then try `4111 1111 1111 1111` on a new signup. Verify: the first starts a trial, and the second's result matches the sandbox findings.
- [x] 10.3 Run `openspec validate signup-card-step --strict`. Verify: it passes.
