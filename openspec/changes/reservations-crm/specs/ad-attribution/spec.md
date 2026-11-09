## Purpose

Keeps, in the backend, which messaging channel, conversation and ad brought each reservation, agent order and guest, as replicated from the WhatsApp agent runtime, so the CRM can tell where guests come from without asking marketing-app.

## ADDED Requirements

### Requirement: Reservations and agent orders carry their origin

A reservation and an order SHALL each be able to store an origin: `channel_id`, `conversation_id`, and an attribution of `source`, `ad_id`, `clicked_at`, `campaign_id` and `campaign_name`. Every field SHALL be optional. The create endpoints for reservations and orders SHALL accept the origin. The origin SHALL be returned when the reservation or order is read by staff, and SHALL NOT be changed by any later update.

#### Scenario: A reservation from an ad

- **WHEN** a reservation is created with origin channel `ch_1`, conversation `conv_9`, source `meta_ctwa`, ad id `120210000000001` and a click time
- **THEN** reading the reservation returns that origin

#### Scenario: An order is updated

- **WHEN** an agent order with an origin has its lines replaced
- **THEN** its origin is unchanged

### Requirement: Invalid attribution never blocks a booking

Each attribution field SHALL be validated on its own: `source` matches `^[a-z0-9_]{1,32}$`; `ad_id` and `campaign_id` are at most 64 characters; `campaign_name` at most 255; `clicked_at` is RFC 3339 and not more than 5 minutes in the future. An invalid attribution field SHALL be dropped and logged, and the reservation or order SHALL still be created with the remaining fields. `channel_id` and `conversation_id` SHALL be at most 128 characters; when longer, the request SHALL fail with a validation error.

#### Scenario: A garbled click time

- **WHEN** a reservation is created with source `meta_ctwa` and `clicked_at` `yesterday`
- **THEN** the reservation is created with source `meta_ctwa` and no click time

### Requirement: A contact keeps its first touch

When a contact is created from a reservation or order that has an attribution, the contact's first touch SHALL be that attribution, and it SHALL never change afterwards. A contact created without one SHALL have none, and SHALL take the attribution of the first later reservation or order that has one.

#### Scenario: Came from an ad, came back organically

- **WHEN** a guest's first reservation came from ad `A` and their second had no attribution
- **THEN** the contact's first touch is still ad `A`

### Requirement: The ad click identifier is not stored

The system SHALL NOT accept or store a click identifier (`ctwa_clid` or any equivalent). It SHALL be ignored if sent.

#### Scenario: A click id is sent

- **WHEN** a reservation's origin includes a `ctwa_clid` field
- **THEN** the reservation is created and no click identifier is stored
