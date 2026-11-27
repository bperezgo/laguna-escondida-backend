## Why

`signup-card-step` saves a restaurant's card and starts its trial, but nothing ever charges that card. On day 14 the account expires and is activated by hand. This change makes the money arrive: the first charge, the renewals, and what the restaurant sees and does when a charge fails.

## What Changes

- **The first charge on the day the trial ends**, with the amount from the backend's price table. An approved charge makes the organization `active` and paid for that period.
- **A renewal charge** on each period's last day, unless the subscription is cancelled.
- **Failed charges are retried** on the due day, one day later and three days later. The organization keeps working those three days, then stops: `trial_expired` for a trial, the new `payment_required` for a paid one.
- **Wompi's events settle each charge.** A new webhook for the platform's own Wompi account checks each event's signature and applies an approval only when amount and currency equal the charge's. Charges with no answer are looked up.
- **A charge can never be made twice** for the same period, whichever replica runs the job.
- **Suscripción page in the app**, for admins: plan, period, next charge, saved card, the charges made, changing the card, cancelling and resuming.
- **A stopped account can pay its way back.** A sign-in refused for an ended trial or a missed payment gives the admin a session limited to a recovery page, where they pay with the saved card or a new one. **BREAKING**: those sign-in responses gain `setup_token`.
- **The trial banner becomes a subscription notice**: days left and first charge date, a failed charge with a link to change the card, and a cancelled subscription's last day.
- **An organization the operator activated by hand stays as it is**: no charges and no expiry.

Out of scope: changing plan or period after the trial starts, prorating, refunds, other payment methods, messages to the owner about a charge, and the electronic invoice we owe for each charge. The last two need their own changes.

## Capabilities

### New Capabilities

- `subscription-charges`: when a subscription is charged, how a charge is settled, retried and protected against duplicates.
- `subscription-management`: what an admin sees and does about the subscription, and how a stopped account recovers.

### Modified Capabilities

- `organization-plans`: a trial with a saved card gets three days of grace; an `active` organization with a paid period stops when it lapses.
- `organization-sign-in`: `trial_expired` and the new `payment_required` carry a setup token for an admin.

This change builds on `signup-card-step`, which must be implemented and archived first.

## Impact

- **Backend (cloud):** charging and settlement, two scheduled jobs, a webhook, admin and recovery routes, a `billing_charges` table, two environment variables, a `billing:manage` permission.
- **Frontend:** `/ajustes/suscripcion`, the recovery page, the subscription notice, sign-in handling of the two refusals.
- **API docs** for each of those. **Edge:** none.
- **Operations:** the platform's Wompi account needs its events address set to the new webhook.
