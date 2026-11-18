# Feedback API

Guests are asked how their visit went the morning after a reservation, and their answer is recorded here. Ratings of 1–3 are flagged for a manager. When the restaurant turns it on, every guest's answer, whatever the rating, returns the Google review link.

**Cloud only.** Not served on the box (edge).

## How a request is made

A cloud job runs hourly. For each `visited` [reservation](reservations.md) that came through a messaging channel and has no request yet, it opens one feedback request at or after 11:00, in the organization's timezone, on the day after the reservation's `starts_at`, and sends `feedback.requested` to marketing-app, which messages the guest. Visits that became `visited` more than 3 days earlier are skipped, so turning the feature on never floods old guests. Reservations entered by staff have no channel and are never asked.

A request stays open for 7 days. Recording again while it is open replaces the rating and comment.

The review link comes only from the organization's [feedback settings](organization-settings.md#feedback): it is returned when `review_link_enabled` is `true` and `review_url` is set, for every rating, and never otherwise. The rating is never an input to that decision.

## Endpoints

| Method | Endpoint        | Permission        | Description                              |
| ------ | --------------- | ----------------- | ---------------------------------------- |
| GET    | `/api/feedback` | `feedback:read`   | List feedback (admin, manager)           |
| POST   | `/api/feedback` | `feedback:create` | Record a guest's answer (admin, agent)   |

---

## The feedback object

| Field               | Type    | Description                                          |
| ------------------- | ------- | ---------------------------------------------------- |
| id                  | string  | UUID v7                                              |
| feedback_request_id | string  | The request it answers                               |
| contact_id          | string  | The guest's contact                                  |
| contact_phone       | string  | E.164                                                |
| rating              | integer | 1–5                                                  |
| comment             | string  | Optional, ≤1000                                      |
| needs_attention     | boolean | `true` for ratings 1–3                               |
| reservation         | object  | `id`, `name`, `party_size`, `starts_at` of the visit |
| created_at          | string  | First answer (UTC)                                   |
| updated_at          | string  | Latest answer (UTC)                                  |

---

## List Feedback

```
GET /api/feedback
```

### Query Parameters

| Param           | Type    | Required | Description                                                                  |
| --------------- | ------- | -------- | ---------------------------------------------------------------------------- |
| from            | string  | No       | First day, `YYYY-MM-DD` in the organization's timezone. Default: this week's Monday |
| to              | string  | No       | Last day, inclusive. Default: six days after `from`                          |
| needs_attention | boolean | No       | `true` lists ratings of 1–3 only                                             |

Filtered on when the feedback was last recorded, newest first.

### Example Response (200 OK)

```json
{
  "feedback": [
    {
      "id": "019a5f02-4b11-7d2e-8a3c-1e9f0b7c6d21",
      "feedback_request_id": "019a5f01-ee30-7a15-9c42-6b8d2e1f0a33",
      "contact_id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
      "contact_phone": "+573005551234",
      "rating": 2,
      "comment": "La música muy alta",
      "needs_attention": true,
      "reservation": { "id": "019a5e1d-0c44-7a8e-b7d2-91f0a6c3e5b2", "name": "Ana", "party_size": 6, "starts_at": "2026-11-10T01:00:00Z" },
      "created_at": "2026-11-10T16:20:00Z",
      "updated_at": "2026-11-10T17:02:00Z"
    }
  ],
  "total": 1
}
```

### Errors

| Status | Code               | When                                                       |
| ------ | ------------------ | ---------------------------------------------------------- |
| 400    | `VALIDATION_ERROR` | `from` or `to` is not a date, or `to` is before `from`     |
| 400    | —                  | `needs_attention` is not `true` or `false`                 |
| 403    | —                  | Missing `feedback:read`                                    |

---

## Record Feedback

```
POST /api/feedback
```

Attaches the answer to the phone's most recent open request.

### Request Body

| Field          | Type    | Required | Description                               |
| -------------- | ------- | -------- | ----------------------------------------- |
| customer_phone | string  | Yes      | The guest's phone                         |
| rating         | integer | Yes      | 1–5                                       |
| comment        | string  | No       | At most 1000 characters                   |

### Example Request

```json
{ "customer_phone": "+573005551234", "rating": 2, "comment": "La música muy alta" }
```

### Example Response (200 OK)

`review_url` is present only while the organization shares it.

```json
{
  "feedback": {
    "id": "019a5f02-4b11-7d2e-8a3c-1e9f0b7c6d21",
    "feedback_request_id": "019a5f01-ee30-7a15-9c42-6b8d2e1f0a33",
    "contact_id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
    "contact_phone": "+573005551234",
    "rating": 2,
    "comment": "La música muy alta",
    "needs_attention": true,
    "reservation": { "id": "019a5e1d-0c44-7a8e-b7d2-91f0a6c3e5b2", "name": "Ana", "party_size": 6, "starts_at": "2026-11-10T01:00:00Z" },
    "created_at": "2026-11-10T16:20:00Z",
    "updated_at": "2026-11-10T17:02:00Z"
  },
  "escalate": true,
  "review_url": "https://g.page/r/abc/review"
}
```

### Errors

| Status | Code                         | When                                                   |
| ------ | ---------------------------- | ------------------------------------------------------ |
| 400    | `VALIDATION_ERROR`           | `rating`, `comment` or `customer_phone` is invalid     |
| 403    | —                            | Missing `feedback:create`                              |
| 404    | `NO_VISIT_AWAITING_FEEDBACK` | The phone has no open request; nothing is stored       |
