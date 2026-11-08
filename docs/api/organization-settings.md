# Organization Settings API

The calling organization's settings: its **ordering** settings (when it takes **delivery** and **takeaway** orders) and its **feedback** settings (whether guests get the review link, see [Feedback](#feedback)). Each type has an emergency **pause** switch and a weekly **schedule**. Creating a takeaway or delivery order while that type is unavailable is refused with `422` (see [Orders → Ordering availability](./orders.md#ordering-availability)). `dine_in` is never affected.

**Cloud only.** These routes exist only in the cloud binary; on the edge binary they return `404`. The box reads its own organization as unconfigured, so takeaway there is always open.

**Tenant-scoped.** Every request acts on the organization of the caller's token or organization API key. There is no organization id in the path. The platform operator sees the stored settings read-only on [`GET /api/platform/organizations/:id`](./organizations.md).

## Endpoints

| Method | Endpoint                     | Description                                         | Permission |
| ------ | ---------------------------- | --------------------------------------------------- | ---------- |
| GET    | `/api/organization/settings` | Stored settings plus availability right now         | `ordering:read` |
| PATCH  | `/api/organization/settings` | Change the pause and/or the schedule of either type | `ordering:read`, plus per field (below) |

| Permission        | Granted to                          | Allows |
| ----------------- | ----------------------------------- | ------ |
| `ordering:read`   | admin, manager, accountant, agent   | Reading the settings and availability |
| `ordering:pause`  | admin, manager                      | Changing `paused` and `paused_reason` |
| `ordering:manage` | admin, manager                      | Changing `schedule` |
| `settings:feedback:update` | admin, manager             | Changing the `feedback` section |

The pause and the hours are separate permissions so a floor role can get the switch without the hours later. Today only admins and managers hold either.

## Model

Each of `delivery` and `takeaway` has:

| Field           | Type           | Description |
| --------------- | -------------- | ----------- |
| `paused`        | boolean        | Orders of this type are stopped. Default `false`. A pause lasts until someone lifts it. |
| `paused_reason` | string \| null | Shown to staff and the agent, at most 255 characters. Always `null` when not paused. |
| `schedule`      | array          | Opening windows. Default `[]`, which means **always open** (when not paused). |

An opening window:

| Field    | Type   | Description |
| -------- | ------ | ----------- |
| `day`    | string | `mon`, `tue`, `wed`, `thu`, `fri`, `sat` or `sun` |
| `opens`  | string | 24-hour `HH:MM`, `00:00`–`23:59` |
| `closes` | string | 24-hour `HH:MM`. Earlier than `opens` means the window runs **past midnight** into the next day. |

Times are read in the organization's `timezone` (see [Organizations](./organizations.md)). A window includes its `opens` minute and excludes its `closes` minute: with `18:00`–`22:30`, an order at 22:30 is refused.

### Schedule rules

A schedule is validated as a whole. An invalid one is rejected with `400` and nothing is stored.

- `day` is one of the seven codes, and `opens` / `closes` are valid `HH:MM`.
- `opens` and `closes` differ.
- Two windows of the same type don't overlap, including a past-midnight window running into the next day's windows (Friday `18:00`–`02:00` overlaps Saturday `01:00`–`04:00`). Back-to-back windows (`11:00`–`15:00`, `15:00`–`18:00`) are fine.
- At most 21 windows per type.

### Availability

Computed at the moment of the request, never stored. Each type reports:

| Field           | Type           | Description |
| --------------- | -------------- | ----------- |
| `available`     | boolean        | `true` when not paused and either the schedule is empty or now is inside a window |
| `reason`        | string \| null | `null` when available, `paused`, or `outside_hours`. `paused` wins over `outside_hours`. |
| `paused_reason` | string \| null | The stored reason when paused |
| `closes_at`     | string \| null | When available through a window, the instant that window closes. `null` when the schedule is empty. |
| `next_opens_at` | string \| null | When `outside_hours`, the next window's opening within seven days. `null` when paused: a pause has no planned end. |

Instants are RFC 3339 with the organization's UTC offset, for example `2026-10-26T22:30:00-05:00`.

---

## Get Settings

`GET /api/organization/settings`

### Example Response (200 OK)

Delivery paused during a rush; takeaway open until 22:30 on a Monday in Bogotá.

```json
{
  "ordering": {
    "delivery": {
      "paused": true,
      "paused_reason": "cocina saturada",
      "schedule": [
        { "day": "fri", "opens": "18:00", "closes": "02:00" },
        { "day": "mon", "opens": "18:00", "closes": "22:30" }
      ]
    },
    "takeaway": {
      "paused": false,
      "paused_reason": null,
      "schedule": [
        { "day": "mon", "opens": "11:30", "closes": "22:30" }
      ]
    }
  },
  "feedback": {
    "review_link_enabled": false,
    "review_url": ""
  },
  "availability": {
    "delivery": {
      "available": false,
      "reason": "paused",
      "paused_reason": "cocina saturada",
      "closes_at": null,
      "next_opens_at": null
    },
    "takeaway": {
      "available": true,
      "reason": null,
      "paused_reason": null,
      "closes_at": "2026-10-26T22:30:00-05:00",
      "next_opens_at": null
    }
  },
  "timezone": "America/Bogota"
}
```

An organization that never saved settings reads as both types `paused: false`, `paused_reason: null`, `schedule: []`, both available, and `feedback` with the link off.

### Error Responses

**403 Forbidden** — the caller lacks `ordering:read`

```json
{
  "error": "Insufficient permissions",
  "required": "ordering:read"
}
```

---

## Patch Settings

`PATCH /api/organization/settings`

Changes only what the body contains. A type absent from the body is left unchanged, and so is any field absent from a type.

- `paused` / `paused_reason` need `ordering:pause`. Setting `paused: false` clears `paused_reason`.
- `schedule` needs `ordering:manage` and **replaces** that type's whole schedule. `[]` clears it (always open).
- A body touching both needs both permissions. If either is missing, the request is refused with `403` and **nothing** is stored.

The response is `200` in the same shape as [Get Settings](#get-settings), already reflecting the change. The instance that saved it uses the new settings at once. Other instances pick them up within 30 seconds.

### Request Body

```json
{
  "ordering": {
    "delivery": { "paused": true, "paused_reason": "cocina saturada" },
    "takeaway": { "schedule": [ { "day": "mon", "opens": "11:30", "closes": "22:30" } ] }
  }
}
```

| Field                              | Type           | Permission        |
| ---------------------------------- | -------------- | ----------------- |
| `ordering.<type>.paused`           | boolean        | `ordering:pause`  |
| `ordering.<type>.paused_reason`    | string \| null | `ordering:pause`  |
| `ordering.<type>.schedule`         | array          | `ordering:manage` |
| `feedback.review_link_enabled`     | boolean        | `settings:feedback:update` |
| `feedback.review_url`              | string         | `settings:feedback:update` |

`<type>` is `delivery` or `takeaway`. An invalid `feedback` section is checked before anything is written, so a body that also touches `ordering` changes nothing.

### Example Requests

Pause deliveries during a rush:

```json
{ "ordering": { "delivery": { "paused": true, "paused_reason": "cocina saturada" } } }
```

Resume them (the reason is cleared):

```json
{ "ordering": { "delivery": { "paused": false } } }
```

Set takeaway hours, with a Friday window past midnight:

```json
{
  "ordering": {
    "takeaway": {
      "schedule": [
        { "day": "mon", "opens": "11:30", "closes": "22:30" },
        { "day": "fri", "opens": "18:00", "closes": "02:00" }
      ]
    }
  }
}
```

### Error Responses

**400 Bad Request** — malformed body

```json
{ "error": "Invalid request body" }
```

**400 Bad Request** — invalid settings; `field` names the order type and the window

```json
{
  "error": "invalid ordering.takeaway.schedule[1]: overlaps ordering.takeaway.schedule[0]",
  "code": "INVALID_ORDERING_SETTINGS",
  "field": "ordering.takeaway.schedule[1]"
}
```

| `field`                              | When |
| ------------------------------------ | ---- |
| `ordering.<type>.schedule[i]`        | Bad day code, bad `HH:MM`, equal times, or overlapping an earlier window |
| `ordering.<type>.schedule`           | More than 21 windows |
| `ordering.<type>.paused_reason`      | Longer than 255 characters |

**403 Forbidden** — the body touches a field the caller may not change

```json
{
  "error": "Insufficient permissions",
  "required": "ordering:manage"
}
```

---

## Feedback

The `feedback` section decides whether guests who answer a [feedback request](./feedback.md) are given the Google review link.

| Field                 | Type    | Description |
| --------------------- | ------- | ----------- |
| `review_link_enabled` | boolean | Default `false`. When `true` and `review_url` is set, **every** recorded answer returns the link, whatever the rating. |
| `review_url`          | string  | An absolute `https` URL, or `""`. Default `""`. |

The rating never decides whether the link is shared: the link goes to every guest or to none. Low ratings (1–3) are flagged for a manager either way.

### Example Request

Turn the link on:

```json
{ "feedback": { "review_link_enabled": true, "review_url": "https://g.page/r/abc/review" } }
```

### Error Responses

**400 Bad Request** — `review_url` is not an absolute `https` URL; nothing is stored

```json
{
  "error": "invalid feedback.review_url: must be an absolute https URL or empty",
  "code": "INVALID_SETTINGS",
  "field": "feedback.review_url"
}
```

**403 Forbidden** — the caller lacks `settings:feedback:update`

```json
{
  "error": "Insufficient permissions",
  "required": "settings:feedback:update"
}
```
