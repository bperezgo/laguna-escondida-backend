## Purpose

A durable, per-organization event bus in the cloud: services publish what
happened, in the same transaction as the change, and each registered
consumer reacts to it independently, at least once.

## ADDED Requirements

### Requirement: An event exists only if its change commits

A service SHALL publish a domain event inside the transaction of the change
it describes. When that transaction commits, the event SHALL be stored with
its id, name, organization, subject id, time and data. When it rolls back,
the event SHALL NOT exist and no consumer SHALL receive it. If storing the
event fails, the change SHALL fail. The bus SHALL run in the cloud only. On
the edge box, publishing SHALL do nothing.

#### Scenario: Committed change

- **WHEN** a delivery order is paid and the payment commits
- **THEN** one `order.paid` event is stored for that order and organization

#### Scenario: Rolled-back change

- **WHEN** paying an order fails after its event was published, and the transaction rolls back
- **THEN** no `order.paid` event exists for it

### Requirement: Each consumer receives each event at least once, on its own

Every consumer subscribed to an event's name when the event is published
SHALL receive it at least once, usually within 2 seconds of the commit.
Each consumer SHALL track its own progress: a consumer that fails SHALL be
retried, after a delay that starts at 1 second and doubles per attempt,
capped at 5 minutes. Other consumers of the same event, and later events,
SHALL NOT wait for it. After 20 failed attempts, the delivery SHALL be
marked `dead`, logged as an error with the event, consumer and last error,
and not retried. Delivery order is not guaranteed.

#### Scenario: Two consumers, one failing

- **WHEN** an event has two consumers and the first fails while the second succeeds
- **THEN** the second is marked done once, and only the first is retried

#### Scenario: A consumer that keeps failing

- **WHEN** a consumer fails the same event 20 times
- **THEN** that delivery is marked `dead`, an error is logged, and the consumer keeps receiving newer events

### Requirement: Consumers are idempotent and run in the event's organization

A consumer SHALL be able to receive the same event more than once without
repeating its effect, and SHALL use the event id to tell. A consumer SHALL
run acting for the event's organization only, under the same isolation as a
request of that organization. A failure for one organization SHALL NOT stop
or delay delivery for the others.

#### Scenario: Redelivered event

- **WHEN** a consumer's work commits but marking the delivery done fails, so the event is delivered again
- **THEN** the consumer's effect is applied once

#### Scenario: Two organizations

- **WHEN** organization A's consumer fails and organization B publishes an event
- **THEN** B's event is delivered without waiting for A

### Requirement: Consumers can be added without changing publishers

A consumer SHALL declare a unique name and the event names it handles, and
SHALL be added by registering it. No publishing service SHALL change. A
newly registered consumer SHALL receive events published after it was
registered, not earlier ones.

#### Scenario: A new consumer

- **WHEN** a consumer for `order.paid` is registered and deployed
- **THEN** it receives the next paid order's event, and `OrderService` is unchanged

### Requirement: The cloud publishes four events

The cloud SHALL publish:

- `order.created` when an order is created in the cloud, with its id, type,
  total, customer name and phone, and delivery zone.
- `order.paid` when an order is paid in the cloud, with the same data plus
  the amount paid.
- `reservation.requested` when a reservation is created `requested`, with
  its id, guest name and phone, party size, start, source and pending
  reason.
- `reservation.confirmed` when a reservation becomes `confirmed`, at
  creation or by a transition, with the same data and `confirmed_by`. This
  is `auto` for a WhatsApp reservation stored `confirmed` at creation, and
  `staff` otherwise.

Orders replicated from the edge box SHALL NOT publish these events. The
existing live staff screens SHALL keep working as before.

#### Scenario: Auto-confirmed reservation

- **WHEN** the auto-confirm policy stores a WhatsApp reservation `confirmed`
- **THEN** one `reservation.confirmed` event is published with `confirmed_by` `auto`, and no `reservation.requested`

#### Scenario: Staff confirms a request

- **WHEN** staff confirm a `requested` reservation
- **THEN** one `reservation.confirmed` event is published with `confirmed_by` `staff`

#### Scenario: Kitchen board unaffected

- **WHEN** an order is created in the cloud
- **THEN** the kitchen board receives it live, as before, and an `order.created` event is also stored
