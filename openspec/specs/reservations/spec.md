# reservations Specification

## Purpose
Lets a restaurant record table reservation requests from guests, whether they arrive through the WhatsApp agent or are entered by staff, keep one contact per guest phone, and move each reservation from request to confirmed, visited, declined, cancelled or no-show.

## Requirements

### Requirement: A contact is one guest phone within an organization

The system SHALL keep at most one contact per organization and E.164 phone. Creating a reservation, or an order through the customer agent, for a phone with no contact SHALL create one. A contact SHALL have an id, the phone, a name, `first_seen_at` and `last_seen_at`. Each later reservation or agent order for the phone SHALL set `last_seen_at`, and SHALL replace the name when it brings a non-empty one. A contact of one organization SHALL never be visible to another.

#### Scenario: First reservation creates the contact

- **WHEN** a reservation is created for phone `+573005551234`, which has no contact
- **THEN** a contact with that phone and the reservation's name exists

#### Scenario: A returning guest

- **WHEN** a second reservation is created for the same phone with name "Ana María"
- **THEN** no new contact is created, and the existing contact's name is "Ana María" and its `last_seen_at` is updated

#### Scenario: Same phone in two restaurants

- **WHEN** organizations A and B each get a reservation from the same phone
- **THEN** each has its own contact, and neither can read the other's

### Requirement: A reservation records who, how many, when and from where

A reservation SHALL have an id, its contact, a `name` (1–100 characters after trimming), a `party_size` (integer 1–50), a `starts_at` (date and time with offset), optional `notes` (at most 500 characters), a `source` (`whatsapp`, `phone`, `walk_in` or `staff`), a status, and `created_at` and `updated_at`. `starts_at` SHALL be in the future and at most 60 days ahead when the reservation is created. Invalid input SHALL be rejected with a validation error that names the field, storing nothing.

#### Scenario: Party size too large

- **WHEN** a reservation is created with `party_size` 80
- **THEN** the request fails with a validation error naming `party_size` and nothing is stored

#### Scenario: Time in the past

- **WHEN** a reservation is created with a `starts_at` one hour ago
- **THEN** the request fails with a validation error naming `starts_at`

### Requirement: Reservations move through a fixed set of states

A reservation SHALL be in exactly one of `requested`, `confirmed`, `visited`, `declined`, `cancelled` or `no_show`. Only these transitions SHALL be allowed:
- `requested` → `confirmed`, `declined` or `cancelled`
- `confirmed` → `visited`, `cancelled` or `no_show`
- `visited` → `no_show`, only while no feedback request exists for it

Any other transition SHALL be refused with HTTP `409` and SHALL change nothing. A decline SHALL accept an optional reason of at most 255 characters.

#### Scenario: Confirming a request

- **WHEN** staff confirm a `requested` reservation
- **THEN** it is `confirmed`

#### Scenario: Confirming a declined reservation

- **WHEN** staff confirm a `declined` reservation
- **THEN** the response is `409` and the reservation stays `declined`

#### Scenario: Correcting a no-show the next morning

- **WHEN** a reservation became `visited` overnight, no feedback has been requested yet, and staff mark it `no_show`
- **THEN** it is `no_show` and no feedback will be requested for it

### Requirement: A confirmed reservation counts as visited two hours after it starts

The system SHALL move every `confirmed` reservation to `visited` within 15 minutes after its `starts_at` plus two hours. It SHALL NOT move a reservation in any other state.

#### Scenario: Dinner at 8 pm

- **WHEN** a reservation for 20:00 is `confirmed` and it is 22:15
- **THEN** the reservation is `visited`

#### Scenario: Never confirmed

- **WHEN** a reservation for 20:00 is still `requested` at 23:00
- **THEN** it stays `requested`

### Requirement: Staff manage reservations in the cloud

The cloud SHALL serve, under `/api/reservations`:
- a list filtered by date (in the organization's timezone) and optionally by status, ordered by `starts_at`, each entry including the contact's phone and attribution;
- reading one reservation;
- creating one, with `source` `phone`, `walk_in` or `staff`, stored directly as `confirmed`;
- confirming, declining (with optional reason), cancelling and marking no-show.

Reading SHALL require `reservations:read` and every change SHALL require `reservations:manage`. The edge box SHALL NOT serve these endpoints.

#### Scenario: Tonight's list

- **WHEN** a manager lists reservations for today
- **THEN** the response holds today's reservations in time order, in every state, with each guest's phone

#### Scenario: A phone booking entered by staff

- **WHEN** a manager creates a reservation with `source` `phone`
- **THEN** it is stored as `confirmed`

#### Scenario: A user without the permission

- **WHEN** a user without `reservations:manage` confirms a reservation
- **THEN** the response is `403` and nothing changes

#### Scenario: On the edge box

- **WHEN** any `/api/reservations` endpoint is called on the edge box
- **THEN** it is not served

### Requirement: Staff can look up a contact's history

The cloud SHALL serve `GET /api/contacts` (search by phone or name, paginated) and `GET /api/contacts/:id`, which SHALL return the contact, its first-touch attribution, its reservations newest first, and its feedback. Both SHALL require `reservations:read`.

#### Scenario: A regular

- **WHEN** a manager opens a contact with three past reservations and two feedback entries
- **THEN** the response lists all three reservations and both feedback entries

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
