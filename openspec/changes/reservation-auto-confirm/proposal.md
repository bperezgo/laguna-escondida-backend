## Why

Every table a guest asks for on WhatsApp is created `requested` and waits
for staff to confirm it, so the guest leaves the chat without an answer. For
an ordinary table, such as four people tomorrow at noon, staff confirm it
without thinking. The restaurant wants the agent to confirm those on the
spot, and leave staff only the bookings that need judgment.

## What Changes

- Each organization gets a `reservations` section in
  `/api/organization/settings` with an auto-confirm policy:
  - `auto_confirm_enabled`, off by default
  - `max_party_size`, 8 by default
  - `min_notice_minutes`, 120 by default
  - `daily_guest_cap`, required when the policy is enabled
  - `schedule`, optional weekly windows in the existing `OpeningWindow` format

  Admins and managers change it with a new `settings:reservations:update`
  permission.
- `create_my_reservation`, and `POST /api/reservations` with `source`
  `whatsapp`, stores the reservation `confirmed` when the policy accepts it,
  and `requested` otherwise, recording the first rule it failed:
  `disabled`, `party_size`, `notice`, `outside_schedule` or `daily_cap`.
  The policy never refuses a booking. It only decides who confirms it.
- The daily cap counts the guests of that day's `confirmed` and `visited`
  reservations from every source. Bookings for the same day are checked one
  at a time, so two concurrent requests can't both take the last seats.
- The tool returns `status` (`confirmed` or `requested`) and, when pending,
  a `pending_reason`. Its description tells the agent to say a confirmed
  table is confirmed, and a requested one is waiting for staff.
- An auto-confirmed reservation records **no** `reservation.confirmed`
  event, because the agent has already told the guest in the chat. Staff
  screens still receive the change live.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `reservations`: adds the auto-confirm policy settings and the rule that a
  WhatsApp request meeting it is stored `confirmed`.
- `customer-agent-access`: `create_my_reservation` returns `confirmed` or
  `requested` with a reason, instead of always `requested`.
- `marketing-events`: an auto-confirmed reservation records no
  `reservation.confirmed` event.

These capabilities are introduced by `reservations-crm`. That change must be
archived before this one.

## Impact

- **Backend:**
  - `internal/domain`: the policy check on create, settings DTOs and
    validation, the permission, and the event rule.
  - A migration for the new settings section.
  - `/api/organization/settings` gains the section.
  - The MCP `create_my_reservation` view changes.
  - `docs/api/reservations.md` and the settings docs.
- **laguna-escondida-frontend:** the settings screen to edit the policy is a
  follow-up in that repo. Until it exists, the policy is set through the API.
- **marketing-app:** the reservations agent's `agent.md` must handle both
  statuses before the policy is turned on for an organization.
- **Rollout:** auto-confirm is off by default, so deploying this changes
  nothing until an admin turns it on.
