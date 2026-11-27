## ADDED Requirements

### Requirement: The cloud holds the price of each plan

The cloud SHALL hold one price table in Colombian pesos. The annual price SHALL be ten times the monthly price. No tax SHALL be added to a price.

| Plan | Monthly | Annual |
|---|---|---|
| `starter` | 25000 | 250000 |
| `einvoicing` | 40000 | 400000 |
| `full` | 100000 | 1000000 |

Every amount the cloud shows or charges for a plan SHALL come from this table. No request SHALL be able to supply an amount.

#### Scenario: Annual price

- **WHEN** the price of `full` for the annual period is read
- **THEN** it is 1000000 pesos

#### Scenario: A request carries an amount

- **WHEN** a billing setup request includes an `amount` field
- **THEN** the field is ignored and the amount comes from the price table

### Requirement: Anyone can read the prices

`GET /api/plans` SHALL exist only in the cloud, SHALL require no authentication, and SHALL answer `200` with the currency, the trial length in days, and each plan's id with its monthly and annual price, cheapest first:

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

#### Scenario: Reading the prices without signing in

- **WHEN** `GET /api/plans` is called with no credentials
- **THEN** the response is `200` with the three plans and their prices

#### Scenario: Edge box

- **WHEN** `GET /api/plans` is called on an edge box
- **THEN** the route is not found
