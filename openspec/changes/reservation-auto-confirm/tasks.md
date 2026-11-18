## 0. Prerequisite

- [x] 0.1 Archive `reservations-crm` (all its tasks are checked) so that `reservations`, `customer-agent-access` and `marketing-events` exist under `openspec/specs/`. Verify with `openspec validate reservation-auto-confirm --strict` after archiving.

## 1. Settings

- [x] 1.1 Add `ReservationSettings` (`auto_confirm_enabled`, `max_party_size`, `min_notice_minutes`, `daily_guest_cap`, `schedule`) to the settings DTOs and views, with defaults applied on read when the key is absent. Verify with a repository test that an organization with no `reservations` key reads `false`/8/120/null/[].
- [x] 1.2 Add validation: the ranges, `daily_guest_cap` required when enabled, and the schedule through the ordering schedule validator. Add `UpdateReservations(merge)` in `OrganizationSettingsRepository`, and the `reservations` section to `PATCH /api/organization/settings`. Verify with service and handler tests for the spec scenarios "Turning auto-confirm on" and "Enabled without a cap".
- [x] 1.3 Add `settings:reservations:update`, granted to admin and manager. Verify with a handler test for the "Without the permission" scenario (403) and the permissions table test.

## 2. Deciding the status

- [x] 2.1 Add `reservation.Decide(policy, req, now, loc, confirmedGuests)`, which returns `confirmed`, or `requested` plus the first failed rule in the spec's order. Verify with a table test: an ordinary table, party size, notice, outside schedule (including a window past midnight), daily cap at exactly the cap and one over, and disabled.
- [x] 2.2 Add the migration pair `add_reservation_pending_reason` (cloud, nullable text with a check on the five values). Add `pending_reason` to the reservation DTO and repository, and let `NewFromCreateRequest` take the initial status and reason. Verify with `make migrate-up`/`migrate-down` on the test database and a repository round-trip test.
- [x] 2.3 In `ReservationService.Create`, for `source` `whatsapp`: load the policy. When it reaches the cap rule, take `pg_advisory_xact_lock` for the organization and day, sum that day's `confirmed` and `visited` party sizes, call `Decide`, and store the result. Verify with service tests for each spec scenario, and with a database-backed test of "Two requests for the last seats" running two creates concurrently.

## 3. Events and the agent tool

- [x] 3.1 Record no `reservation.confirmed` event when a reservation is created `confirmed` by the policy, and leave later transitions unchanged. Verify with event-service tests for "The policy confirms a WhatsApp reservation" and "Staff confirm a WhatsApp reservation".
- [x] 3.2 In the MCP `create_my_reservation`, return `status` and, when `requested`, `pending_reason`. Rewrite its description for both outcomes. Verify with tests in `tools_customer_reservations_test.go` for "Booking for six at eight" (with `pending_reason` `disabled`) and "A table the policy confirms".

## 4. Docs and verification

- [x] 4.1 Update `docs/api/reservations.md` (the lifecycle diagram, a WhatsApp reservation starting `confirmed`, `pending_reason`) and `docs/api/organization-settings.md` (the `reservations` section and its permission). Verify that both describe every field in the spec.
- [ ] 4.2 Run `make test` and `make test-acceptance` and confirm they pass. Then, against the local stack: enable the policy for Laguna through the API, book 4 people for tomorrow at noon through the customer MCP, and confirm it comes back `confirmed`, appears on the staff screen, and records no marketing event. Then book 10 people and confirm it comes back `requested` with `party_size`.
