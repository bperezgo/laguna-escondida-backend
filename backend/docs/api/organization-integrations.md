# Organization Integrations

An organization's credentials for a third-party provider, set by the platform operator or by the organization's admin, so a restaurant can be connected without changing the cloud's environment. `wompi` is the only provider: its private key creates the organization's [payment links](orders.md#payment-links-agent-orders) and its events secret verifies its [Wompi events](wompi-webhook.md).

- **Cloud only.** The edge does not serve these routes (`404`) and never stores or receives integrations.
- **Two sets of routes, one behavior.** The platform routes (`/api/platform/organizations/:id/integrations…`) take `X-API-Key: <ADMIN_API_KEY>` and name the organization in the path; a user JWT or an organization's API key is refused with `401`. The staff routes (`/api/organization/integrations…`) take a sign-in token and act on the token's organization. Bodies, responses, validation and errors are the same.
- **Write-only secrets.** Secrets are stored encrypted (AES-256-GCM under `INTEGRATION_SECRETS_KEY`). No response, error or log line contains them, in plaintext or encrypted: reads return the last 4 characters of each as a hint.
- **One per provider.** Putting an integration again replaces it. A change applies to the next link, change or event, without a restart.

## Integration Fields

| Field                | Type   | Description                                                                 |
| -------------------- | ------ | --------------------------------------------------------------------------- |
| `provider`           | string | `wompi`                                                                     |
| `environment`        | string | `sandbox` (`prv_test_` keys) or `production` (`prv_prod_` keys)             |
| `private_key_hint`   | string | Last 4 characters of the private key                                        |
| `events_secret_hint` | string | Last 4 characters of the events secret                                      |
| `webhook_path`       | string | `/api/webhooks/wompi/:organization_id`: the path to set as the events URL in Wompi's dashboard |
| `created_at`         | string | RFC 3339                                                                    |
| `updated_at`         | string | RFC 3339; when the keys were last put                                       |

## Endpoints

Platform operator (`X-API-Key: <ADMIN_API_KEY>`):

| Method | Endpoint                                                  | Description                              |
| ------ | --------------------------------------------------------- | ---------------------------------------- |
| GET    | `/api/platform/organizations/:id/integrations`            | List the organization's integrations     |
| GET    | `/api/platform/organizations/:id/integrations/:provider`  | Read one integration                     |
| PUT    | `/api/platform/organizations/:id/integrations/wompi`      | Create or replace the Wompi integration  |
| DELETE | `/api/platform/organizations/:id/integrations/:provider`  | Remove an integration                    |

The organization's admin (`Authorization: Bearer <token>`, the token's organization):

| Method | Endpoint                                   | Permission            | Description                              |
| ------ | ------------------------------------------ | --------------------- | ---------------------------------------- |
| GET    | `/api/organization/integrations`           | `integrations:read`   | List the organization's integrations     |
| GET    | `/api/organization/integrations/:provider` | `integrations:read`   | Read one integration                     |
| PUT    | `/api/organization/integrations/wompi`     | `integrations:manage` | Create or replace the Wompi integration  |
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

**404 Not Found** - No organization has the id, or `:provider` is not `wompi`

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

Responds **204 No Content**, or **404** `{"error": "Integration not found"}` when there was none. From then on the organization's agent orders are refused with `422 PAYMENT_LINKS_NOT_CONFIGURED`. Links already created stay as they are; their Wompi events are refused until the integration is put again.

---

## Configuration (cloud)

| Env var                   | Required | Description |
| ------------------------- | -------- | ----------- |
| `INTEGRATION_SECRETS_KEY` | No       | 32 random bytes, base64 (`openssl rand -base64 32`). Without it the routes answer `503` and no organization takes agent orders. A value that is not base64 or not 32 bytes fails startup |

`WOMPI_PRIVATE_KEY_<SLUG>` and `WOMPI_EVENTS_SECRET_<SLUG>` are **ignored**: startup warns once per variable still set, by name. Unset them after putting the keys here.

### Losing or changing the key

Every stored secret is encrypted under `INTEGRATION_SECRETS_KEY`; there is no re-encryption. If the key is lost or replaced, the stored secrets can't be decrypted and are treated as absent: each organization's agent orders are refused, and the cloud logs an error naming the organization and provider (never the value) on each attempt. Put each organization's keys again with the PUT to restore it.
