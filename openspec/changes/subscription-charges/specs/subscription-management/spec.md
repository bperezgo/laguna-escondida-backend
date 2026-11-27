## ADDED Requirements

### Requirement: An admin reads the subscription

`GET /api/billing` SHALL exist only in the cloud and SHALL require the `billing:manage` permission, which only the admin role holds. It SHALL answer `200` with the organization's plan, period, status, due date, the amount of the next charge, whether the subscription is cancelled, the saved card's brand and last four digits when there is one, the state of the latest charge, and the charges made, newest first, each with its date, amount, status and the period it covers. It SHALL never carry the payment source's identifier.

An organization without a saved card SHALL get the same body with no card, no next charge and no charges.

#### Scenario: During the trial

- **WHEN** the admin of a trial organization on `einvoicing`, monthly, with a Visa ending in 4242, reads the subscription
- **THEN** the response shows the trial's end as the due date, 40000 as the next charge and the card

#### Scenario: A manager

- **WHEN** a manager calls `GET /api/billing`
- **THEN** the response is `403`

### Requirement: An admin changes the card

`PUT /api/billing/card` SHALL take a Wompi card token and the two acceptance tokens, create a new payment source and replace the saved one, under the same rules, limits and failures as saving the first card (see `signup-billing-setup`). `GET /api/billing/card-setup` SHALL return what the browser needs to send a card to Wompi, as the setup state does. The previous card SHALL stay in use when the new one is refused.

#### Scenario: New card after a declined charge

- **WHEN** an admin whose renewal was declined yesterday saves a new card
- **THEN** the new card is the saved one
- **AND** one charge for the unpaid period is created at once

#### Scenario: New card refused

- **WHEN** Wompi refuses the new card
- **THEN** the response is `402` with code `card_refused` and the previous card is still the saved one

### Requirement: An admin cancels and resumes

`POST /api/billing/cancel` SHALL mark the subscription as cancelled. No charge SHALL be created for a cancelled subscription. The organization SHALL keep working until its due date and SHALL then stop as described in `organization-plans`, with no grace. The saved card SHALL be kept.

`POST /api/billing/resume` SHALL remove the cancellation while the due date has not passed. After it has, it SHALL answer `409` with code `subscription_lapsed`.

Both SHALL require `billing:manage` and SHALL answer `200` with the same body as `GET /api/billing`. Both SHALL answer `409` with code `no_subscription` for an organization without a saved card.

#### Scenario: Cancelling on day 10 of the trial

- **WHEN** an admin cancels on day 10
- **THEN** the organization works until day 14, no charge is made, and it stops on day 14

#### Scenario: Changing their mind

- **WHEN** that admin resumes on day 12
- **THEN** the first charge is made on day 14 as if they had never cancelled

### Requirement: A stopped account can pay its way back

The admin of an organization stopped for an ended trial or a missed payment gets a setup session at sign-in (see `organization-sign-in`). With it, and with nothing else, the routes under `/api/billing/recovery` SHALL work. They SHALL exist only in the cloud and SHALL answer `409` with code `not_stopped` for an organization that is working, pending or suspended.

- `GET /api/billing/recovery` SHALL answer with the organization's name, plan, period, the amount due, the saved card's brand and last four digits when there is one, the state of the latest charge, and what the browser needs to send a card to Wompi when card entry is on.
- `PUT /api/billing/recovery/selection` SHALL change plan and period as in `signup-billing-setup`.
- `POST /api/billing/recovery/pay` SHALL take an optional new card (token and acceptances), save it when given, and create one charge at once for the plan and period. It SHALL answer `202` with the charge's state. Without a saved card and without a new one it SHALL answer `400` naming the card token. It SHALL answer `409` with code `charge_pending` while a charge is pending.

When that charge is approved, the organization SHALL become `active` with a paid period that starts at the approval, and its cancellation, if any, SHALL be removed. Once it is, `GET /api/billing/recovery` SHALL answer with the same body as a successful sign-in, once, so the person is signed in.

#### Scenario: Paying after the trial ended without a card

- **WHEN** the admin of an organization whose trial the operator started, and which ended a week ago, pays `starter`, monthly, with a new card that is approved
- **THEN** the organization is `active` and paid for one month from the approval
- **AND** the admin is signed in

#### Scenario: The saved card works again

- **WHEN** the admin of an organization stopped after three failed renewals pays with the saved card and it is approved
- **THEN** the organization is `active`

#### Scenario: Declined again

- **WHEN** the recovery charge is declined
- **THEN** the organization stays stopped and the recovery state shows the declined charge

#### Scenario: A working organization

- **WHEN** a setup token of an organization that has since been activated calls `POST /api/billing/recovery/pay`
- **THEN** the response is `409` with code `not_stopped`

### Requirement: The Suscripción page

In the cloud, the app SHALL offer admins a Suscripción page under Ajustes, shown to nobody else and not on an edge box. It SHALL show the plan and period, the state in words (on trial until a date, paid until a date, cancelled with its last day, payment failed), the next charge's date and amount, the saved card, and the charges made with their date, amount and state. Amounts SHALL be in pesos with a dot as thousands separator and no tax line.

From it the admin SHALL be able to change the card, with the same card form as signup, and to cancel or resume. Cancelling SHALL ask for confirmation, stating the last day of access. Changing plan or period SHALL be offered as a message to support.

#### Scenario: Cancelling

- **WHEN** an admin paid until 15 January chooses to cancel
- **THEN** the page asks to confirm and says access lasts until 15 January
- **AND** after confirming, the state reads as cancelled with that date and offers to resume

#### Scenario: An organization without a card

- **WHEN** the admin of an organization activated by hand opens the page
- **THEN** it shows the plan and that billing is handled with support, and no card or charge

### Requirement: The recovery page

A sign-in refused with `trial_expired` or `payment_required` that carries a setup token SHALL take the person to a recovery page on the public layout. Without a setup token the sign-in page SHALL show the refusal's message and the support contact, as today.

The page SHALL say why the account stopped, show the plan, the period and the amount due with a way to change both, and offer one main action: paying with the saved card when there is one, with a way to use another card, or the card form when there is none. While the charge is pending it SHALL say so and check again every few seconds without the person doing anything. An approved charge SHALL take them into the app. A declined one SHALL say so and offer another card.

When card entry is off, the page SHALL show the reason and the support contact only.

#### Scenario: Trial ended, card on file

- **WHEN** an admin whose trial ended and whose three charges failed signs in
- **THEN** they are on the recovery page, which shows the amount due and "Pagar con Visa terminada en 4242" as the main action

#### Scenario: Paid

- **WHEN** the recovery charge is approved while the page is open
- **THEN** the person is in the app without signing in again

### Requirement: The subscription notice

In the cloud the app SHALL show admins one notice at the top of every page, in place of the trial banner, in these cases only. Other users SHALL see the trial's days left during a trial, as today, and nothing otherwise. An edge box SHALL show none.

| Case | Notice | Link |
|---|---|---|
| Trial with a saved card, not cancelled | days left and the date and amount of the first charge | Suscripción |
| Trial without a saved card | days left | Suscripción, or the support contact when card entry is off |
| A charge failed and the organization is still working | the payment failed and the day access ends | change the card |
| Cancelled and still working | the last day of access | resume |

A paid organization with nothing wrong SHALL see no notice.

#### Scenario: Three days left with a card

- **WHEN** an admin of a trial that ends in three days, on `einvoicing` monthly with a saved card, opens any page
- **THEN** the notice says three days are left and that $40.000 will be charged on the trial's last day

#### Scenario: Renewal declined yesterday

- **WHEN** an admin of an organization whose renewal was declined yesterday opens any page
- **THEN** the notice says the payment failed, gives the date access ends, and links to changing the card

#### Scenario: Paid and fine

- **WHEN** an admin of a paid organization opens any page
- **THEN** no notice is shown
