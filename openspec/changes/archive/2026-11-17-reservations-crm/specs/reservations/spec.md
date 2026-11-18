## Purpose

Lets a restaurant record table reservation requests from guests, whether they arrive through the WhatsApp agent or are entered by staff, keep one contact per guest phone, and move each reservation from request to confirmed, visited, declined, cancelled or no-show.

## ADDED Requirements

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
