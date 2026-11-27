## Context

See proposal.md for why. Observed in `backend/`:

- `ReservationService.transition` (`internal/domain/service/reservation_service.go`) runs staff changes in `unitOfWork.Do` and calls `events.RecordReservation` only `if aggregate.GuestIsNotified(now)`, i.e. only while `starts_at` is still in the future. `MarkNoShow` goes through it, but a no-show is always marked after `starts_at`, so the guard would swallow every attendance event.
- `ReservationService.MarkVisited` (the job) calls `reservations.Update` per row with **no** unit of work and never touches `events`. The spec requires the event in the same transaction as the change.
- `marketingEvent.ReservationEventType` maps `confirmed`/`declined`/`cancelled` only; `reservationData` has no `previous_status`.
- The outbox stores the body as JSON, so a new field and two new types need no migration.

## Goals / Non-Goals

**Goals:**
- Both attendance events recorded atomically with the status change, by the job and by staff.
- `previous_status` on the two new events.

**Non-Goals:**
- Adding `previous_status` to `reservation.confirmed|declined|cancelled`. Their contract is unchanged.
- Changing the transition rules or the job schedule.

## Decisions

**1. Attendance bypasses the `GuestIsNotified` guard.** The guard exists so a guest is not messaged about a booking already over. Attendance events carry facts for a ledger and message nobody, per the proposal. In `transition`, the guard stays for confirmed/declined/cancelled; a status of `visited` or `no_show` records regardless of time. Alternative: drop the guard everywhere. Rejected: it would send late "cancelled" messages for staff clean-ups.

**2. `previous_status` is a separate argument, not a DTO field.** `RecordReservation(ctx, reservation, at)` gains no parameter for the old events; a new `RecordReservationAttendance(ctx, reservation, previous, at)` on `ports.MarketingEventRecorder` records `visited`/`no_show` and is a no-op for other statuses. Alternative: add `PreviousStatus` to `dto.Reservation`. Rejected: it is not persisted state, and it would leak into every API response. The mock in `internal/domain/ports/mocks` is regenerated.

**3. A separate payload type.** `attendanceData` embeds `reservationData` and adds `PreviousStatus string \`json:"previous_status"\``. `NewReservationEvent` is untouched, so existing event bytes do not change. `newNotification`'s type constraint gains `attendanceData`. `ReservationEventType` is not extended: attendance has its own constructor, `NewAttendanceEvent`. Constants `MarketingEventReservationVisited = "reservation.visited"` and `MarketingEventReservationNoShow = "reservation.no_show"` join `dto/marketing_event.go`.

**4. `MarkVisited` wraps each reservation in `unitOfWork.Do`.** One transaction per row (not one for the batch), so a failed row rolls back alone and the others still move. The `ErrReservationTransition` skip ("staff changed it since the list was read") keeps working: it returns from inside the closure as a sentinel the loop treats as a skip, and no event is recorded for a skipped row. `s.live.ReservationChanged` stays outside the transaction, after commit. Alternative: one transaction for the whole batch. Rejected: a single bad row would block every visit.

**5. `occurred_at` is `now` of the change** (the service's `s.now()`), the same value written to `updated_at`, so out-of-order delivery is resolved by time (spec: "Visited is a presumption…").

**6. Staff `no_show` from `requested`** (a request nobody answered, once its time has come) also records `reservation.no_show` with `previous_status` `requested`. It is a real attendance outcome and the spec says "enters `no_show`" without exception.

## Risks / Trade-offs

- [Receiver treats `visited` as proof] → docs say "presumption" in the event table and next to the example.
- [`visited` then `no_show` for one reservation] → by design; `previous_status` and `occurred_at` let marketing-app order it. No withdrawal event.
- [Wrapping `MarkVisited` in a transaction changes its error path] → test the skip-on-conflict and rollback cases explicitly.
- [Old marketing-app answers `404` to the new types] → the outbox retries and expires after 72 hours; the proposal already requires marketing-app to accept unknown types.

## Implementation map

- `backend/internal/domain/dto/marketing_event.go`: two constants
- `backend/internal/domain/aggregate/marketing_event/marketing_event.go` (+ its test): `attendanceData`, `NewAttendanceEvent`
- `backend/internal/domain/ports/marketing_events.go` and `ports/mocks/`: `RecordReservationAttendance`
- `backend/internal/domain/service/marketing_event_service.go` (+ `marketing_event_service_test.go`)
- `backend/internal/domain/service/reservation_service.go` (+ `reservation_service_test.go`): `transition`, `MarkVisited`
- `backend/test/acceptance/reservations/` (if it covers outbox rows)
- `backend/docs/api/marketing-events.md`
