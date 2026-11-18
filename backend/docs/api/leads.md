# Leads API

A lead is a restaurant owner's demo request from the public marketing site (`landing/`). It is **platform data**: it belongs to no organization, like `organizations`, and no tenant user or tool can read it.

- **Cloud only.** The edge does not serve these routes (`404`).
- **Two route groups.** `POST /api/public/leads` takes no credential. `/api/platform/leads` requires `X-API-Key: <ADMIN_API_KEY>`; a user token of any role or an organization API key is refused with `401`.
- **No IP stored.** The sender's address is used only to rate-limit and is never written with the lead.

## Lead Fields

| Field             | Type          | Notes                                                                  |
| ----------------- | ------------- | ---------------------------------------------------------------------- |
| `id`              | string (UUID) | Generated (v7). Never returned by the public endpoint                  |
| `name`            | string        | 1–100 characters after trimming                                        |
| `restaurant_name` | string        | 1–120 characters after trimming                                        |
| `phone`           | string        | E.164, e.g. `+573001234567`                                            |
| `status`          | string        | `new` (default), `contacted`, `won`, `lost`                            |
| `notes`           | string        | Operator notes, at most 2000 characters; empty by default              |
| `consent_at`      | string        | RFC 3339. When the prospect accepted the privacy policy                |
| `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content` | string | At most 255 characters each; `""` when absent |
| `fbclid`, `gclid` | string        | At most 512 characters; `""` when absent                               |
| `landing_url`, `referrer` | string | Absolute `http`/`https` URL, at most 2048 characters; `""` when absent |
| `event_id`        | string        | `^[A-Za-z0-9-]{8,64}$`. The id the browser sent to Google Analytics and Meta, kept so a later server-side conversion call can deduplicate against it |
| `created_at`, `updated_at` | string | RFC 3339                                                              |

## Endpoints

| Method | Endpoint                  | Auth          | Description                          |
| ------ | ------------------------- | ------------- | ------------------------------------ |
| POST   | `/api/public/leads`       | none          | Submit a lead from the marketing site |
| GET    | `/api/platform/leads`     | admin API key | List leads, newest first              |
| GET    | `/api/platform/leads/:id` | admin API key | Get a lead                            |
| PATCH  | `/api/platform/leads/:id` | admin API key | Change `status` and `notes`           |
| DELETE | `/api/platform/leads/:id` | admin API key | Delete a lead permanently             |

## Submit a Lead

`POST /api/public/leads` — callable from a browser on another origin (CORS is open).

### Request

```json
{
  "name": "Ana Ruiz",
  "restaurant_name": "El Fogón",
  "phone": "300 123 4567",
  "consent": true,
  "website": "",
  "utm_source": "facebook",
  "utm_campaign": "lanzamiento",
  "fbclid": "IwAR0abc",
  "landing_url": "https://kuenta.com/?utm_source=facebook&utm_campaign=lanzamiento",
  "referrer": "https://l.facebook.com/",
  "event_id": "0b9c1c3e-6a1f-4c1e-9d55-3f2a8b7c1d10"
}
```

- `name`, `restaurant_name`, `phone` and `consent` are required. Everything else is optional.
- `phone` without a country code is read as Colombian; a number starting with `+` is international (`+52 55 1234 5678` is stored as `+525512345678`).
- `consent` must be `true`.
- `website` is a honeypot: a person never fills it. A non-empty value is answered like a success and nothing is stored.
- An invalid attribution field never rejects the lead: it is dropped and the rest is stored.

### Response — `201 Created`

```json
{ "status": "received" }
```

The body never carries the lead's id or any stored field. The same response is returned for a honeypot hit and for a phone already submitted in the last 24 hours (nothing new is stored or alerted).

### Errors

| Status | When                                                                                  | Body                                                                            |
| ------ | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `400`  | Invalid JSON, or a required field is empty, too long or invalid; `field` names it      | `{"error": "invalid phone: is not a valid phone number", "field": "phone"}`     |
| `413`  | Body larger than 16 KB                                                                | `{"error": "Request body too large"}`                                           |
| `429`  | More than 5 submissions from one address in an hour (`Retry-After` is the seconds left in the window), or more than 300 leads stored in the last hour across all clients (`Retry-After: 300`) | `{"error": "Too many requests"}` |

Fields that can be named in `field`: `name`, `restaurant_name`, `phone`, `consent`.

## List Leads

`GET /api/platform/leads?status=new&page=1&page_size=20`

| Query       | Default | Notes                                                |
| ----------- | ------- | ---------------------------------------------------- |
| `status`    | all     | One of `new`, `contacted`, `won`, `lost`; otherwise `400` with `field` `status` |
| `page`      | `1`     |                                                      |
| `page_size` | `20`    | Capped at `100`                                      |

```json
{
  "leads": [
    {
      "id": "019a1f4e-3c5b-7d2a-8e11-5f0c9a7b2d44",
      "name": "Ana Ruiz",
      "restaurant_name": "El Fogón",
      "phone": "+573001234567",
      "status": "new",
      "notes": "",
      "consent_at": "2026-11-18T15:00:00Z",
      "utm_source": "facebook",
      "utm_medium": "",
      "utm_campaign": "lanzamiento",
      "utm_term": "",
      "utm_content": "",
      "fbclid": "IwAR0abc",
      "gclid": "",
      "landing_url": "https://kuenta.com/?utm_source=facebook&utm_campaign=lanzamiento",
      "referrer": "https://l.facebook.com/",
      "event_id": "0b9c1c3e-6a1f-4c1e-9d55-3f2a8b7c1d10",
      "created_at": "2026-11-18T15:00:00Z",
      "updated_at": "2026-11-18T15:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

## Get a Lead

`GET /api/platform/leads/:id` returns one lead (the object above), or `404` `{"error": "Lead not found"}`.

## Update a Lead

`PATCH /api/platform/leads/:id` changes **only** `status` and `notes`; a field left out is unchanged. Any other field in the body is ignored: name, phone and attribution cannot be changed.

```json
{ "status": "contacted", "notes": "Demo el jueves" }
```

Returns `200` with the updated lead. `400` with `field` `status` for an unknown status, or `notes` over 2000 characters; `404` for an unknown id.

## Delete a Lead

`DELETE /api/platform/leads/:id` removes the lead permanently (a prospect asked for their data to be erased). Returns `204`, or `404` when it does not exist.

## Common Errors (platform routes)

**401 Unauthorized** - Missing or wrong `X-API-Key` (a JWT is not accepted)

```json
{ "error": "API key is required" }
```

## Operator alert

When `LEAD_ALERT_DISCORD_WEBHOOK_URL` is set, each stored lead sends one Discord message (name, restaurant, phone, `utm_source`, `utm_campaign`) within about 15 seconds. A failed send is retried with delays from 15 seconds doubling to 30 minutes, for up to 24 hours, then given up and logged at error level. A failed alert never fails or delays the submission. A lead stored while no webhook is set is never alerted, even if one is set later.

## Environment settings (cloud)

| Variable                         | Default | Meaning                                                                                       |
| -------------------------------- | ------- | --------------------------------------------------------------------------------------------- |
| `LEAD_ALERT_DISCORD_WEBHOOK_URL` | unset   | Discord webhook that receives lead alerts. Unset turns alerts off; a malformed value stops startup |
| `TRUSTED_PROXIES`                | unset   | Comma-separated IPs/CIDRs allowed to set `X-Forwarded-For`. Unset trusts every peer, so a forged header dodges the per-address limit. **Set it in production to the load balancer's range.** A malformed entry stops startup |
