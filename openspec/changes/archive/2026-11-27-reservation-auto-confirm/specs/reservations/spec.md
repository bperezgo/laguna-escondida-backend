## ADDED Requirements

### Requirement: Staff set the reservation auto-confirm policy

`GET /api/organization/settings` SHALL include a `reservations` section with
`auto_confirm_enabled` (boolean, default `false`), `max_party_size` (integer
1–50, default 8), `min_notice_minutes` (integer 0–10080, default 120),
`daily_guest_cap` (positive integer, or null) and `schedule` (a list of
opening windows in the format and with the validation of the ordering
schedules; empty means any time). An organization with no saved section
SHALL read these defaults. Changing it through `PATCH
/api/organization/settings` SHALL require `settings:reservations:update`,
granted to admins and managers. Enabling the policy without a
`daily_guest_cap`, or saving any field out of range, SHALL be rejected with a
validation error naming the field, saving nothing. The section SHALL be
served in the cloud only.

#### Scenario: Turning auto-confirm on

- **WHEN** a manager patches `reservations` with `auto_confirm_enabled: true` and `daily_guest_cap: 120`
- **THEN** the settings are saved, with `max_party_size` 8 and `min_notice_minutes` 120

#### Scenario: Enabled without a cap

- **WHEN** a manager patches `reservations` with `auto_confirm_enabled: true` and no `daily_guest_cap`
- **THEN** the request fails with a validation error naming `daily_guest_cap` and the settings are unchanged

#### Scenario: Without the permission

- **WHEN** a user without `settings:reservations:update` patches `reservations`
- **THEN** the response is `403` and nothing changes

### Requirement: A WhatsApp request that meets the policy is confirmed at once

When a reservation with `source` `whatsapp` is created, the system SHALL
store it `confirmed` if all of these hold. Otherwise it SHALL store it
`requested`, with a `pending_reason` naming the first rule that failed, in
this order:

1. `disabled`: the policy is enabled.
2. `party_size`: `party_size` is at most `max_party_size`.
3. `notice`: `starts_at` is at least `min_notice_minutes` after now.
4. `outside_schedule`: `starts_at` falls inside the schedule, or the schedule
   is empty. It is read in the organization's timezone.
5. `daily_cap`: the guests of that day's `confirmed` and `visited`
   reservations, of every source, plus this `party_size`, are at most
   `daily_guest_cap`. The day is the organization's calendar day.

The policy SHALL NOT refuse a reservation, and a `requested` one SHALL follow
the existing transitions. Reservations for the same organization and day
SHALL be checked against the cap one at a time, so concurrent requests never
confirm more guests than the cap. Staff-created reservations SHALL be
unaffected and SHALL keep being stored `confirmed`. They do count toward the
cap.

#### Scenario: An ordinary table

- **WHEN** the policy is enabled with the defaults and a cap of 120, 40 guests are already confirmed for tomorrow, and the agent requests a table for 4 tomorrow at noon
- **THEN** the reservation is stored `confirmed`

#### Scenario: A large party

- **WHEN** the agent requests a table for 10 under the same policy
- **THEN** the reservation is stored `requested` with `pending_reason` `party_size`

#### Scenario: Too little notice

- **WHEN** the agent requests a table starting in 90 minutes
- **THEN** the reservation is stored `requested` with `pending_reason` `notice`

#### Scenario: The day is full

- **WHEN** 118 guests are confirmed for Saturday, the cap is 120, and the agent requests a table for 4 on Saturday
- **THEN** the reservation is stored `requested` with `pending_reason` `daily_cap`

#### Scenario: Two requests for the last seats

- **WHEN** 116 guests are confirmed for Saturday, the cap is 120, and two requests for 4 arrive at the same moment
- **THEN** exactly one is `confirmed` and the other is `requested` with `pending_reason` `daily_cap`

#### Scenario: Auto-confirm is off

- **WHEN** the organization never enabled the policy and the agent requests a table
- **THEN** the reservation is stored `requested` with `pending_reason` `disabled`
