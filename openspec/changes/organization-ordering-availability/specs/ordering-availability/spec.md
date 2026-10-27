## Purpose

Lets a restaurant say when it takes delivery and takeaway orders, with a weekly schedule and an emergency pause per order type, and tells staff and the customer agent whether each type is available right now.

## ADDED Requirements

### Requirement: An organization has ordering settings for delivery and takeaway

An organization SHALL have ordering settings for each of the order types `delivery` and `takeaway`. Each SHALL have:
- `paused`: whether orders of that type are stopped, default `false`
- `paused_reason`: optional text shown to staff and the agent, at most 255 characters, `null` when not paused
- `schedule`: a list of opening windows, each with a `day` (`mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`), an `opens` time and a `closes` time in 24-hour `HH:MM`, read in the organization's timezone. The default is an empty list.

An organization that has never stored ordering settings SHALL behave as if both types had `paused: false` and an empty schedule. `dine_in` SHALL have no ordering settings.

#### Scenario: An organization with nothing configured

- **WHEN** the settings of an organization that never saved any are read
- **THEN** both `delivery` and `takeaway` are `paused: false`, `paused_reason: null`, `schedule: []`

### Requirement: Schedules are validated

Every schedule SHALL be validated as a whole, and an invalid one SHALL be rejected with a validation error naming the order type and the offending window, storing nothing:
- `day` SHALL be one of the seven day codes
- `opens` and `closes` SHALL be valid `HH:MM` times from `00:00` to `23:59`
- `opens` and `closes` SHALL differ
- a window whose `closes` is earlier than its `opens` SHALL run past midnight into the next day
- two windows of the same order type SHALL NOT overlap, including a past-midnight window overlapping the next day's windows
- a type SHALL have at most 21 windows

#### Scenario: A window past midnight

- **WHEN** the delivery schedule is saved with `{"day": "fri", "opens": "18:00", "closes": "02:00"}`
- **THEN** it is stored, and delivery is open from Friday 18:00 to Saturday 02:00

#### Scenario: Overlapping windows

- **WHEN** the takeaway schedule is saved with Monday 11:00–15:00 and Monday 14:00–18:00
- **THEN** the request fails with a validation error and the stored schedule is unchanged

#### Scenario: Overlap across midnight

- **WHEN** the delivery schedule is saved with Friday 18:00–02:00 and Saturday 01:00–04:00
- **THEN** the request fails with a validation error

#### Scenario: Equal times

- **WHEN** a window has `opens: "12:00"` and `closes: "12:00"`
- **THEN** the request fails with a validation error

### Requirement: Availability is computed from the pause and the schedule

At a given instant, an order type SHALL be available when it is not paused and either its schedule is empty or the instant falls inside one of its windows. The organization's timezone SHALL be used to place the instant on a day and time. A window SHALL include its `opens` minute and exclude its `closes` minute.

The availability of each type SHALL report:
- `available`: `true` or `false`
- `reason`: `null` when available, `paused` when paused, otherwise `outside_hours`. `paused` SHALL take precedence over `outside_hours`.
- `paused_reason`: the stored reason when paused, otherwise `null`
- `closes_at`: when available through a window, the instant that window closes, otherwise `null`. It is `null` when available because the schedule is empty.
- `next_opens_at`: when `outside_hours`, the instant the next window opens within the next seven days, otherwise `null`. It is `null` when paused, because a pause has no planned end.

Instants SHALL be returned as RFC 3339 timestamps carrying the organization's UTC offset.

#### Scenario: Inside a window

- **WHEN** the timezone is `America/Bogota`, the delivery schedule has Monday 18:00–22:30, and it is Monday 19:00 in Bogotá
- **THEN** delivery is available with `closes_at` Monday 22:30 `-05:00`

#### Scenario: Before opening

- **WHEN** the same schedule is read on Monday at 17:00
- **THEN** delivery is unavailable with reason `outside_hours` and `next_opens_at` Monday 18:00 `-05:00`

#### Scenario: Exactly at closing

- **WHEN** the same schedule is read on Monday at 22:30
- **THEN** delivery is unavailable with reason `outside_hours` and `next_opens_at` the next window's opening

#### Scenario: Paused inside a window

- **WHEN** delivery is paused with reason "cocina saturada" and it is inside a delivery window
- **THEN** delivery is unavailable with reason `paused`, `paused_reason` "cocina saturada" and `next_opens_at: null`

#### Scenario: Empty schedule

- **WHEN** takeaway is not paused and has no schedule
- **THEN** takeaway is available at any time with `closes_at: null`

#### Scenario: The early hours of a past-midnight window

- **WHEN** the delivery schedule has Friday 18:00–02:00 and it is Saturday 01:30 in the organization's timezone
- **THEN** delivery is available with `closes_at` Saturday 02:00

### Requirement: Staff and the agent read the ordering settings of their organization

`GET /api/organization/settings` SHALL return, for the organization the request runs as:
- `ordering`: the stored settings of `delivery` and `takeaway`
- `availability`: the availability of `delivery` and `takeaway` at the moment of the request
- `timezone`: the organization's timezone

It SHALL require the `ordering:read` permission and respond `403` without it. It SHALL accept a user's token or an organization API key. It SHALL NOT return settings of any other organization.

#### Scenario: Reading the settings

- **WHEN** a manager reads `GET /api/organization/settings`
- **THEN** the response is `200` with `ordering`, `availability` and `timezone` for the manager's organization

#### Scenario: Another organization's settings are not visible

- **WHEN** organizations A and B have different schedules and a user of A reads the settings
- **THEN** only A's settings and availability are returned

#### Scenario: Without permission

- **WHEN** a user without `ordering:read` reads the settings
- **THEN** the response is `403`

### Requirement: Staff change the pause and the schedule with separate permissions

`PATCH /api/organization/settings` SHALL change the ordering settings of the organization the request runs as, and only the parts present in the body:
- `ordering.<type>.paused` and `ordering.<type>.paused_reason` SHALL require `ordering:pause`
- `ordering.<type>.schedule` SHALL replace that type's whole schedule and SHALL require `ordering:manage`. An empty list SHALL clear it.
- a body that touches both SHALL require both permissions. When either is missing, the request SHALL be refused with `403` and nothing SHALL change.
- setting `paused` to `false` SHALL clear `paused_reason`
- a type absent from the body SHALL be left unchanged

The response SHALL be `200` in the same shape as `GET`. An invalid body SHALL be rejected with a validation error and nothing SHALL change.

#### Scenario: Pausing deliveries during a rush

- **WHEN** a user with `ordering:pause` patches `{"ordering": {"delivery": {"paused": true, "paused_reason": "cocina saturada"}}}`
- **THEN** delivery is paused with that reason, takeaway and the delivery schedule are unchanged, and the response shows delivery unavailable with reason `paused`

#### Scenario: Resuming clears the reason

- **WHEN** delivery is paused with a reason and the user patches `{"ordering": {"delivery": {"paused": false}}}`
- **THEN** delivery is not paused and `paused_reason` is `null`

#### Scenario: Pause permission alone cannot change hours

- **WHEN** a user with `ordering:pause` but not `ordering:manage` patches a takeaway schedule
- **THEN** the response is `403` and the schedule is unchanged

#### Scenario: Mixed body without both permissions

- **WHEN** a user with only `ordering:manage` patches a body that changes the delivery schedule and pauses takeaway
- **THEN** the response is `403` and neither change is stored

#### Scenario: Clearing a schedule

- **WHEN** a user with `ordering:manage` patches `{"ordering": {"takeaway": {"schedule": []}}}`
- **THEN** takeaway has no schedule and is available whenever it is not paused

### Requirement: Ordering permissions are granted by role

The system SHALL define the permissions `ordering:read`, `ordering:pause` and `ordering:manage`.
- `admin` SHALL have all three.
- `manager` SHALL have all three.
- `accountant` SHALL have `ordering:read`.
- `agent` SHALL have `ordering:read` only.

#### Scenario: The agent cannot pause

- **WHEN** a user with only the `agent` role patches `{"ordering": {"delivery": {"paused": true}}}`
- **THEN** the response is `403` and nothing changes

### Requirement: Ordering settings are served in the cloud only

The `/api/organization/settings` routes SHALL be served only in cloud mode. An edge box SHALL NOT serve them. Its organization row carries no ordering settings, so the box behaves as if nothing was configured.

#### Scenario: Edge mode

- **WHEN** the backend runs in edge mode
- **THEN** `/api/organization/settings` is not served

### Requirement: A saved change takes effect promptly

After a successful `PATCH`, the next availability read and the next order created on the instance that saved the change SHALL use the new settings. Every other instance SHALL use them within 30 seconds.

#### Scenario: Pause then order

- **WHEN** delivery is paused and, immediately after, a delivery order is created on the same instance
- **THEN** the order is refused as paused
