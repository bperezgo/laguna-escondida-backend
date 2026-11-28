## Context

- `ReservationService.Create` (`internal/domain/service/reservation_service.go`)
  validates the request, then, in one unit of work, touches the contact,
  builds the aggregate (`reservation.NewFromCreateRequest`), stores it and
  records its marketing event (`RecordReservation`). After commit it tells the
  live staff screens.
- `RecordReservation` maps the reservation's status to an event type. A
  WhatsApp reservation created `confirmed` would therefore record
  `reservation.confirmed`, and marketing-app would message the guest a second
  time, after the agent has already told them.
- Organization settings are keys of one JSON document per organization
  (`OrganizationSettingsRepository`). `UpdateFeedback` merges only its own
  key, and `OpeningWindow` plus the ordering schedule validation already
  model weekly hours in the organization's timezone.
- The backend has no dine-in hours and no table or seating model.

## Goals / Non-Goals

**Goals:**
- Decide `confirmed` or `requested` inside the create transaction, from
  settings staff control, without ever refusing a booking.
- Never confirm more guests than the cap, even under concurrent requests.

**Non-Goals:**
- Tables, time slots, or seating per hour.
- Changing staff-created reservations or any existing transition.
- The settings screen in `laguna-escondida-frontend`, which is a follow-up
  there.
- Telling the guest why a booking is pending. That's the agent's wording, in
  marketing-app.

## Decisions

**D1. The policy is a pure domain function evaluated inside the unit of
work.**
`reservation.Decide(policy, req, now, loc, confirmedGuests) (status,
pendingReason)` checks the rules in the spec's order and returns the first
failure. The service loads the policy and the day's guest count inside the
same transaction, then passes the result to `NewFromCreateRequest`, which
gains the initial status. A pure function makes each rule table-testable
without a database.

**D2. A transaction advisory lock per organization and day serializes the
cap.**
Before counting, the service takes `pg_advisory_xact_lock(hashtext(org_id
|| ':' || day))`, the same pattern as `sync_outbox_repository.go`. The lock
is released at commit, so two requests for the same day can't both read the
same count. Requests for other days aren't blocked. Alternatives:
- A counter row per day with `SELECT … FOR UPDATE`. It needs a table and
  upkeep, for the same effect.
- `SERIALIZABLE` isolation. Its retries would surface as failures to the
  agent.

The lock only matters when the policy reaches the cap rule. Disabled
organizations skip both the lock and the count.

**D3. `pending_reason` is stored on the reservation.**
It's a nullable column, set only when a WhatsApp reservation is created
`requested`, and never changed afterwards. Staff screens can show why a
request is waiting ("grupo grande", "día lleno"), and the MCP view returns
it. It needs one cloud migration pair, `add_reservation_pending_reason`.

**D4. Event suppression is decided at creation.**
`RecordReservation` gains the knowledge that it's recording a creation, for
example through a separate `RecordReservationCreated`. On creation, a
`confirmed` status records nothing, and every later transition records as
it does today. The event rule stays in the event service, not the
reservation service.

**D5. Settings use the existing document and permission pattern.**
- The settings document gets a `reservations` key with
  `UpdateReservations(merge)`, like `UpdateFeedback`.
- Defaults are applied on read when the key is absent, so there's no
  backfill.
- The new `settings:reservations:update` permission is granted to admin and
  manager, next to `settings:feedback:update`.
- The schedule reuses the ordering schedule validator.

## Risks / Trade-offs

- [The daily cap is coarse: 120 guests a day can still all arrive at 1
  p.m.] → It's the agreed v1. A per-slot cap is a later change that can
  reuse `Decide` and the same lock, keyed by slot.
- [Staff could lower the cap below what's already confirmed.] → Nothing is
  undone. Later requests that day go to `requested`.
- [The agent might still say "pendiente" for a confirmed table, or the
  reverse.] → marketing-app updates the reservations `agent.md` and its
  evals to read `status` before any organization enables the policy. The
  policy is off by default.

## Migration Plan

1. Apply migration `add_reservation_pending_reason` (cloud), then deploy.
   Nothing changes for any organization while the policy is off.
2. Ship marketing-app's agent update for both statuses.
3. An admin enables the policy for one organization through the API, and
   watches the staff screen for a day.
4. Roll back by turning the policy off. Reservations it confirmed stay
   confirmed. The column's down migration only drops it.

## Open Questions

- Should the staff screen's reservations list filter by `pending_reason`?
  This is frontend wording and filtering, and doesn't change this API.
