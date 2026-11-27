## Context

See `proposal.md` for the motivation. What shapes the approach:

- **A pending account has no session.** `signup-approval-gate` made signup answer `201` without a token, and a pending sign-in answer `403 pending_approval`. The card step comes after both, so it needs its own, narrower credential.
- **`OrganizationService.StartTrial`** already moves `pending` to `trial` with a conditional update and knows nothing about its caller.
- **One sign-in token type.** `JWTService.ValidateToken` parses any HS256 token signed with the secret into `JWTClaims`. A second token type signed with the same secret would pass it unless it is told apart.
- **Wompi code exists for the restaurants' own payments.** `WompiClient` (`internal/platform/httpclient/wompi_client.go`) takes the key per call, and `MerchantTerms` already returns the two document addresses and the two acceptance tokens. `WOMPI_API_URL` is shared configuration. The acceptance tests have a fake Wompi (`test/acceptance/testsupport/wompi.go`).
- **Wompi saves a card in three calls.** The browser encrypts the card with Wompi's tokenization key and posts it to `POST /v1/tokens/cards` with the public key. The merchant's server posts the token to `POST /v1/payment_sources` with the private key and both acceptance tokens, and gets a payment source that later charges reference. Nothing is charged. Wompi's widget documents only paying an amount, not saving a card.
- **Prices exist only in the landing** (`landing/src/config/plans.ts`). `frontend/lib/plans.ts` has names and feature lists, no prices.
- **Session cookies are set by Next.js route handlers** (`lib/auth/sessionResponse.ts`), and `signupDecision` already special-cases the `201`.
- **The public pages are inline-styled** (`PublicShell`, `SignInForm`), while `app-shell-redesign` moved the rest of the app to CSS modules, Manrope and Phosphor.

## Goals / Non-Goals

**Goals:**
- Everything up to a running trial works end to end against the fake Wompi, so live keys are only needed for the last check.
- Card data never reaches our backend or the Next.js server.
- Card entry is switched by configuration. Without keys the flow still ends somewhere sensible.
- The payment-source call sits behind a port, so the charge change adds methods to it instead of reaching into signup.

**Non-Goals:**
- Any charge, webhook, renewal or retry (`subscription-charges`).
- A second payment method. The form leaves room for a method selector and shows none.
- Editing the selection or the card after the trial starts.
- A platform UI for the operator.

## Decisions

### 1. The setup token is a JWT with an audience

`JWTService` gains `GenerateSetupToken(userID, orgID)` and `ValidateSetupToken`. The token is signed with the same secret, carries `aud: "billing-setup"`, the user and the organization, no roles and no permissions, and expires in 60 minutes. `ValidateToken` refuses any token with an audience, and `ValidateSetupToken` refuses any token without that one. A `SetupAuthMiddleware` puts the organization and user in the request context and answers `409 not_pending` when the organization's status is not `pending`. It reads the status from the database, not through the access cache: caching `pending` on a replica would make that replica refuse, for the cache's lifetime, the sign-in token a saved card has just issued.

*Alternatives considered:* a random token in a table. Rejected: it needs storage and cleanup for something that lives an hour. Asking for Google again on the payment page. Rejected: it is one more step at the moment a person is most likely to leave. Giving a pending admin a normal token and letting plan checks refuse the rest. Rejected: it reopens the gate that `signup-approval-gate` closed, one forgotten route at a time.

### 2. Where the setup token is issued

`SignupService.create` returns it with the `201`. In `completeSignIn`, the pending branch returns `ErrOrganizationPending` wrapped with the user, and `respondPendingApproval` adds `setup_token` when that user holds the admin role. The box user has no email and cannot reach this branch in the cloud.

### 3. Prices live in the `plans` package

`plans.Price(plan, period) int` in pesos, a `Period` type (`monthly`, `annual`), and `annual = monthly * 10` computed, not stored. `GET /api/plans` is a public cloud route with no rate limit beyond the global one, since it returns constants.

The app reads prices from `GET /api/plans` through a Next.js route handler, so it holds no copy. The landing keeps its display copy and `release-landing.yml` runs a script that fetches the production `GET /api/plans` and compares. It runs at release and not in `landing-ci.yml`, so a pull request does not depend on the network.

*Alternative considered:* a table of prices in the database. Rejected for now: a price change is rare and a release is an acceptable way to make it. The `Price` function is the only reader, so moving it later touches one place.

### 4. `organization_billing`, one row per organization

Cloud migration `000098`: `organization_id` (primary key, references `organizations`), `period` (`monthly | annual`, default `monthly`), `provider` (`wompi`, nullable), `payment_source_id`, `card_brand`, `card_last_four`, `terms_accepted_at`, timestamps. The row is created by the first selection or card save. It follows the tenant rules of the other cloud tables and is never synced to a box.

The plan stays on `organizations.plan`. The period is new information and belongs to billing, so it is not a column on `organizations`.

`payment_source_id` is stored as is. It is useless without our private key.

### 5. The billing provider port

```go
type BillingProvider interface {
    Terms(ctx context.Context) (dto.WompiMerchantTerms, error)
    SavePaymentSource(ctx context.Context, req dto.PaymentSourceRequest) (dto.PaymentSource, error)
}
```

The Wompi adapter reuses `WompiClient.call` and `MerchantTerms`, built with the platform's own keys from configuration. `BillingSetupService` holds the port as a nilable dependency: nil means card entry is off, `State` returns `card_entry: false`, and the card route is not registered.

Configuration is `BILLING_WOMPI_PUBLIC_KEY` and `BILLING_WOMPI_PRIVATE_KEY`. Both set turns card entry on. One without the other fails startup. Keys of different environments (`pub_test_` with `prv_prod_`) fail startup too. The API address is the existing `WOMPI_API_URL`.

### 6. Saving the card: Wompi first, then one transaction

`BillingSetupService.SaveCard`:
1. Count the attempt against the organization's limit (5 per hour). The count is a middleware on the route (`RateLimitByOrganization`, the existing limiter keyed by organization instead of address), so the service never sees a sixth attempt. Like the other limits it is per process.
2. Call `SavePaymentSource`. A refusal or a status other than `AVAILABLE` returns `ErrCardRefused`.
3. In one database transaction: upsert `organization_billing` and call the same conditional update `StartTrial` uses. If the update changes no row, the transaction rolls back and the result is `ErrOrganizationNotPending`.
4. Issue a sign-in token with the existing `GenerateToken`.

If step 3 fails after step 2 succeeded, a payment source exists at Wompi that we do not reference. It costs nothing and charges nothing. The person retries with a new card token.

*Alternative considered:* starting the trial from a webhook, as first assumed. Rejected: a payment source is created synchronously and Wompi sends no event for it.

### 7. The browser tokenizes the card

`lib/billing/wompiToken.ts` fetches Wompi's tokenization key, builds the encrypted payload with the browser's Web Crypto (RSA-OAEP-256 and A256GCM), and posts it to Wompi with the public key from the setup state. No new dependency. `lib/billing/card.ts` holds the pure parts: grouping, brand from the number's prefix, check digit, expiry validation.

The form's fields have no `name` that a form post would send, and the form never submits to our origin. Acceptance tokens are single use, so every attempt reads the setup state again first.

*Alternative considered:* Wompi's widget. Rejected: its documentation only describes paying an amount. An unverified mention of a tokenize-only mode exists; task 6.1 checks it in the sandbox, and if it works it replaces only this decision.

### 8. Cookies and route handlers

The Next.js handlers for signup and sign-in store `setup_token` in an `httpOnly`, `sameSite: strict` cookie named `setup_token`, with a 60-minute life, and never return it to the page. New handlers under `app/api/billing/setup/` forward it as a bearer token. The card handler, on `200`, sets `access_token` as sign-in does and clears `setup_token`.

### 9. Layout and components

`components/public/PublicLayout` (CSS module, two panels), `Stepper`, `PlanSummary`, `PlanPicker`, `PeriodToggle`, `ChargeSummary`, `CardForm`, `RequestStatus`. They use the existing tokens in `globals.css` and the `components/ui` primitives. `PublicShell` is removed once nothing uses it. Each page's decisions (which plan is selected, which step, where an outcome leads, how an amount is written) live in pure functions under `lib/`, tested as `lib/signup.ts` is today.

The edge box's sign-in page uses the same form component inside a single-panel variant, so the box bundle gains no brand panel and no signup link.

### 10. Step 3 gets the period and the card from step 2

No route in this change lets a signed-in trial admin read their billing; `GET /api/billing` belongs to `subscription-charges`. So step 2 hands step 3 a summary in the tab's `sessionStorage`: plan, period, amount, and the brand and last four digits Wompi returned with the token. Never the card. Step 3 always shows the trial's end and the plan from `/auth/me`, and shows the first charge and the card only when the summary is there. Opened later, or in another tab, those two lines are missing until `subscription-charges` adds the read.

### 11. The billing fields of the platform responses live on the organization

`dto.Organization` gains `billing_period` and `has_saved_card`, filled by `OrganizationService` from the billing repository, which the cloud sets with `ShowBilling`. An edge box has no billing and answers `monthly` and `false`.

## Sandbox findings

Not run yet: there are no sandbox keys (task 6.1). What the code assumes from Wompi's documentation, to check first:

- **Tokenization**: `GET /tokens/keys/tokenization` with the public key as bearer answers `data.publicKey`, an RSA public key the browser imports as SPKI (PEM or bare base64). `POST /tokens/cards` takes `{"payload": "<compact JWE, RSA-OAEP-256 + A256GCM>"}` of `number`, `cvc`, `exp_month`, `exp_year`, `card_holder`, and answers `data.id`, `data.brand`, `data.last_four`.
- **Payment source**: the answer's `data.public_data` is read for `brand`, `bin` and `last_four`. If it carries none, the stored brand and last four are empty and step 3 still shows what the browser got from the token.
- **Refusals**: `400`, `404` and `422` from `POST /payment_sources` are treated as a refused card; `401`, `403` and `5xx` as Wompi unavailable.
- **Merchant terms**: `Terms` reuses `WompiClient.MerchantTerms`, which calls `GET /merchants/{public key}`. Wompi's documentation says that route is replaced by `GET /merchants/info` with an `x-merchant-public-key` header and stops working on 31 October 2026. If it is gone, the setup state answers `502` and the restaurants' QR payments fail too. Check this before anything else.
- **3D Secure** at save time and a **tokenize-only widget** are still unverified.

## Risks / Trade-offs

- **Card numbers are typed into our page** → Our PCI scope becomes the self-assessment for merchants whose page handles card data. The page loads no third-party script, and a content security policy for the public pages limits `connect-src` to our origin, Google and Wompi. The owner should confirm the obligation with Wompi when asking for the keys.
- **A stolen card can start a free trial** → It only costs us a trial. The limit of 5 attempts per organization, plus the signup limit per address, keeps the route from being useful for testing cards.
- **Recurring card charges may need 3D Secure at save time** → Not confirmed from the documentation. Task 6.1 checks it before the card form is built. If it does, step 2 gains a challenge state and this design is updated first.
- **Wompi may accept a card it cannot later charge** → The first charge fails on day 14, which `subscription-charges` handles.
- **The landing's price check needs the cloud up at release time** → The script fails with a clear message, and the release can be rerun.
- **Two changes are not archived** → `signup-approval-gate` and `landing-pricing-and-product-proof` hold the requirements these deltas modify. Archive both first.

## Migration Plan

1. Cloud migration `000098` adds `organization_billing`. The down migration drops it.
2. Ship backend and frontend in one `cloud/v*` release, without billing keys. The flow ends in the status page and the operator approves as today.
3. Ship the `landing/v*` release after the cloud one, since its price check needs `GET /api/plans`.
4. Set the two sandbox keys in a test environment and run task 9.1. Then set the production keys.
5. Rollback: remove the keys to turn card entry off, or redeploy the previous cloud release. Saved payment sources stay at Wompi and are never charged.

## Implementation map

- **Migration**: `backend/internal/platform/postgres/migrations/cloud/000098_add_organization_billing.{up,down}.sql`
- **Domain**: `internal/domain/plans/plans.go` (prices, period), `internal/domain/dto/billing.go` (new), `internal/domain/dto/signup.go`, `internal/domain/error/billing_error.go` (new), `internal/domain/ports/billing_provider.go` and `billing_repository.go` (new), `internal/domain/service/jwt.go`, `billing_setup_service.go` (new), `signup_service.go`, `user_service.go`, `organization_service.go` (shared conditional update, platform response fields)
- **Adapters**: `internal/platform/httpclient/wompi_client.go` (+ payment source), `internal/platform/postgres/repository/billing_repository.go` (new), `internal/platform/handler/billing_setup_handler.go`, `plans_handler.go` (new), `signup_handler.go`, `user_handler.go`, `middleware.go`, `organization_handler.go`, `rate_limit_middleware.go`, `internal/platform/httpclient/wompi_billing.go` (new), `internal/platform/config/billing.go` (new), `internal/bootstrap/routes_cloud.go`, `routes_signup.go` + `testdata/routes_cloud.golden`
- **Tests**: next to each file above; acceptance in `test/acceptance/` with `testsupport/wompi.go` extended for payment sources
- **API docs**: `backend/docs/api/billing.md` and `plans.md` (new), `signup.md`, `auth.md`, `organizations.md`
- **Frontend**: `components/public/*` (new), `components/signup/SignupPageClient.tsx`, `components/auth/SignInForm.tsx`, `components/auth/PublicShell.tsx` (removed), `app/registro/page.tsx`, `app/registro/pago/page.tsx` and `app/registro/listo/page.tsx` (new), `app/registro/pendiente/page.tsx` (redirect), `app/api/signup/google/route.ts`, `app/api/auth/signin/route.ts`, `app/api/auth/google/route.ts`, `app/api/billing/setup/**` and `app/api/plans/route.ts` (new), `lib/signup.ts`, `lib/signIn.ts`, `lib/plans.ts`, `lib/auth/sessionDecision.ts`, `lib/auth/sessionResponse.ts`, `lib/billing/*` (new), `next.config.*` (content security policy)
- **Landing**: `src/components/sections/Pricing.astro`, `src/components/PrimaryAction.astro`, `src/config/actions.ts`, `src/config/plans.ts`, `src/components/sections/faq-data.ts`, `src/pages/terminos.astro`, `scripts/check-prices.*` (new), `.github/workflows/release-landing.yml`

## Open Questions

- The exact Spanish wording of each screen. It can be adjusted in review.
- The brand panel's image: a product screenshot from the landing's set, or text only.
