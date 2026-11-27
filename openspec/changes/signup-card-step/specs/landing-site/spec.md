## MODIFIED Requirements

### Requirement: The pricing section

The home page SHALL show the three plans with their names, their monthly price in Colombian pesos and what each includes:

| Plan | Monthly price |
|---|---|
| Básico | $25.000 |
| Facturación electrónica | $40.000 |
| Completo | $100.000 |

It SHALL NOT mention IVA. It SHALL offer annual payment at ten times the monthly price, described as two months free, and SHALL show that annual price for each plan. It SHALL say that the system is installed on the computer the restaurant already has. Each plan SHALL carry the primary action. When that action is the trial, a plan's action SHALL lead to the signup address with that plan named in the `plan` query parameter (`starter`, `einvoicing` or `full`); the trial actions outside the pricing section SHALL carry no plan. The Facturación electrónica plan SHALL be marked as the recommended one and set apart visually from the other two, without calling it the most chosen or citing any figure. The section SHALL NOT offer a way to pay on the site.

The prices shown SHALL equal the ones the cloud returns from `GET /api/plans`. A release of the site SHALL fail when they differ.

#### Scenario: A visitor compares the plans

- **WHEN** a visitor opens the plans section
- **THEN** the three plans are shown with $25.000, $40.000 and $100.000 per month and what each includes, and IVA is not mentioned

#### Scenario: The recommended plan

- **WHEN** a visitor opens the plans section
- **THEN** the Facturación electrónica plan is labelled "Recomendado" and stands out from the other two

#### Scenario: Annual payment

- **WHEN** a visitor reads the Completo plan
- **THEN** its annual price is shown as $1.000.000, with the two free months stated

#### Scenario: No payment on the site

- **WHEN** a visitor activates a plan's action
- **THEN** the primary action's destination opens, and no payment detail is asked for on the site

#### Scenario: The plan travels to signup

- **WHEN** signup is open and a visitor activates the Completo plan's action
- **THEN** the signup address opens with `plan=full`

#### Scenario: A price drifts

- **WHEN** the site is released with $45.000 for Facturación electrónica while the cloud returns 40000
- **THEN** the release fails and names the plan
