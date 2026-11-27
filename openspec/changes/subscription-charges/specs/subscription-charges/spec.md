## ADDED Requirements

### Requirement: A subscription is an organization with a saved card

An organization SHALL have a subscription when it has a saved payment source. A subscription has a period (`monthly` or `annual`), a due date, and may be cancelled. Only an organization with a subscription that is not cancelled SHALL ever be charged. An organization without one, including one the operator activated by hand, SHALL never be charged and SHALL keep the access rules it has today.

The due date SHALL be the trial's end while the organization is on trial, and the end of the paid period once it has paid.

#### Scenario: Activated by hand

- **WHEN** the operator set an organization to `active` and it has no saved card
- **THEN** no charge is ever created for it

#### Scenario: Cancelled before the trial ends

- **WHEN** a trial organization with a saved card cancelled on day 10
- **THEN** no charge is created on day 14

### Requirement: The first charge is made when the trial ends

When a trial organization's subscription reaches its due date, the cloud SHALL create one charge at Wompi against the saved payment source, for the price of its plan and period in the price table (see `plan-prices`), in Colombian pesos, with no tax added. The charge SHALL be created within one hour of the due date.

When that charge is approved, the organization SHALL become `active`, and its paid period SHALL run from the trial's end for one month or one year, by its period.

#### Scenario: Monthly trial ends

- **WHEN** an organization on `einvoicing`, monthly, reaches the end of its trial on 2026-12-15 with a saved card
- **THEN** a charge of 40000 pesos is created at Wompi
- **AND** once approved, the organization is `active` and paid until 2027-01-15

#### Scenario: Annual trial ends

- **WHEN** an organization on `full`, annual, reaches the end of its trial
- **THEN** the charge is 1000000 pesos and, once approved, it is paid for one year

### Requirement: A paid period is renewed on its last day

When an `active` organization's subscription reaches its due date, the cloud SHALL create one charge as for the first one, with the price of its plan and period at that moment. When it is approved, the paid period SHALL extend by one month or one year from the previous due date, not from the day of the payment.

#### Scenario: Renewal paid two days late

- **WHEN** a monthly organization paid until 2027-01-15 has its renewal approved on 2027-01-17
- **THEN** it is paid until 2027-02-15

#### Scenario: Plan changed during the period

- **WHEN** the operator moved an organization from `einvoicing` to `full` during a paid month
- **THEN** its next renewal is 100000 pesos

### Requirement: A charge is settled by what Wompi reports

A charge SHALL be `pending` from its creation until Wompi reports a final status, and then `approved`, `declined`, `voided` or `error`. Only an `approved` charge SHALL change an organization's status or paid period.

An approval SHALL be applied only when the reported amount and currency equal the charge's. An approval with a different amount or currency SHALL NOT be applied, SHALL leave the charge `pending`, and SHALL be logged as an error naming the organization and both amounts.

Each Wompi transaction SHALL be applied at most once, however many times it is reported.

#### Scenario: Approved with the right amount

- **WHEN** Wompi reports the transaction of a 40000 peso charge as approved for 4000000 cents in COP
- **THEN** the charge is `approved` and the paid period starts

#### Scenario: Approved with a wrong amount

- **WHEN** Wompi reports that transaction as approved for 100000 cents
- **THEN** the charge stays `pending`, the organization is unchanged and an error is logged

#### Scenario: Reported twice

- **WHEN** the same approved transaction is reported twice
- **THEN** the paid period is extended once

### Requirement: Wompi's billing events are accepted only when signed

`POST /api/webhooks/wompi-billing` SHALL exist only in the cloud and SHALL require no credential. It SHALL accept an event only when its checksum is the SHA-256 of the values named in the event's `signature.properties`, then its `timestamp`, then the platform's events secret. An event with a wrong or missing checksum SHALL answer `401` and change nothing. A signed event that is not `transaction.updated`, or whose reference matches no charge, SHALL answer `200` and change nothing.

While the cloud has no events secret, the route SHALL answer `404`.

#### Scenario: Forged event

- **WHEN** an event with an approved transaction and a wrong checksum arrives
- **THEN** the response is `401` and no charge changes

#### Scenario: Event of another kind

- **WHEN** a signed `nequi_token.updated` event arrives
- **THEN** the response is `200` and nothing changes

### Requirement: A charge with no answer is looked up

A charge still `pending` 10 minutes after its creation SHALL be looked up at Wompi, and settled by the status Wompi returns under the same rules as an event. The lookup SHALL repeat until the charge is settled. A charge still `pending` after 24 hours SHALL be logged as an error and SHALL count as failed for retries.

#### Scenario: The event never arrives

- **WHEN** a charge is approved at Wompi but no event reaches the cloud
- **THEN** within 15 minutes of its creation the charge is `approved` and the organization is paid

### Requirement: A failed charge is retried three times

When a charge is declined, voided or in error, the cloud SHALL try again one day after the due date, and again three days after the due date. It SHALL NOT create a charge while another for the same organization is `pending`. After the third failure it SHALL create no more charges for that due date on its own.

Saving a new card (see `subscription-management`) SHALL cause one more charge at once when the due date has passed and no charge was approved for it.

#### Scenario: Declined on the due day, approved the next

- **WHEN** the first charge is declined on the due date and the second is approved a day later
- **THEN** the organization is paid from the due date and was never stopped

#### Scenario: Three failures

- **WHEN** the charges on the due date, one day later and three days later all fail
- **THEN** no fourth charge is created
- **AND** the organization stops as described in `organization-plans`

### Requirement: No period is charged twice

For one organization and one due date there SHALL be at most one approved charge and at most one pending charge, even when several cloud replicas run the job at the same moment or the job runs again after a crash. Each charge SHALL carry a reference unique to its organization, due date and attempt.

#### Scenario: Two replicas at the due time

- **WHEN** two replicas find the same organization due in the same second
- **THEN** one transaction is created at Wompi

#### Scenario: Crash after Wompi accepted

- **WHEN** the cloud stops after Wompi accepted a transaction and before the charge was stored as sent
- **THEN** after restart no second transaction is created for that attempt, and the first one is settled by its reference
