# Stock API

On-hand stock and the movement history behind it.

## Where each endpoint answers

Stock is owned by the **cloud**. On-hand is never assigned from a reported amount: it is the
sum of the movements recorded against a product, and the cloud is the only node that folds
them. That is why authoring lives there.

| Endpoint                                  | Cloud | Edge   |
| ----------------------------------------- | ----- | ------ |
| `GET /api/stock`                          | Yes   | Yes    |
| `GET /api/stock/sync-freshness`           | Yes   | `404`  |
| `POST /api/stock`                         | Yes   | `404`  |
| `PUT /api/stock/:product_id/add-or-decrease` | Yes | `404`  |
| `DELETE /api/stock/:product_id`           | Yes   | `404`  |
| `POST /api/stock/bulk`                    | Yes   | `404`  |
| `POST /api/stock/opening-balances`        | Yes   | `404`  |

The restaurant still changes stock — a sale decrements it and a delivery recorded on site
increases it — but only as a consequence of a business event, replicated to the cloud as a
signed change. The edge's own numbers are a **daily copy** of the cloud's, refreshed once a
day; between refreshes they reflect the restaurant's movements and not the office's.

## Endpoints

| Method | Endpoint                                     | Description                                   |
| ------ | -------------------------------------------- | --------------------------------------------- |
| GET    | `/api/stock`                                 | List on-hand stock                            |
| GET    | `/api/stock/sync-freshness`                  | How current the cloud's view of the edge is   |
| POST   | `/api/stock`                                 | Create a stock row for a product              |
| PUT    | `/api/stock/:product_id/add-or-decrease`     | Adjust on-hand by a signed change             |
| DELETE | `/api/stock/:product_id`                     | Delete a stock row                            |
| POST   | `/api/stock/bulk`                            | Record a batch count                          |
| POST   | `/api/stock/opening-balances`                | Cutover only — baseline the movement ledger   |

---

## Stock Sync Freshness

Reports when the cloud last folded a movement replicated from the restaurant.

```
GET /api/stock/sync-freshness
```

Requires `stock:read`. Cloud only.

### Why it exists

A batch count is recorded as `counted amount - the cloud's current on-hand`. Sales made at the
restaurant that have not reached the cloud yet are missing from that current on-hand, so they
subtract **twice**: once implicitly (the goods were already off the shelf when counted) and
again when they arrive. The size of that error is bounded by how far behind the restaurant's
push is — which is exactly what this endpoint measures.

Read it before offering a batch count and warn the counter when the value is large.

### Response `200 OK`

```json
{
  "last_movement_applied_at": "2026-09-22T14:31:07.482913Z",
  "staleness_seconds": 154
}
```

| Field                      | Type          | Description                                                                 |
| -------------------------- | ------------- | --------------------------------------------------------------------------- |
| `last_movement_applied_at` | string \| null | When the cloud last applied a movement replicated from a node, UTC RFC 3339 |
| `staleness_seconds`        | number \| null | Seconds since that moment                                                   |

A node that has never pushed a movement returns nulls:

```json
{
  "last_movement_applied_at": null,
  "staleness_seconds": null
}
```

Treat nulls as **unknown**, not as fresh. Either the restaurant has never replicated a stock
movement, or this cloud has no edge attached — neither means "nothing is outstanding".

### Errors

| Status | Condition                                   |
| ------ | ------------------------------------------- |
| `401`  | Missing or invalid token                    |
| `403`  | Token lacks `stock:read`                    |
| `404`  | Called on a node running in edge mode       |
| `500`  | The freshness query failed                  |


---

## List Stock

Returns the on-hand amount for every product that has a stock row.

```
GET /api/stock
```

Requires `stock:read`. Served on **both** nodes — but they answer different questions. The
cloud's number is authoritative. The edge's is a copy taken at the last daily refresh, moved
since only by what the restaurant itself sold; anything the office recorded today is not in
it yet. Label it accordingly in any screen that shows both.

### Response `200 OK`

```json
{
  "stocks": [
    {
      "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e",
      "version": 1,
      "amount": 140,
      "unit_of_measure": "unit",
      "created_at": "2026-09-01T12:00:00Z",
      "updated_at": "2026-09-22T18:08:00.048818Z"
    }
  ],
  "total": 1
}
```

A negative `amount` is legitimate: on-hand is the sum of the movements recorded against the
product, and a negative total is a real signal (usually a sale whose delivery was never
recorded), not an error to hide.

---

## Create Stock

Gives a product its first stock row. Records an `adjustment` movement for the full amount.

```
POST /api/stock
```

Requires `stock:create`. **Cloud only.**

### Request Body

| Field      | Type   | Required | Description                     |
| ---------- | ------ | -------- | ------------------------------- |
| product_id | string | Yes      | UUID of an existing product     |
| amount     | number | Yes      | Starting on-hand                |

```json
{ "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e", "amount": 100 }
```

### Response `201 Created`

```json
{
  "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e",
  "version": 1,
  "amount": 100,
  "unit_of_measure": "unit",
  "created_at": "2026-09-22T18:00:00Z",
  "updated_at": "2026-09-22T18:00:00Z"
}
```

### Errors

| Status | Condition                                        |
| ------ | ------------------------------------------------ |
| `400`  | Malformed body                                   |
| `401`  | Missing or invalid token                         |
| `403`  | Token lacks `stock:create`                       |
| `404`  | Product not found, or called on an edge node     |
| `409`  | Stock already exists for this product            |
| `500`  | Write failed                                     |

---

## Adjust Stock

Applies a signed change to a product's on-hand and records an `adjustment` movement.

```
PUT /api/stock/:product_id/add-or-decrease
```

Requires `stock:update`. **Cloud only.**

### Request Body

| Field  | Type   | Required | Description                                          |
| ------ | ------ | -------- | ---------------------------------------------------- |
| change | number | Yes      | Signed change — positive adds, negative removes       |

```json
{ "change": -15 }
```

The ledger records the **change**, never the resulting amount. That is what lets a sale
replicated from the restaurant afterwards land on top of this correction instead of racing it.

### Response `204 No Content`

### Errors

| Status | Condition                                                      |
| ------ | -------------------------------------------------------------- |
| `400`  | Malformed body, or the product's version no longer matches      |
| `401`  | Missing or invalid token                                        |
| `403`  | Token lacks `stock:update`                                      |
| `404`  | Product or stock row not found, or called on an edge node       |
| `500`  | Write failed                                                    |

---

## Delete Stock

Soft-deletes a product's stock row.

```
DELETE /api/stock/:product_id
```

Requires `stock:delete`. **Cloud only.**

A delete removes the row rather than moving its amount, so **no movement is recorded**. The
restaurant learns about it at the next daily refresh, which carries `deleted_at`. Because the
product's past movements remain on the ledger, a deleted product will read as diverged in the
reconciliation report — expected, and the reason deletes should be rare.

### Response `204 No Content`

### Errors

| Status | Condition                                         |
| ------ | ------------------------------------------------- |
| `401`  | Missing or invalid token                          |
| `403`  | Token lacks `stock:delete`                        |
| `404`  | Stock not found, or called on an edge node        |
| `500`  | Write failed                                      |

---

## Record a Batch Count

Submits counted amounts for one or more products.

```
POST /api/stock/bulk
```

Requires `stock:create`. **Cloud only.**

### Request Body

| Field         | Type   | Required | Description                        |
| ------------- | ------ | -------- | ---------------------------------- |
| items         | array  | Yes      | One entry per counted product      |
| items[].product_id | string | Yes | UUID of an existing product        |
| items[].amount     | number | Yes | The counted amount (min 0)         |

```json
{
  "items": [
    { "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e", "amount": 20 },
    { "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6f", "amount": 8 }
  ]
}
```

A count of `20` against a current on-hand of `30` records a movement of **`-10`**, not `20`.
An item whose counted amount already equals the current on-hand records nothing.

> ### ⚠️ Read the freshness signal before offering a count
>
> The delta is computed against *the cloud's current on-hand*. Sales made at the restaurant
> that have not been replicated yet are missing from that number, so they will subtract
> **twice** — once implicitly (the goods were already off the shelf when counted) and again
> when they arrive.
>
> Call `GET /api/stock/sync-freshness` first and warn the counter when the value is large or
> null. The error is bounded by how far behind the restaurant's push is, which is exactly
> what that endpoint measures.

### Response `204 No Content`

### Errors

| Status | Condition                                                   |
| ------ | ----------------------------------------------------------- |
| `400`  | Malformed body, or a product's version no longer matches     |
| `401`  | Missing or invalid token                                     |
| `403`  | Token lacks `stock:create`                                   |
| `404`  | One or more products not found, or called on an edge node    |
| `500`  | Write failed                                                 |

---

## Write Opening Balances

Cutover only. Writes the one `opening_balance` movement per product that makes each amount
equal the sum of its movements.

```
POST /api/stock/opening-balances
```

Requires the **admin API key** (`X-API-Key`), not a JWT — this is an operational step run
once when the cloud adopts on-hand, not something a stock manager does. **Cloud only.**

### Why it is needed

Movement history from before replication was never sent to the cloud, so amounts are real but
the movements explaining them are missing. This closes that gap so `amount == SUM(change)`
holds from cutover forward. See `docs/playbooks/STOCK_CUTOVER_RUNBOOK.md`.

### Response `200 OK`

```json
{ "checked_products": 214, "written_balances": 198 }
```

Safe to re-run: a second call finds no gap and reports `"written_balances": 0`.

### Errors

| Status | Condition                                    |
| ------ | -------------------------------------------- |
| `401`  | Missing or wrong admin API key               |
| `404`  | Called on a node running in edge mode        |
| `500`  | Write failed                                 |
