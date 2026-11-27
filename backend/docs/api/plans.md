# Plans API

The price of each plan. The cloud's price table is the only source of an amount: no request can supply one.

- **Cloud only.** The route does not exist on an edge box.
- **No authentication.** It returns constants, so it has no rate limit beyond the global one.
- **Pesos, no tax.** The annual price is ten times the monthly price.

## Endpoints

| Method | Endpoint     | Auth | Description                          |
| ------ | ------------ | ---- | ------------------------------------ |
| GET    | `/api/plans` | None | Each plan's monthly and annual price |

---

## List Plan Prices

`GET /api/plans`

### Response Fields

| Field             | Type    | Description                                              |
| ----------------- | ------- | -------------------------------------------------------- |
| `currency`        | string  | Always `COP`                                             |
| `trial_days`      | integer | Length of a trial, in days                               |
| `plans`           | array   | Cheapest first                                           |
| `plans[].id`      | string  | `starter`, `einvoicing` or `full` (see [Plans](./organizations.md#plans)) |
| `plans[].monthly` | integer | Pesos per month                                          |
| `plans[].annual`  | integer | Pesos per year                                           |

### Example Response (200 OK)

```json
{
  "currency": "COP",
  "trial_days": 14,
  "plans": [
    { "id": "starter", "monthly": 25000, "annual": 250000 },
    { "id": "einvoicing", "monthly": 40000, "annual": 400000 },
    { "id": "full", "monthly": 100000, "annual": 1000000 }
  ]
}
```

The landing keeps a display copy of these prices; its release fails when the copy differs from this response.
