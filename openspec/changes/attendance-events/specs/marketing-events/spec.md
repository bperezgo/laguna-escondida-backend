## ADDED Requirements

### Requirement: Attendance changes produce events

The system SHALL record `reservation.visited` when an agent reservation enters `visited`, and
`reservation.no_show` when it enters `no_show`, whether the change was made by the visit job or by
staff. Each event SHALL be posted to `<events URL>/backend/notifications` with the notifications
envelope, and its `data` SHALL hold what `reservation.*` events hold (`reservation_id`, `name`,
`party_size`, `starts_at`, `status`) plus `previous_status`, the status the reservation left.
`occurred_at` SHALL be the time of the change. A reservation that is not an agent's (no
`channel_id`) SHALL record no attendance event, and the change SHALL still succeed.

#### Scenario: The visit job marks a confirmed reservation visited

- **WHEN** a confirmed agent reservation passes two hours after its start and the job moves it to `visited`
- **THEN** one `reservation.visited` event is recorded with `status` `visited` and `previous_status` `confirmed`

#### Scenario: Staff mark a no-show

- **WHEN** staff mark a confirmed agent reservation `no_show`
- **THEN** one `reservation.no_show` event is recorded with `status` `no_show` and `previous_status` `confirmed`

#### Scenario: A visit is corrected to a no-show

- **WHEN** a reservation became `visited` overnight, no feedback has been requested, and staff mark it `no_show`
- **THEN** a `reservation.no_show` event is recorded with `previous_status` `visited`
- **AND** the earlier `reservation.visited` event is neither withdrawn nor changed

#### Scenario: A reservation staff took at the till

- **WHEN** a reservation with no `channel_id` becomes `visited`
- **THEN** no event is recorded and the reservation is `visited`

#### Scenario: The change rolls back

- **WHEN** marking a reservation `no_show` fails and the change rolls back
- **THEN** no `reservation.no_show` event exists

### Requirement: Visited is a presumption a later event may correct

The system SHALL describe `reservation.visited` as the system's presumption that the guest came, not
a confirmation, and SHALL deliver events at least once and unordered as for every event. A receiver
SHALL be able to order a reservation's attendance events by `occurred_at`, and to recognise a
correction by `previous_status`.

#### Scenario: Events arrive out of order

- **WHEN** `reservation.no_show` (previous status `visited`) is delivered before `reservation.visited`
- **THEN** both carry the `occurred_at` of their own change, so the later one by time is the reservation's current attendance

#### Scenario: Redelivery

- **WHEN** the receiver is down and an attendance event is retried
- **THEN** each retry carries the same `event_id` and body
