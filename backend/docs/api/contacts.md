# Contacts API

A contact is one guest phone within an organization. It is created by the guest's first [reservation](reservations.md) or customer-agent order, and kept up to date by the later ones: `last_seen_at` moves forward and a non-empty name replaces the old one. The first ad attribution ever seen becomes the contact's `first_touch` and never changes.

**Cloud only.** Not served on the box (edge). Both routes need `reservations:read`. Contacts are never shared between organizations: the same phone in two restaurants is two contacts.

## Endpoints

| Method | Endpoint            | Description                                           |
| ------ | ------------------- | ----------------------------------------------------- |
| GET    | `/api/contacts`     | Search contacts by phone or name                      |
| GET    | `/api/contacts/:id` | A contact with its reservations and feedback          |

---

## The contact object

| Field         | Type   | Description                                                     |
| ------------- | ------ | --------------------------------------------------------------- |
| id            | string | UUID v7                                                         |
| phone         | string | E.164                                                           |
| name          | string | The latest non-empty name given; omitted when none was          |
| first_touch   | object | The ad that first brought the guest (`source`, `ad_id`, `clicked_at`, `campaign_id`, `campaign_name`); omitted when organic |
| first_seen_at | string | RFC 3339 (UTC)                                                  |
| last_seen_at  | string | RFC 3339 (UTC)                                                  |
| created_at, updated_at | string | RFC 3339 (UTC)                                         |

---

## List Contacts

```
GET /api/contacts
```

### Query Parameters

| Param  | Type    | Required | Description                                              |
| ------ | ------- | -------- | -------------------------------------------------------- |
| search | string  | No       | Part of the phone, or of the name (case-insensitive)     |
| limit  | integer | No       | Page size, default 50, at most 200                       |
| offset | integer | No       | Default 0                                                |

Most recently seen first.

### Example Response (200 OK)

```json
{
  "contacts": [
    {
      "id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
      "phone": "+573005551234",
      "name": "Ana María",
      "first_touch": { "source": "meta_ctwa", "ad_id": "120210000000001", "clicked_at": "2026-10-02T14:00:00Z" },
      "first_seen_at": "2026-10-02T15:10:40Z",
      "last_seen_at": "2026-11-08T20:10:40Z",
      "created_at": "2026-10-02T15:10:40Z",
      "updated_at": "2026-11-08T20:10:40Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Errors

| Status | When                                   |
| ------ | -------------------------------------- |
| 400    | `limit` or `offset` is not a number    |
| 403    | Missing `reservations:read`            |

---

## Get Contact

```
GET /api/contacts/:id
```

The contact, its reservations newest first, and its feedback newest first.

### Example Response (200 OK)

```json
{
  "id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
  "phone": "+573005551234",
  "name": "Ana María",
  "first_touch": { "source": "meta_ctwa", "ad_id": "120210000000001" },
  "first_seen_at": "2026-10-02T15:10:40Z",
  "last_seen_at": "2026-11-08T20:10:40Z",
  "created_at": "2026-10-02T15:10:40Z",
  "updated_at": "2026-11-08T20:10:40Z",
  "reservations": [
    {
      "id": "019a5e1d-0c44-7a8e-b7d2-91f0a6c3e5b2",
      "contact_id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
      "contact_phone": "+573005551234",
      "name": "Ana María",
      "party_size": 6,
      "starts_at": "2026-11-10T01:00:00Z",
      "source": "whatsapp",
      "status": "requested",
      "feedback_requested": false,
      "origin": { "channel_id": "ch_1", "conversation_id": "conv_9" },
      "created_at": "2026-11-08T20:10:40Z",
      "updated_at": "2026-11-08T20:10:40Z"
    }
  ],
  "feedback": [
    {
      "id": "019a3b10-2a11-7c3e-9f42-0d5e8a7b6c01",
      "feedback_request_id": "019a3b0f-9e20-7b44-8c11-3f2a6d9e5b10",
      "contact_id": "019a5e1d-0c43-7d15-8e6a-c2b7f4a19d03",
      "contact_phone": "+573005551234",
      "rating": 5,
      "comment": "Todo delicioso",
      "needs_attention": false,
      "reservation": { "id": "019a3a77-1c20-7e55-b1a9-2c4d6e8f0a12", "name": "Ana", "party_size": 2, "starts_at": "2026-10-03T01:00:00Z" },
      "created_at": "2026-10-03T16:05:00Z",
      "updated_at": "2026-10-03T16:05:00Z"
    }
  ]
}
```

### Errors

| Status | Code                | When                                      |
| ------ | ------------------- | ----------------------------------------- |
| 404    | `CONTACT_NOT_FOUND` | No such contact in this organization      |
