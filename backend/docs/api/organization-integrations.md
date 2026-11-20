# Organization Integrations

An organization's credentials for a third-party provider, set by the platform operator or by the organization's admin, so a restaurant can be connected without changing the cloud's environment. Two providers:

- `wompi`: its private key creates the organization's [payment links](orders.md#payment-links-agent-orders) and its events secret verifies its [Wompi events](wompi-webhook.md).
- `marketing_app`: one signing secret, which signs the organization's [marketing events](marketing-events.md). The backend generates it and returns it once, to paste into each of the organization's channels in marketing-app; an existing value can also be put.

- **Cloud only.** The edge does not serve these routes (`404`) and never stores or receives integrations.
- **Two sets of routes, one behavior.** The platform routes (`/api/platform/organizations/:id/integrations…`) take `X-API-Key: <ADMIN_API_KEY>` and name the organization in the path; a user JWT or an organization's API key is refused with `401`. The staff routes (`/api/organization/integrations…`) take a sign-in token and act on the token's organization. Bodies, responses, validation and errors are the same.
- **Write-only secrets.** Secrets are stored encrypted (AES-256-GCM under `INTEGRATION_SECRETS_KEY`). No response, error or log line contains them, in plaintext or encrypted: reads return the last 4 characters of each as a hint. The one exception is the response to [generating the marketing-app secret](#generate-the-marketing-app-secret), which carries the new secret once.
- **One per provider.** Putting an integration again replaces it. A change applies to the next link, change or event, without a restart.

## Integration Fields

| Field                | Type   | Description                                                                 |
| -------------------- | ------ | --------------------------------------------------------------------------- |
| `provider`           | string | `wompi` or `marketing_app`                                                  |
| `environment`        | string | Wompi only: `sandbox` (`prv_test_` keys) or `production` (`prv_prod_` keys). Absent for `marketing_app` |
| `private_key_hint`   | string | Wompi only: last 4 characters of the private key                            |
| `events_secret_hint` | string | Wompi only: last 4 characters of the events secret                          |
| `signing_secret_hint` | string | marketing-app only: last 4 characters of the signing secret               |
| `webhook_path`       | string | Wompi only: `/api/webhooks/wompi/:organization_id`, the path to set as the events URL in Wompi's dashboard |
| `created_at`         | string | RFC 3339                                                                    |
| `updated_at`         | string | RFC 3339; when the keys were last put                                       |

## Endpoints

Platform operator (`X-API-Key: <ADMIN_API_KEY>`):

| Method | Endpoint                                                  | Description                              |
| ------ | --------------------------------------------------------- | ---------------------------------------- |
| GET    | `/api/platform/organizations/:id/integrations`            | List the organization's integrations     |
| GET    | `/api/platform/organizations/:id/integrations/:provider`  | Read one integration                     |
| PUT    | `/api/platform/organizations/:id/integrations/wompi`      | Create or replace the Wompi integration  |
| PUT    | `/api/platform/organizations/:id/integrations/marketing_app` | Put an existing marketing-app secret  |
| POST   | `/api/platform/organizations/:id/integrations/marketing_app/secret` | Generate the marketing-app secret (returned once) |
| DELETE | `/api/platform/organizations/:id/integrations/:provider`  | Remove an integration                    |

The organization's admin (`Authorization: Bearer <token>`, the token's organization):

| Method | Endpoint                                   | Permission            | Description                              |
| ------ | ------------------------------------------ | --------------------- | ---------------------------------------- |
| GET    | `/api/organization/integrations`           | `integrations:read`   | List the organization's integrations     |
| GET    | `/api/organization/integrations/:provider` | `integrations:read`   | Read one integration                     |
| PUT    | `/api/organization/integrations/wompi`     | `integrations:manage` | Create or replace the Wompi integration  |
| PUT    | `/api/organization/integrations/marketing_app` | `integrations:manage` | Put an existing marketing-app secret |
| POST   | `/api/organization/integrations/marketing_app/secret` | `integrations:manage` | Generate the marketing-app secret (returned once) |
| DELETE | `/api/organization/integrations/:provider` | `integrations:manage` | Remove an integration                    |

Only the admin role holds `integrations:read` and `integrations:manage`. A manager, any other role and an organization's API key (the WhatsApp agent) are refused with `403`.

Each section below shows the platform path; the staff path takes the same body and gives the same response.

### Common Errors

**401 Unauthorized** - Missing or wrong `X-API-Key` (platform routes), or a missing or invalid token (staff routes)

```json
{
  "error": "API key is required"
}
```

**403 Forbidden** - Staff routes: the caller lacks the permission

```json
{
  "error": "Insufficient permissions",
  "required": "integrations:manage"
}
```

**404 Not Found** - No organization has the id, or `:provider` is not `wompi` or `marketing_app` (for `…/secret`, not `marketing_app`)

```json
{
  "error": "Organization not found"
}
```

```json
{
  "error": "Unknown integration provider"
}
```

**503 Service Unavailable** - The cloud runs without `INTEGRATION_SECRETS_KEY`: integrations can be neither saved nor read

```json
{
  "error": "Integrations are not configured for this deployment",
  "code": "INTEGRATIONS_UNAVAILABLE"
}
```

---

## Put the Wompi Integration

Creates the organization's Wompi integration, or replaces its keys. From then on the organization takes agent orders. Both values are in Wompi's dashboard, under *Desarrolladores → Llaves del API* (private key) and *Secretos para integración técnica* (events secret).

```
PUT /api/platform/organizations/:id/integrations/wompi
```

### Request Body

| Field           | Type   | Required | Description                                           |
| --------------- | ------ | -------- | ----------------------------------------------------- |
| `private_key`   | string | Yes      | Starts with `prv_test_` or `prv_prod_` (trimmed)       |
| `events_secret` | string | Yes      | Starts with `test_events_` or `prod_events_` (trimmed) |

### Validation

Nothing is stored when the request is refused.

- Each value is required and must have its prefix. A public key (`pub_test_…`) or an integrity secret (`test_integrity_…`) is refused naming the field.
- Both values must be for the same environment (`test` or `prod`).
- They must match the cloud's `WOMPI_API_URL`: `sandbox.wompi.co` takes only `test` keys and `production.wompi.co` only `prod` keys. Any other host (a local stand-in) takes either.

### Example Request

```json
{
  "private_key": "prv_test_your_private_key_wxyz",
  "events_secret": "test_events_your_events_secret_9876"
}
```

### Example Response (200 OK)

```json
{
  "provider": "wompi",
  "environment": "sandbox",
  "private_key_hint": "wxyz",
  "events_secret_hint": "9876",
  "webhook_path": "/api/webhooks/wompi/019a1c2e-5f00-7a4b-8c3d-2e1f0a9b8c7d",
  "created_at": "2026-11-19T15:04:05Z",
  "updated_at": "2026-11-19T15:04:05Z"
}
```

### Error Responses

**400 Bad Request** - A value is missing or has the wrong prefix (the value itself is never echoed)

```json
{
  "error": "invalid private_key: must start with prv_test_ or prv_prod_",
  "field": "private_key"
}
```

**400 Bad Request** - The keys are for different environments, or not the cloud's

```json
{
  "error": "the keys are for production and this cloud uses Wompi's sandbox"
}
```

```json
{
  "error": "the private key is for sandbox and the events secret for production"
}
```

---

## Generate the Marketing-App Secret

Creates the organization's marketing-app integration with a new random secret, or replaces the one it has (rotation). The response is the only time the secret is returned: paste it into **each** of the organization's channels in marketing-app, under *Canal → Ajustes → Secreto de eventos del sistema de pedidos*. From then on its events are signed with it.

```
POST /api/platform/organizations/:id/integrations/marketing_app/secret
```

No request body. The secret is 32 random bytes as 43 URL-safe base64 characters, without padding. The response carries `Cache-Control: no-store`.

### Example Response (200 OK)

```json
{
  "provider": "marketing_app",
  "signing_secret_hint": "Q7xk",
  "created_at": "2026-11-19T15:04:05Z",
  "updated_at": "2026-11-19T15:04:05Z",
  "signing_secret": "<43 URL-safe base64 characters, ending in Q7xk>"
}
```

Regenerating while marketing-app still holds the old secret makes it answer `401` until the new one is pasted on every channel. Those events are retried hourly for up to 72 hours, so they arrive late, not lost.

### Error Responses

**404 Not Found** - `POST …/integrations/wompi/secret`, or any provider other than `marketing_app`: nothing changes

```json
{
  "error": "Unknown integration provider"
}
```

---

## Put the Marketing-App Secret

Creates or replaces the marketing-app integration with a secret the organization already uses in marketing-app, so it moves over without touching marketing-app. The Ajustes page does not offer this; it is API only.

```
PUT /api/platform/organizations/:id/integrations/marketing_app
```

### Request Body

| Field            | Type   | Required | Description                                               |
| ---------------- | ------ | -------- | --------------------------------------------------------- |
| `signing_secret` | string | Yes      | At least 32 characters, no whitespace (trimmed at the ends) |

### Example Request

```json
{
  "signing_secret": "the-secret-marketing-app-already-holds-for-laguna"
}
```

### Example Response (200 OK)

```json
{
  "provider": "marketing_app",
  "signing_secret_hint": "guna",
  "created_at": "2026-11-19T15:04:05Z",
  "updated_at": "2026-11-19T15:04:05Z"
}
```

### Error Responses

**400 Bad Request** - The secret is missing, shorter than 32 characters or contains whitespace (the value is never echoed); nothing is stored

```json
{
  "error": "invalid signing_secret: must be at least 32 characters",
  "field": "signing_secret"
}
```

---

## List Integrations

```
GET /api/platform/organizations/:id/integrations
```

Ordered by provider; empty when the organization has none.

### Example Response (200 OK)

```json
{
  "integrations": [
    {
      "provider": "wompi",
      "environment": "sandbox",
      "private_key_hint": "wxyz",
      "events_secret_hint": "9876",
      "webhook_path": "/api/webhooks/wompi/019a1c2e-5f00-7a4b-8c3d-2e1f0a9b8c7d",
      "created_at": "2026-11-19T15:04:05Z",
      "updated_at": "2026-11-19T15:04:05Z"
    },
    {
      "provider": "marketing_app",
      "signing_secret_hint": "Q7xk",
      "created_at": "2026-11-19T15:10:00Z",
      "updated_at": "2026-11-19T15:10:00Z"
    }
  ]
}
```

---

## Get an Integration

```
GET /api/platform/organizations/:id/integrations/:provider
```

Responds with the integration as in the list, or **404** `{"error": "Integration not found"}` when the organization has none for the provider.

---

## Remove an Integration

```
DELETE /api/platform/organizations/:id/integrations/:provider
```

Responds **204 No Content**, or **404** `{"error": "Integration not found"}` when there was none.

- `wompi`: from then on the organization's agent orders are refused with `422 PAYMENT_LINKS_NOT_CONFIGURED`. Links already created stay as they are; their Wompi events are refused until the integration is put again.
- `marketing_app`: from then on the organization records no marketing events. Events already recorded stay pending and expire after 72 hours unless a secret is put or generated again.

---

## Configuration (cloud)

| Env var                   | Required | Description |
| ------------------------- | -------- | ----------- |
| `INTEGRATION_SECRETS_KEY` | No       | 32 random bytes, base64 (`openssl rand -base64 32`). Without it the routes answer `503`, no organization takes agent orders and none records marketing events. A value that is not base64 or not 32 bytes fails startup |

`WOMPI_PRIVATE_KEY_<SLUG>`, `WOMPI_EVENTS_SECRET_<SLUG>` and `MARKETING_APP_EVENTS_SECRET_<SLUG>` are **ignored**: startup warns once per variable still set, by name. Unset them after putting the secrets here.

### Losing or changing the key

Every stored secret is encrypted under `INTEGRATION_SECRETS_KEY`; there is no re-encryption. If the key is lost or replaced, the stored secrets can't be decrypted and are treated as absent: each organization's agent orders are refused and its agent orders and reservations record no marketing events, and the cloud logs an error naming the organization and provider (never the value) on each attempt. Put each organization's keys again with the PUT, or generate the marketing-app secret again, to restore it.
