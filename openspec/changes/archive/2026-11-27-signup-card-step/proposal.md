## Why

A restaurant that signs up today picks its plan twice, fills a plain form and lands on a paragraph about waiting for approval. The owner wants a trial only for restaurants that leave a card, and a signup that looks like the landing and the redesigned app. Wompi can save a card without charging it, so the card step can start the trial.

## What Changes

- **A three-step signup on one shared public layout**: Cuenta, Pago, Listo. Sign-in moves to the same layout.
- **The plan chosen on the landing arrives in the app.** Signup opens with it selected.
- **The backend owns plan prices.** A public `GET /api/plans` returns the monthly and annual price of each plan. The app reads them from it, and the landing's release checks its copy against it.
- **A setup session for a pending account.** Signup, and a correct sign-in of a pending admin, return a 60-minute token that only the billing setup routes accept. **BREAKING**: both responses gain `setup_token`.
- **Step 2, Pago**: the plan, monthly or annual, "hoy pagas $0", the date and amount of the first charge, a card form, and Wompi's two acceptances.
- **Saving a card starts the trial.** The browser sends the card to Wompi. The backend turns Wompi's token into a payment source, starts the 14-day trial and signs the person in. Our servers never receive the card number.
- **Without billing keys, step 2 is a status page.** The operator starts the trial as today. Turning card entry on is configuration.
- **Step 3, Listo**: the trial's end date, the first charge, and installing the box.
- **No IVA** on the landing or in any screen, pending the accountant's confirmation. The terms page stops saying "sin tarjeta".

Out of scope: every charge and what follows it (`subscription-charges`), other payment methods, and the trial banner.

## Capabilities

### New Capabilities

- `plan-prices`: the price of each plan and period, and the public route that returns them.
- `signup-billing-setup`: the setup session, choosing plan and period, saving a card that starts the trial, and the fallback without billing keys.
- `signup-flow`: the public screens of the app: layout, steps, plan handoff, card form, status page and sign-in.

### Modified Capabilities

- `self-serve-signup`: the `201` carries a setup token.
- `organization-sign-in`: `pending_approval` carries a setup token for an admin.
- `landing-site`: each plan's trial action carries its plan, and the IVA note is removed.

`signup-approval-gate` and `landing-pricing-and-product-proof` must be archived before this change is.

## Impact

- **Backend (cloud):** price table, setup token, four new routes, a Wompi payment-source call, an `organization_billing` table, two environment variables.
- **Frontend:** public layout, the three signup pages, sign-in page, card tokenization in the browser.
- **Landing:** plan in the trial link, IVA and "sin tarjeta" copy, a price check in the release.
- **API docs** for each of those. **Edge:** none.
- **Security:** the card number is typed into our page, which puts us in PCI scope.
