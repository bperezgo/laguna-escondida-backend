## Why

marketing-app is adding an outcome ledger: each business configures which results matter, and
marketing-app counts them and relates them to what the agent cost. A table reservation that was
honoured is the result a restaurant cares about most, but this backend reports none of it. Today
"a request, a visit and a no-show produce no event", so marketing-app can see a reservation
confirmed and cancelled, and never whether the guest came.

## What Changes

- Record `reservation.visited` when an agent reservation enters `visited`, and `reservation.no_show`
  when it enters `no_show`, in the same transaction as the change, through the existing outbox and on
  the existing `/backend/notifications` envelope.
- Their `data` is the one `reservation.*` already carries, plus `previous_status`, so a receiver can
  tell a correction from a first outcome.
- `visited` is presumed, not observed: the job sets it two hours after `starts_at`, and staff can
  still correct it to `no_show` until feedback is requested. A guest may therefore receive
  `reservation.visited` and later `reservation.no_show`. The spec says so, so no receiver treats
  `visited` as proof the guest came.
- No new endpoint, signature, retry rule or configuration.

### Non-goals

- A `repeat_customer` or other CRM-derived event. marketing-app can derive it from the events it
  already receives.
- Events for reservations without a `channel_id` (staff-entered), as for every other event.
- Sending anything to guests. These events carry facts only; marketing-app decides what to do.

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `marketing-events`: two more reservation events and the rule for a corrected attendance.

## Impact

- Backend: the reservation transitions to `visited` (job) and `no_show` (staff) record an event;
  the marketing-events recorder and the notifications body gain `previous_status`.
- `docs/api/marketing-events.md`: the event table, and the sentence "a visit and a no-show produce
  no event".
- No migration is expected: the outbox stores the body as is.
- marketing-app: consumes these in its `outcome-ledger` change. It must accept an event type it does
  not know, so either side can ship first.
