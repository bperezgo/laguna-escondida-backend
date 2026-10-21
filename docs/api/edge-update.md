# Edge Update (Owner-Triggered Appliance Updates)

Lets an admin see whether a newer appliance release is staged on the box and ask for it to be
applied. The backend only reads the updater's state files and drops a request file; the
appliance updater (`laguna-escondida-edge`, `docs/UPDATE_PROTOCOL.md`) verifies and applies the
release on its own.

> **Edge mode only.** These routes are registered only when the backend runs with
> `APP_MODE=edge`. In `cloud` mode they do not exist (404).

**Auth:** JWT; requires the `edge:update` permission (admin and manager roles).

## Endpoints

| Method | Endpoint           | Description                              |
| ------ | ------------------ | ---------------------------------------- |
| GET    | `/api/edge/update` | Installed, staged and last update state  |
| POST   | `/api/edge/update` | Ask the updater to apply the staged one  |

## Configuration

| Env var            | Meaning                                                      | Example                |
| ------------------ | ------------------------------------------------------------ | ---------------------- |
| `LAGUNA_STATE_DIR` | The updater's state directory. Unset turns the feature off.   | `C:\laguna-edge\state` |

---

## Get Update State

`GET /api/edge/update`

Each field is `null` when its file is missing or does not parse; a bad file never fails the
request. Times are passed through as the updater wrote them (RFC 3339).

### Example Response (200 OK)

```json
{
  "enabled": true,
  "installed": { "version": "v1.0.0", "applied_at": "2026-10-19T15:02:11-05:00" },
  "available": {
    "version": "v1.0.1",
    "current": "v1.0.0",
    "published_at": "2026-10-19T14:00:00Z",
    "notes_url": "https://github.com/bperezgo/laguna-escondida-edge/releases/tag/v1.0.1",
    "components": { "edge-node": "v1.1.19", "next": "v1.0.3" },
    "checked_at": "2026-10-19T14:30:00-05:00"
  },
  "requested": null,
  "in_progress": null,
  "last_result": {
    "version": "v1.0.0",
    "outcome": "success",
    "message": "Updated to edge-node v1.1.18, next v1.0.2.",
    "started_at": "2026-10-12T15:00:05-05:00",
    "finished_at": "2026-10-12T15:04:40-05:00",
    "from": { "edge-node": "v1.1.17", "next": "v1.0.1" },
    "to": { "edge-node": "v1.1.18", "next": "v1.0.2" }
  }
}
```

`last_result.outcome` is one of `success`, `refused`, `rolled_back`, `rollback_failed`,
`drill_rolled_back`.

### Feature off (200 OK)

```json
{ "enabled": false }
```

---

## Request Update

`POST /api/edge/update`

Writes `update-requested.json` for the staged version. The POS goes down for a few minutes
while the updater applies it; poll `GET` for `in_progress` and then `last_result`.

### Request Body

| Field     | Type   | Required | Description                                         |
| --------- | ------ | -------- | --------------------------------------------------- |
| `version` | string | Yes      | Must equal `available.version` from `GET`           |

```json
{ "version": "v1.0.1" }
```

### Example Response (202 Accepted)

```json
{
  "version": "v1.0.1",
  "requested_by": "019a0000-0000-7000-8000-0000000000aa",
  "requested_at": "2026-10-19T15:00:00-05:00"
}
```

### Error Responses

**400 Bad Request** — missing, empty or non-string `version`

```json
{ "error": "validation_error", "message": "version is required" }
```

**404 Not Found** — feature off (`LAGUNA_STATE_DIR` unset)

```json
{ "error": "edge_update_disabled" }
```

**409 Conflict** — `error` is one of:

| `error`            | When                                                  |
| ------------------ | ----------------------------------------------------- |
| `no_staged_update` | No release is staged (or its file does not parse)     |
| `version_mismatch` | `version` is not the staged release                   |
| `update_pending`   | A request or an apply is already in progress          |

```json
{ "error": "version_mismatch", "message": "version is not the staged update" }
```
