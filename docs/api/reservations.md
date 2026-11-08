# Reservations API

Table reservations, from a guest's WhatsApp request or entered by staff. v1 records reservations and lets staff answer them; there are no tables, time slots or capacity checks.

**Cloud only.** These routes are not served on the box (edge); every one returns `404` there.

## Lifecycle

```
            staff confirm             starts_at + 2h (job)
 requested -------------> confirmed ------------------------> visited
     |  \                   |   \                              |
     |   \ staff decline    |    \ staff no-show               | staff no-show,
     |    v                 |     v                            | until feedback is requested
     |   declined           |    no_show <---------------------+
     v                      v
 cancelled (guest/staff)    cancelled (guest/staff)
```

- A `whatsapp` reservation starts as `requested`; one entered by staff (`phone`, `walk_in`, `staff`) starts as `confirmed`.
- A cloud job runs every 15 minutes and moves each `confirmed` reservation to `visited` two hours after its `starts_at`.
- Any other move answers `409 INVALID_RESERVATION_TRANSITION` and changes nothing.
- Each reservation belongs to a [contact](contacts.md), one per phone, created on the guest's first reservation or agent order.

## Statuses and sources

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| `requested` | Asked for through the WhatsApp agent; waiting for staff |
| `confirmed` | Staff confirmed it, or entered it themselves            |
| `visited`   | Two hours past its start while confirmed                |
| `declined`  | Staff turned the request down                           |
| `cancelled` | Cancelled by the guest or by staff (`cancelled_by`)     |
| `no_show`   | The guest did not come                                  |

| Source     | Who creates it                     | Starts as   |
| ---------- | ---------------------------------- | ----------- |
| `whatsapp` | The customer agent                 | `requested` |
| `phone`    | Staff, for a booking taken by phone | `confirmed` |
| `walk_in`  | Staff, for a guest at the door     | `confirmed` |
| `staff`    | Staff, any other way               | `confirmed` |

## Permissions

| Permission            | Granted to               | Allows                                                    |
| --------------------- | ------------------------ | --------------------------------------------------------- |
| `reservations:read`   | admin, manager, agent    | List and read reservations and contacts                   |
| `reservations:create` | admin, agent             | Create a `whatsapp` request; cancel it for its own guest   |
| `reservations:manage` | admin, manager           | Create a staff reservation; confirm, decline, cancel, no-show |

## Endpoints

| Method | Endpoint                         | Permission                     | Description                          |
| ------ | -------------------------------- | ------------------------------ | ------------------------------------ |
| GET    | `/api/reservations`              | `reservations:read`            | List a day's (or a guest's) reservations |
| GET    | `/api/reservations/:id`          | `reservations:read`            | Get a reservation                    |
| POST   | `/api/reservations`              | `reservations:create` or `:manage` | Create a reservation             |
| POST   | `/api/reservations/:id/confirm`  | `reservations:manage`          | Confirm a request                    |
| POST   | `/api/reservations/:id/decline`  | `reservations:manage`          | Decline a request                    |
| POST   | `/api/reservations/:id/cancel`   | `reservations:create` or `:manage` | Cancel, as staff or for the guest |
| POST   | `/api/reservations/:id/no-show`  | `reservations:manage`          | Mark a no-show                       |

---

## The reservation object

| Field                | Type    | Description                                                              |
| -------------------- | ------- | ------------------------------------------------------------------------ |
| id                   | string  | UUID v7                                                                  |
| contact_id           | string  | The guest's contact                                                      |
| contact_phone        | string  | The guest's phone, E.164                                                 |
| name                 | string  | The name the table is booked under                                       |
| party_size           | integer | 1–50                                                                     |
| starts_at            | string  | RFC 3339                                                                 |
| notes                | string  | Optional                                                                 |
| source               | string  | `whatsapp`, `phone`, `walk_in` or `staff`                                |
| status               | string  | See [statuses](#statuses-and-sources)                                    |
| decline_reason       | string  | Set when declined with a reason                                          |
| cancelled_by         | string  | `customer` or `staff`, set when cancelled                                |
| visited_at           | string  | When it became `visited`                                                 |
| feedback_requested   | boolean | Whether the guest was asked for feedback (after which no-show is refused) |
| origin               | object  | Where it came from; `{}` for staff reservations. See [Origin](#origin)  |
| created_at, updated_at | string | RFC 3339                                                                |

### Origin

Set from the WhatsApp agent runtime's headers, never by the model, and never changed afterwards.

| Field                       | Type   | Description                                         |
| --------------------------- | ------ | --------------------------------------------------- |
| channel_id                  | string | marketing-app channel the guest wrote on (≤128)     |
| conversation_id             | string | marketing-app conversation (≤128)                   |
| attribution.source          | string | e.g. `meta_ctwa`; `^[a-z0-9_]{1,32}$`               |
| attribution.ad_id           | string | Meta ad id (≤64)                                    |
| attribution.clicked_at      | string | RFC 3339, not more than 5 minutes in the future     |
| attribution.campaign_id     | string | ≤64                                                 |
| attribution.campaign_name   | string | ≤255                                                |

An invalid attribution field is dropped (and logged) and the reservation is still created. An over-long `channel_id` or `conversation_id` is a `400`. A click identifier (`ctwa_clid` or similar) is ignored if sent and never stored.

---

## List Reservations

```
GET /api/reservations
```

### Query Parameters

| Param  | Type   | Required | Description                                                                 |
| ------ | ------ | -------- | --------------------------------------------------------------------------- |
| date   | string | No       | `YYYY-MM-DD` in the organization's timezone. Default: today, unless `phone` is given |
| status | string | No       | One status, or several comma-separated: `requested,confirmed`              |
| phone  | string | No       | Only this guest's reservations (national or `+international`)              |
| from   | string | No       | With `phone`: only those starting at or after this RFC 3339 time           |

Ordered by `starts_at`, oldest first, in every status unless filtered. Times in responses are UTC.

### Example Response (200 OK)

```json
{
  "reservations": [
    {
      "id": "019a5e1c-7b2e-7c41-9d0a-2f6c1e8b4a10",
      "contact_id": "019a5e1c-7b2d-7f90-a3b1-6e2d4c9f8a01",
      "contact_phone": "+573005559876",
      "name": "Luis",
      "party_size": 4,
      "starts_at": "2026-11-10T00:00:00Z",
      "source": "phone",
      "status": "confirmed",
      "feedback_requested": false,
      "origin": {},
      "created_at": "2026-11-08T20:02:11Z",
      "updated_at": "2026-11-08T20:02:11Z"
    },
    {
      "id": "019a5e1d-0c44-7a8e-b7d2-91f0a6c3e5b2",
      "contact_id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
      "contact_phone": "+573005551234",
      "name": "Ana",
      "party_size": 6,
      "starts_at": "2026-11-10T01:00:00Z",
      "source": "whatsapp",
      "status": "requested",
      "feedback_requested": false,
      "origin": {
        "channel_id": "ch_1",
        "conversation_id": "conv_9",
        "attribution": { "source": "meta_ctwa", "ad_id": "120210000000001", "clicked_at": "2026-11-08T14:00:00Z" }
      },
      "created_at": "2026-11-08T20:10:40Z",
      "updated_at": "2026-11-08T20:10:40Z"
    }
  ],
  "total": 2
}
```

### Errors

| Status | Code               | When                                          |
| ------ | ------------------ | --------------------------------------------- |
| 400    | `VALIDATION_ERROR` | `date`, `status`, `phone` or `from` is invalid; `field` names it |
| 403    | —                  | Missing `reservations:read`                   |

---

## Get Reservation

```
GET /api/reservations/:id
```

Returns one [reservation](#the-reservation-object).

### Errors

| Status | Code                    | When                                         |
| ------ | ----------------------- | -------------------------------------------- |
| 404    | `RESERVATION_NOT_FOUND` | No such reservation in this organization     |

---

## Create Reservation

```
POST /api/reservations
```

A `whatsapp` reservation needs `reservations:create` and is stored as `requested`. Any other source needs `reservations:manage` and is stored as `confirmed`. The guest's contact is created or updated in the same transaction.

### Request Body

| Field          | Type    | Required | Description                                                 |
| -------------- | ------- | -------- | ----------------------------------------------------------- |
| customer_phone | string  | Yes      | National (read in the home region) or `+international`      |
| name           | string  | Yes      | 1–100 characters after trimming                             |
| party_size     | integer | Yes      | 1–50                                                        |
| starts_at      | string  | Yes      | RFC 3339 with offset; in the future, at most 60 days ahead  |
| notes          | string  | No       | At most 500 characters                                      |
| source         | string  | Yes      | `whatsapp`, `phone`, `walk_in` or `staff`                   |
| origin         | object  | No       | See [Origin](#origin)                                       |

### Example Request (staff)

```json
{
  "customer_phone": "300 555 9876",
  "name": "Luis",
  "party_size": 4,
  "starts_at": "2026-11-09T19:00:00-05:00",
  "notes": "Cumpleaños",
  "source": "phone"
}
```

### Example Response (201 Created)

```json
{
  "id": "019a5e1c-7b2e-7c41-9d0a-2f6c1e8b4a10",
  "contact_id": "019a5e1c-7b2d-7f90-a3b1-6e2d4c9f8a01",
  "contact_phone": "+573005559876",
  "name": "Luis",
  "party_size": 4,
  "starts_at": "2026-11-10T00:00:00Z",
  "notes": "Cumpleaños",
  "source": "phone",
  "status": "confirmed",
  "feedback_requested": false,
  "origin": {},
  "created_at": "2026-11-08T20:02:11Z",
  "updated_at": "2026-11-08T20:02:11Z"
}
```

### Errors

| Status | Code               | When                                                         |
| ------ | ------------------ | ------------------------------------------------------------ |
| 400    | `VALIDATION_ERROR` | A field is invalid; `field` names it (e.g. `party_size`, `starts_at`, `origin.channel_id`) |
| 403    | —                  | `whatsapp` without `reservations:create`, or another source without `reservations:manage` |

```json
{ "error": "invalid party_size: must be between 1 and 50", "code": "VALIDATION_ERROR", "field": "party_size" }
```

---

## Confirm, Decline, Cancel, No-show

```
POST /api/reservations/:id/confirm
POST /api/reservations/:id/decline
POST /api/reservations/:id/cancel
POST /api/reservations/:id/no-show
```

Each returns the updated [reservation](#the-reservation-object).

| Action    | From                                   | Body                                   |
| --------- | -------------------------------------- | -------------------------------------- |
| confirm   | `requested`                            | none                                   |
| decline   | `requested`                            | `{"reason": "Fully booked"}` (optional, ≤255) |
| cancel    | `requested`, `confirmed`               | none as staff; `{"customer_phone": "..."}` for the guest |
| no-show   | `confirmed`; `visited` until feedback is requested | none                       |

**Cancel** with `reservations:manage` cancels as staff (`cancelled_by: "staff"`). Without it, the body's `customer_phone` must own the reservation, which must be `requested` or `confirmed` and not started; anything else answers `404 RESERVATION_NOT_FOUND`, the same as an unknown id, and changes nothing.

A WhatsApp reservation's confirm, decline and cancel each send an event to marketing-app, which messages the guest (see [marketing events](marketing-events.md)).

### Example Request (decline)

```json
{ "reason": "Fully booked" }
```

### Example Response (200 OK)

```json
{
  "id": "019a5e1d-0c44-7a8e-b7d2-91f0a6c3e5b2",
  "contact_id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
  "contact_phone": "+573005551234",
  "name": "Ana",
  "party_size": 6,
  "starts_at": "2026-11-10T01:00:00Z",
  "source": "whatsapp",
  "status": "declined",
  "decline_reason": "Fully booked",
  "feedback_requested": false,
  "origin": { "channel_id": "ch_1", "conversation_id": "conv_9" },
  "created_at": "2026-11-08T20:10:40Z",
  "updated_at": "2026-11-08T20:31:02Z"
}
```

### Errors

| Status | Code                             | When                                                      |
| ------ | -------------------------------- | --------------------------------------------------------- |
| 400    | `VALIDATION_ERROR`               | `reason` too long; guest cancel without `customer_phone`  |
| 403    | —                                | Missing the permission                                    |
| 404    | `RESERVATION_NOT_FOUND`          | Unknown id, or not the guest's to cancel                  |
| 409    | `INVALID_RESERVATION_TRANSITION` | Not allowed from the current status                       |

```json
{
  "error": "a declined reservation cannot become confirmed",
  "code": "INVALID_RESERVATION_TRANSITION",
  "from": "declined",
  "to": "confirmed"
}
```
