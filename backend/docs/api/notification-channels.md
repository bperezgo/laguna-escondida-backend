# Notification Channels API

> **Plan:** in the cloud these routes need the `full` plan; otherwise `403 plan_required`. A suspended organization or an ended trial is refused first. See [Organization access refusals](./auth.md#organization-access-refusals).

Send a message to a staff chat (Discord) when something happens: an order is created or paid, a reservation is requested or confirmed. A **channel** is where messages go (a Discord webhook). A **rule** says which events go to which channel, optionally narrowed by conditions.

**Cloud only.** These routes exist only in the cloud binary; on the edge binary they return `404`. Orders replicated from the edge box produce no events.

**Needs a key.** The deployment must set `NOTIFICATION_SECRETS_KEY` (base64, 32 bytes). Without it every endpoint below answers `503 NOTIFICATIONS_DISABLED` and nothing is recorded or sent.

## Endpoints

| Method | Endpoint                                | Description                           | Permission                      |
| ------ | --------------------------------------- | ------------------------------------- | ------------------------------- |
| GET    | `/api/notification-channels`            | List channels                         | `notification-channels:read`    |
| GET    | `/api/notification-channels/:id`        | Get a channel                         | `notification-channels:read`    |
| POST   | `/api/notification-channels`            | Create a channel                      | `notification-channels:manage`  |
| PUT    | `/api/notification-channels/:id`        | Update name, enabled flag or webhook  | `notification-channels:manage`  |
| DELETE | `/api/notification-channels/:id`        | Delete a channel and its rules        | `notification-channels:manage`  |
| POST   | `/api/notification-channels/:id/test`   | Send a sample message right now       | `notification-channels:manage`  |
| GET    | `/api/notification-rules`               | List rules (`?channel_id=` filter)    | `notification-channels:read`    |
| GET    | `/api/notification-rules/:id`           | Get a rule                            | `notification-channels:read`    |
| POST   | `/api/notification-rules`               | Create a rule                         | `notification-channels:manage`  |
| PUT    | `/api/notification-rules/:id`           | Update a rule                         | `notification-channels:manage`  |
| DELETE | `/api/notification-rules/:id`           | Delete a rule                         | `notification-channels:manage`  |

`notification-channels:read` is granted to **admin** and **manager**; `notification-channels:manage` only to **admin**, because a webhook is a credential and a rule decides who gets pinged. A manager gets `403` on every change and on test sends. Every other role gets `403` on all of them. Everything is scoped to the caller's organization: another organization's channel or rule answers `404`.

## Error shape

Validation failures are `400` and name the offending field:

```json
{
  "error": "settings.webhook_url: must be an https Discord webhook URL (discord.com or discordapp.com, /api/webhooks/...)",
  "code": "NOTIFICATION_CHANNEL_INVALID_FIELD",
  "field": "settings.webhook_url"
}
```

| Status | `code`                              | When |
| ------ | ----------------------------------- | ---- |
| 400    | `NOTIFICATION_CHANNEL_INVALID_FIELD` | A channel field is invalid; `field` is its path (`name`, `type`, `settings.webhook_url`) |
| 400    | `NOTIFICATION_RULE_INVALID_FIELD`    | A rule field is invalid; `field` is its path (`event`, `channel_id`, `conditions.order_types`, `conditions.min_party_size`, `title_prefix`, `expires_after_minutes`) |
| 400    | (none)                               | Body is not valid JSON |
| 403    | (none)                               | Missing permission |
| 404    | (none)                               | Channel or rule not found, or belongs to another organization |
| 502    | `NOTIFICATION_CHANNEL_UNREACHABLE`   | Test send only: the destination refused or could not be reached |
| 503    | `NOTIFICATIONS_DISABLED`             | The deployment has no `NOTIFICATION_SECRETS_KEY` |

---

## Channels

A channel has a `name` (1-80 characters, unique within the organization), a `type`, an `enabled` flag (default `true`) and the settings of its type. The only type is `discord`, whose setting is `webhook_url`: an `https` URL on `discord.com` or `discordapp.com` under `/api/webhooks/`.

**Secrets are never returned.** The webhook is stored encrypted. Responses show `settings.webhook_url_hint` instead: `…` plus the last four characters of the URL. On `PUT`, omit `settings` to keep the stored webhook.

### Channel object

```json
{
  "id": "019a1000-0000-7000-8000-0000000000c1",
  "name": "Domicilios",
  "type": "discord",
  "enabled": true,
  "settings": { "webhook_url_hint": "…1234" },
  "created_at": "2026-11-18T14:00:00Z",
  "updated_at": "2026-11-18T14:00:00Z"
}
```

### List Channels

`GET /api/notification-channels`

```json
{ "notification_channels": [ { "id": "019a1000-0000-7000-8000-0000000000c1", "name": "Domicilios", "type": "discord", "enabled": true, "settings": { "webhook_url_hint": "…1234" }, "created_at": "2026-11-18T14:00:00Z", "updated_at": "2026-11-18T14:00:00Z" } ] }
```

### Get Channel

`GET /api/notification-channels/:id` returns the channel object (`200`), or `404`.

### Create Channel

`POST /api/notification-channels`

| Field                | Type    | Required | Description |
| -------------------- | ------- | -------- | ----------- |
| name                 | string  | Yes      | 1-80 characters, unique |
| type                 | string  | Yes      | `discord` |
| enabled              | boolean | No       | Defaults to `true` |
| settings.webhook_url | string  | Yes      | Discord webhook URL |

```json
{
  "name": "Domicilios",
  "type": "discord",
  "settings": { "webhook_url": "https://discord.com/api/webhooks/123/abcd1234" }
}
```

Response `201 Created`: the channel object. `400` with `field` `settings.webhook_url` for a URL outside Discord, `name` for an empty, too long or duplicated name, `type` for an unknown type.

### Update Channel

`PUT /api/notification-channels/:id`. Every field is optional.

```json
{ "name": "Cocina" }
```

Response `200 OK`: the channel object. Send `settings.webhook_url` to replace the webhook.

### Delete Channel

`DELETE /api/notification-channels/:id` deletes the channel and its rules.

```json
{ "message": "Notification channel deleted successfully" }
```

### Test Channel

`POST /api/notification-channels/:id/test`

Sends a sample message right away. Nothing is stored or retried. Response `204 No Content` when the destination accepts it. Otherwise `502`:

```json
{
  "error": "The destination refused the message or could not be reached",
  "code": "NOTIFICATION_CHANNEL_UNREACHABLE",
  "destination_status": 404
}
```

`destination_status` is omitted when the destination never answered.

---

## Rules

### Rule object

```json
{
  "id": "019a1000-0000-7000-8000-0000000000d1",
  "channel_id": "019a1000-0000-7000-8000-0000000000c1",
  "event": "order.paid",
  "conditions": { "order_types": ["delivery"] },
  "title_prefix": "<@&123456789>",
  "expires_after_minutes": 60,
  "enabled": true,
  "created_at": "2026-11-18T14:05:00Z",
  "updated_at": "2026-11-18T14:05:00Z"
}
```

`title_prefix` goes first in the message, unchanged, so a Discord role mention pings.

### List Rules

`GET /api/notification-rules?channel_id=<id>`

```json
{ "notification_rules": [ { "id": "019a1000-0000-7000-8000-0000000000d1", "channel_id": "019a1000-0000-7000-8000-0000000000c1", "event": "order.paid", "conditions": { "order_types": ["delivery"] }, "title_prefix": "", "expires_after_minutes": 60, "enabled": true, "created_at": "2026-11-18T14:05:00Z", "updated_at": "2026-11-18T14:05:00Z" } ] }
```

### Get Rule

`GET /api/notification-rules/:id` returns the rule object (`200`), or `404`.

### Create Rule

`POST /api/notification-rules`

| Field                 | Type    | Required | Description |
| --------------------- | ------- | -------- | ----------- |
| channel_id            | string  | Yes      | A channel of the same organization |
| event                 | string  | Yes      | One of the [events](#event-catalogue) |
| conditions            | object  | No       | Only the conditions of the event |
| title_prefix          | string  | No       | Up to 100 characters |
| expires_after_minutes | integer | No       | 1-10080. Defaults to 60 for order events, 720 for reservation events |
| enabled               | boolean | No       | Defaults to `true` |

```json
{
  "channel_id": "019a1000-0000-7000-8000-0000000000c1",
  "event": "order.paid",
  "conditions": { "order_types": ["delivery"] }
}
```

Response `201 Created`: the rule object. `400` with `field` `channel_id` for another organization's channel, `event` for an unknown event, `conditions.<name>` for a condition the event does not have or an unknown value.

### Update Rule

`PUT /api/notification-rules/:id`. Every field is optional; `event` cannot change.

```json
{ "enabled": false }
```

Response `200 OK`: the rule object.

### Delete Rule

`DELETE /api/notification-rules/:id`

```json
{ "message": "Notification rule deleted successfully" }
```

---

## Event catalogue

| Event                   | When                                                                 | Conditions                               |
| ----------------------- | -------------------------------------------------------------------- | ---------------------------------------- |
| `order.created`         | An order is created in the cloud                                     | `order_types`                            |
| `order.paid`            | An order is paid in the cloud                                        | `order_types`                            |
| `reservation.requested` | A reservation is created `requested`                                 | `sources`, `min_party_size`              |
| `reservation.confirmed` | A reservation becomes `confirmed`, at creation or by a transition    | `sources`, `confirmed_by`, `min_party_size` |

- `order_types`: `dine_in`, `takeaway`, `delivery`.
- `sources`: the reservation sources: `whatsapp`, `phone`, `walk_in`, `staff`.
- `confirmed_by`: `staff` or `auto` (a WhatsApp reservation stored `confirmed` at creation is `auto`).
- `min_party_size`: a positive integer; matches parties at least that large.

A rule matches when every condition it sets holds. A list matches when it contains the event's value; an empty or missing list matches any value.

## Delivery behavior

Messages are sent within seconds, at least once and in no particular order. A failed send is retried after 10 seconds, doubling up to 5 minutes (a Discord `429` retry delay is honored). A message not delivered within the rule's `expires_after_minutes` is dropped as `expired`. A `401`, `403` or `404` from Discord marks it `rejected` with no retry. A rule or channel deleted or disabled before sending drops its pending messages.

## Losing the key

Webhooks are encrypted with `NOTIFICATION_SECRETS_KEY`. If the key is lost or changed, stored webhooks cannot be decrypted: channels still list, but sending and testing fail. Re-enter each channel's webhook with `PUT /api/notification-channels/:id` after setting the new key. Unsetting the key turns notifications off (`503`) without deleting anything.
