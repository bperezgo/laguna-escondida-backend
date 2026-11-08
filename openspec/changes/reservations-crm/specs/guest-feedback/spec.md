## Purpose

Asks guests how their visit went the day after a reservation, records their rating and comment, flags unhappy guests for the manager, and, only when the restaurant turns it on, shares the Google review link with every guest regardless of their rating.

## ADDED Requirements

### Requirement: Feedback is requested the morning after a visit

For each `visited` reservation that came through a messaging channel, the system SHALL create exactly one feedback request at or after 11:00, in the organization's timezone, on the day after the reservation's `starts_at`, and SHALL emit a `feedback.requested` event for it. It SHALL NOT request feedback for a reservation that is not `visited`, that has no channel, or that became `visited` more than 3 days earlier.

#### Scenario: Dinner last night

- **WHEN** a WhatsApp reservation for yesterday at 20:00 is `visited` and it is 11:05 today in the organization's timezone
- **THEN** one feedback request exists for it and one `feedback.requested` event is emitted

#### Scenario: A no-show

- **WHEN** a reservation for yesterday was marked `no_show`
- **THEN** no feedback is requested

#### Scenario: Booked by phone

- **WHEN** a staff-entered `phone` reservation is `visited`
- **THEN** no feedback is requested, because there is no channel to send it on

#### Scenario: An old backlog

- **WHEN** the feature is first deployed and a reservation became `visited` 5 days ago
- **THEN** no feedback is requested for it

### Requirement: The guest's answer is recorded against their latest open request

A feedback request SHALL stay open for 7 days. Recording feedback for a phone SHALL attach a `rating` (integer 1–5) and an optional `comment` (at most 1000 characters) to that phone's most recent open request. Recording again while the request is open SHALL replace its rating and comment. With no open request, recording SHALL fail with a "no visit awaiting feedback" error and store nothing.

#### Scenario: A five-star answer

- **WHEN** the guest with an open request answers 5 with "Todo delicioso"
- **THEN** feedback with rating 5 and that comment is stored on the request

#### Scenario: The guest adds a comment later

- **WHEN** the same guest records rating 5 and then, an hour later, rating 4 with a comment
- **THEN** the request holds rating 4 and the comment

#### Scenario: Nothing to answer

- **WHEN** feedback is recorded for a phone whose only request closed 8 days ago
- **THEN** the call fails with "no visit awaiting feedback" and nothing is stored

### Requirement: Low ratings are escalated

Recording a rating of 1, 2 or 3 SHALL mark the feedback as needing attention, and the response SHALL say to escalate. Ratings of 4 and 5 SHALL NOT be escalated.

#### Scenario: A bad night

- **WHEN** a guest records rating 2
- **THEN** the response says to escalate and the feedback is listed as needing attention

### Requirement: The review link is shared with every guest or with none

An organization's settings SHALL have a `feedback` section with `review_link_enabled` (default `false`) and `review_url` (an absolute `https` URL or empty). When `review_link_enabled` is `true` and `review_url` is set, every recorded feedback response SHALL include `review_url`, whatever the rating. Otherwise no response SHALL include it. The rating SHALL NOT affect whether the link is included.

#### Scenario: Link switched off

- **WHEN** `review_link_enabled` is `false` and a guest records rating 5
- **THEN** the response has no review link

#### Scenario: Link on, unhappy guest

- **WHEN** `review_link_enabled` is `true` with a `review_url`, and a guest records rating 2
- **THEN** the response includes the review link and says to escalate

#### Scenario: Switched on without a URL

- **WHEN** `review_link_enabled` is `true` and `review_url` is empty
- **THEN** no response includes a review link

### Requirement: Staff manage the feedback settings and read the feedback

`GET /api/organization/settings` SHALL include the `feedback` section. Changing it through `PATCH /api/organization/settings` SHALL require `settings:feedback:update`, and an invalid `review_url` SHALL be rejected with a validation error. The cloud SHALL serve `GET /api/feedback`, filtered by date range and optionally to those needing attention, newest first, each with its rating, comment, contact phone and reservation. It SHALL require `feedback:read`.

#### Scenario: Turning the link on

- **WHEN** a manager patches `feedback` with `review_link_enabled: true` and `review_url: "https://g.page/r/abc/review"`
- **THEN** the settings are saved and later feedback responses include the link

#### Scenario: A plain-http URL

- **WHEN** a manager patches `review_url` with `http://example.com`
- **THEN** the request fails with a validation error and the settings are unchanged

#### Scenario: Reading this week's unhappy guests

- **WHEN** a manager lists feedback for this week, needing attention only
- **THEN** only this week's feedback rated 1–3 is returned
