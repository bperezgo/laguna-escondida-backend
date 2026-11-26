## Purpose

Receives a demo request from a prospect on the public marketing site, stores it with the ad attribution that brought them, limits abuse, alerts the platform operator, and lets the operator follow each lead up.

## ADDED Requirements

### Requirement: A lead and its fields

A lead SHALL have an id (UUID), a `name` (1–100 characters after trimming), a `restaurant_name` (1–120 characters after trimming), a `phone` stored in E.164 form, a `status`, optional operator `notes` (at most 2000 characters), the time the prospect gave consent, and a creation time. A lead MAY carry an attribution: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `fbclid`, `gclid`, `landing_url`, `referrer` and `event_id`. A lead SHALL NOT belong to any organization. The system SHALL NOT store the sender's IP address with the lead.

#### Scenario: A lead from an ad

- **WHEN** a lead is submitted with name `Ana Ruiz`, restaurant `El Fogón`, phone `300 123 4567`, consent, `utm_source` `facebook`, `utm_campaign` `lanzamiento` and an `fbclid`
- **THEN** the lead is stored with phone `+573001234567`, status `new`, that attribution, and no organization

### Requirement: The public can submit a lead without signing in

`POST /api/public/leads` SHALL accept a JSON body and SHALL require no credential. On success it SHALL respond `201 Created` with `{"status": "received"}` and SHALL NOT return the lead's id or any stored field. The endpoint SHALL be callable from a browser on another origin. It SHALL be served only by the cloud; on an edge box it SHALL respond `404`.

#### Scenario: A valid submission

- **WHEN** a browser on the marketing site's origin posts a valid lead with no credential
- **THEN** the response is `201` with `{"status": "received"}` and one lead exists

#### Scenario: On the edge box

- **WHEN** the same request is sent to an edge box
- **THEN** the response is `404` and no lead is stored

### Requirement: Required fields are validated and reported by field

A submission SHALL be rejected with `400 Bad Request`, naming the offending field, and SHALL store nothing, when: `name` or `restaurant_name` is empty or too long; `phone` is not a valid number (a number without a country code SHALL be read as Colombian, and a number starting with `+` as international); or `consent` is not `true`. A body that is not valid JSON SHALL be rejected with `400`. A body larger than 16 KB SHALL be rejected with `413`.

#### Scenario: No consent

- **WHEN** a lead is submitted with valid fields and `consent` `false`
- **THEN** the response is `400` naming `consent` and no lead is stored

#### Scenario: A phone that is not a number

- **WHEN** a lead is submitted with phone `12345`
- **THEN** the response is `400` naming `phone` and no lead is stored

#### Scenario: A foreign number

- **WHEN** a lead is submitted with phone `+52 55 1234 5678`
- **THEN** the lead is stored with phone `+525512345678`

### Requirement: Invalid attribution never blocks a lead

Each attribution field SHALL be validated on its own: the five `utm_*` fields are at most 255 characters; `fbclid` and `gclid` at most 512; `landing_url` and `referrer` at most 2048 and, when present, absolute `http` or `https` URLs; `event_id` matches `^[A-Za-z0-9-]{8,64}$`. An invalid attribution field SHALL be dropped and logged, and the lead SHALL still be stored with the remaining fields.

#### Scenario: A garbled landing URL

- **WHEN** a lead is submitted with `utm_source` `google` and `landing_url` `javascript:alert(1)`
- **THEN** the lead is stored with `utm_source` `google` and no landing URL

### Requirement: Automated and repeated submissions are contained

The submission form carries a field named `website` that a person never fills. A submission with a non-empty `website` SHALL be answered `201` with the normal body and SHALL store nothing and alert no one.

A submission whose phone matches a lead created in the previous 24 hours SHALL be answered `201` with the normal body and SHALL NOT create a second lead or a second alert.

The system SHALL accept at most 5 submissions per client address per hour, and at most 300 stored leads per hour across all clients. A submission beyond either limit SHALL be rejected with `429 Too Many Requests` and a `Retry-After` header, and SHALL store nothing.

#### Scenario: A bot fills every field

- **WHEN** a submission arrives with valid fields and `website` `http://spam.example`
- **THEN** the response is `201` with `{"status": "received"}`, no lead is stored and no alert is sent

#### Scenario: A double submit

- **WHEN** the same prospect submits twice within a minute with the same phone
- **THEN** both responses are `201`, one lead exists and one alert is sent

#### Scenario: One address floods the form

- **WHEN** one client address sends a sixth valid submission within an hour, each with a different phone
- **THEN** the sixth response is `429` with `Retry-After`, and five leads exist

### Requirement: The operator is alerted about each new lead

When a lead is stored and a lead alert destination is configured, the system SHALL send the operator one message with the lead's name, restaurant, phone, `utm_source` and `utm_campaign`, within one minute. Sending SHALL be at least once: a failed send SHALL be retried with growing delays for up to 24 hours, then given up and logged as an error. A failed or slow alert SHALL NOT fail or delay the submission. When no destination is configured, the lead SHALL be stored and no alert SHALL be sent, then or later. The alert destination SHALL be one environment setting; a malformed value SHALL stop the cloud from starting.

#### Scenario: The destination is down

- **WHEN** a lead is submitted while the alert destination answers `503`, and it recovers ten minutes later
- **THEN** the submission answers `201` immediately and the operator receives the alert after the destination recovers

#### Scenario: Not configured

- **WHEN** no alert destination is set and a lead is submitted
- **THEN** the lead is stored, no alert is sent, and setting a destination later sends nothing for that lead

### Requirement: The operator can list, read, update and delete leads

The cloud SHALL serve, under `/api/platform/leads`:
- `GET` a list, newest first, paginated, optionally filtered by `status`, each entry with every lead field including its attribution;
- `GET /:id` one lead, or `404`;
- `PATCH /:id` changing only `status` and `notes` when present; `status` SHALL be one of `new`, `contacted`, `won` or `lost`, any other value rejected with `400`; no other field of a lead SHALL be changeable;
- `DELETE /:id` removing the lead permanently, answering `204`, or `404` when it does not exist.

#### Scenario: Following a lead up

- **WHEN** the operator patches a lead with status `contacted` and notes `Demo el jueves`
- **THEN** reading the lead returns that status and those notes, with its name, phone and attribution unchanged

#### Scenario: A deletion request

- **WHEN** the operator deletes a lead whose owner asked for their data to be removed
- **THEN** the response is `204` and reading or listing no longer returns it

### Requirement: Only the platform operator can read leads

Every `/api/platform/leads` route SHALL require a valid admin API key in the `X-API-Key` header and SHALL respond `401` without one. A user token of any role and an organization API key SHALL NOT grant access. No endpoint or tool available to an organization's users SHALL return a lead. These routes SHALL be served only by the cloud.

#### Scenario: A restaurant admin tries to read leads

- **WHEN** a signed-in organization admin requests `GET /api/platform/leads` with their token and no admin API key
- **THEN** the response is `401` and no lead is returned
