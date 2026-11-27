## ADDED Requirements

### Requirement: The public pages share one layout

The cloud app's signup pages and its sign-in page SHALL use one layout with the app's fonts, icons and colours. At 1024 pixels wide or more it SHALL have two panels: a brand panel that names the product and the 14-day trial, and a panel with the page's form. Below that width only the form panel SHALL show, under the logo, with no horizontal scrolling at 360 pixels.

The signup pages SHALL show three steps, Cuenta, Pago and Listo, with the current one marked and the finished ones shown as done.

The sign-in page SHALL show the organization's name when it is known and the product's name otherwise, and in the cloud SHALL link to signup. On an edge box the sign-in page SHALL keep its username and password form and SHALL show no signup link and no brand panel.

#### Scenario: Signup on a phone

- **WHEN** `/registro` opens at 360 by 640 pixels
- **THEN** the logo, the steps and the form are visible and nothing scrolls sideways

#### Scenario: Sign-in on an edge box

- **WHEN** the sign-in page opens on an edge box
- **THEN** it shows the username and password form and no link to signup

### Requirement: Step 1 creates the account

`/registro` SHALL ask for the restaurant's name and show the chosen plan with its monthly price, read from `GET /api/plans`.

- The plan SHALL be the one named by the `plan` query parameter when it is a valid plan, and the recommended plan (`einvoicing`) otherwise.
- The person SHALL be able to change the plan on this page, seeing each plan's name, price and what it includes, with the recommended one marked.
- The Google button SHALL be visible from the start. Using it with an empty or too long name SHALL show the error on the name field and send nothing.
- When prices cannot be read, the page SHALL show the plans without prices and signup SHALL still work.

A created account, and an account that is already pending, SHALL lead to step 2. An account that already has access SHALL be signed in and taken to the app.

#### Scenario: Arriving from the landing with a plan

- **WHEN** `/registro?plan=full` opens
- **THEN** the Completo plan is selected and its price shown

#### Scenario: Arriving with no plan

- **WHEN** `/registro` opens with no query, or with `plan=gold`
- **THEN** the Facturación electrónica plan is selected

#### Scenario: Google before the name

- **WHEN** the person uses the Google button with the name empty
- **THEN** the name field shows "Escribe el nombre del restaurante." and no account is created

#### Scenario: Account created

- **WHEN** signup answers `201`
- **THEN** the person is on `/registro/pago`

### Requirement: Step 2 shows what will be paid and takes the card

`/registro/pago` SHALL require a setup session. Without one it SHALL send the person to sign in.

It SHALL show the restaurant's name, the plan with a way to change it, and a choice between monthly and annual with both prices and the annual one described as two months free. It SHALL say that today's charge is $0, and SHALL give the date and the amount of the first charge, 14 days ahead. Amounts SHALL be the ones the backend returns, in pesos with a dot as thousands separator, with no tax line. Changing the plan or the period SHALL update the amount shown.

When card entry is on, it SHALL show a card form: number, expiry, security code and holder's name.
- The number SHALL be grouped as it is typed, the card's brand shown once it can be told, and a number that fails its check digit refused before anything is sent.
- The person SHALL have to accept Wompi's terms and its personal-data authorization, each linked to its document, before the card can be sent.
- The card SHALL go from the browser to Wompi only. No request to the app or to the backend SHALL carry the number, the expiry or the security code.
- While the card is being saved the button SHALL be disabled and say so.
- A refused card SHALL show "No pudimos guardar tu tarjeta. Revisa los datos o prueba con otra." and keep the holder's name. Too many attempts and an unreachable Wompi SHALL each show their own message. Every one of these SHALL offer the support contact when one is configured.
- The page SHALL say that the payment is handled by Wompi.

A saved card SHALL lead to step 3. An expired setup session SHALL send the person to sign in, with a message that they must sign in again to continue.

#### Scenario: Annual chosen

- **WHEN** the person on Facturación electrónica chooses the annual period
- **THEN** the page shows "Hoy pagas $0" and a first charge of $400.000 on the date 14 days ahead

#### Scenario: Card refused

- **WHEN** the card save answers `card_refused`
- **THEN** the refused-card message is shown, the person is still on step 2, and the number field is empty

#### Scenario: The card never reaches our servers

- **WHEN** a card is saved
- **THEN** the only request that carried the card number went to Wompi's address

#### Scenario: Consent missing

- **WHEN** the person sends the form without accepting Wompi's terms
- **THEN** the acceptance shows an error and nothing is sent

#### Scenario: Opening step 2 without a session

- **WHEN** `/registro/pago` opens with no setup session
- **THEN** the person is on the sign-in page

### Requirement: Step 2 is a status page when card entry is off

When card entry is off, `/registro/pago` SHALL show no card form. It SHALL keep the plan and period choice, and SHALL show the request's progress as three lines: request received (done), we review your request (current), your 14 days start (to do). It SHALL say that the person will be able to sign in once the request is approved, SHALL NOT promise a notification or a time, and SHALL offer the support contact when one is configured.

`/registro/pendiente` SHALL lead to `/registro/pago`.

#### Scenario: Waiting for approval

- **WHEN** card entry is off and a pending admin opens `/registro/pago`
- **THEN** the page shows the restaurant, the plan, the three progress lines and no card form

#### Scenario: Old pending address

- **WHEN** `/registro/pendiente` opens
- **THEN** the person is on `/registro/pago`

### Requirement: Step 3 confirms the trial

`/registro/listo` SHALL require a signed-in user of an organization on trial; anyone else SHALL be sent to the app's first screen or to sign in. It SHALL show the date the trial ends, the plan, the amount and date of the first charge, and the saved card's brand and last four digits. It SHALL offer installing the box as its main action and going to the app as the other.

#### Scenario: After saving the card

- **WHEN** the card is saved for Facturación electrónica, monthly, with a Visa ending in 4242
- **THEN** the page shows the trial's end date, "$40.000" as the first charge, and "Visa terminada en 4242"

### Requirement: A pending sign-in continues the signup

A sign-in that answers `pending_approval` SHALL take the person to `/registro/pago` with a setup session when the response carries a setup token, and SHALL show a message that the account is not active yet when it does not.

#### Scenario: Returning the next day

- **WHEN** an admin who left at step 2 signs in with Google
- **THEN** they are on `/registro/pago` with their restaurant and plan shown
