## MODIFIED Requirements

### Requirement: The agent takes a reservation for the calling customer

`create_my_reservation` SHALL take a `name`, a `party_size`, a `starts_at` (date and time with offset) and optional `notes`, and SHALL create a reservation with `source` `whatsapp`, the header phone and the origin from the headers, stored `confirmed` or `requested` as the organization's auto-confirm policy decides. It SHALL require `X-Channel-Id`; without it the tool SHALL fail with an error saying the channel is missing, and SHALL NOT reach the backend. The tool SHALL return the reservation's id, name, party size, time and status, and, when the status is `requested`, its `pending_reason`. Its description SHALL tell the agent that a `confirmed` reservation is confirmed and that staff will confirm a `requested` one. Validation failures SHALL be returned as tool errors naming the field.

#### Scenario: Booking for six at eight

- **WHEN** `create_my_reservation` is called with header phone `573005551234`, channel `ch_1`, name "Ana", party size 6 and tomorrow at 20:00, and the organization's policy is off
- **THEN** a `requested` reservation for `+573005551234` with that channel exists, and the tool returns it with `pending_reason` `disabled`

#### Scenario: A table the policy confirms

- **WHEN** `create_my_reservation` is called for 4 people tomorrow at noon and the organization's policy accepts it
- **THEN** the tool returns the reservation with status `confirmed` and no `pending_reason`

#### Scenario: No channel header

- **WHEN** `create_my_reservation` is called without `X-Channel-Id`
- **THEN** the tool returns an error and no reservation is created
