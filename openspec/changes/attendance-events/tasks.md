## 1. Event shape

- [x] 1.1 Add `MarketingEventReservationVisited` and `MarketingEventReservationNoShow` to `backend/internal/domain/dto/marketing_event.go`; verify `go build ./...` passes from `backend/`
- [x] 1.2 Add `attendanceData` (reservationData + `previous_status`) and `NewAttendanceEvent` to the marketing_event aggregate; verify `marketing_event_service_test.go` (fixtures `reservation_visited.json`, `reservation_no_show.json`; no aggregate tests per `backend/AGENTS.md`) checks both types' body: `data` keys, `status`, `previous_status`, `occurred_at` in the org's offset, and that `NewReservationEvent` bytes are unchanged

## 2. Recording

- [x] 2.1 Add `RecordReservationAttendance(ctx, reservation, previous, at)` to `ports.MarketingEventRecorder`, implement it in `MarketingEventService` (no-op without `channel_id`, for other statuses, when disabled or without a secret), and regenerate the mocks; verify `marketing_event_service_test.go` covers recorded, no channel, no secret, and a non-attendance status
- [x] 2.2 In `ReservationService.transition`, record attendance for `visited`/`no_show` without the `GuestIsNotified` guard, passing `current.Status` as `previous`; keep the guard for the other statuses; verify tests: confirmed→no_show after `starts_at` records one event, visited→no_show records `previous_status` `visited`, requested→no_show records one, a rollback leaves none
- [x] 2.3 Wrap each row of `ReservationService.MarkVisited` in `unitOfWork.Do` with the update and the attendance event; keep the transition-conflict skip and call `live.ReservationChanged` after commit; verify tests: one event per visited row with `previous_status` `confirmed`, a skipped row records none, a failing event append rolls back the status change

## 3. Acceptance and contract

- [x] 3.1 Cover the outbox rows in `backend/test/acceptance/reservations/` (job marks visited, staff correct to no-show, staff-entered reservation records none); run with `DB_HOST=` prefix and verify they pass
- [x] 3.2 Update `backend/docs/api/marketing-events.md`: add both rows to the event table, replace "A request, a visit and a no-show produce no event" with "A request produces no event", document `previous_status` and the visited-is-a-presumption rule with a `reservation.no_show` example; verify no remaining sentence says a visit or no-show produces no event
- [x] 3.3 From `backend/`, run `make lint` and `make test` (piped through `| tail -40`) and verify both are clean
