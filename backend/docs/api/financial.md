# Financial Summary

Financial summary endpoint that aggregates revenue (bills), expenses, and purchase entries for a given date range, providing a centralized view of the company's financial status.

**Cloud only.** `GET /api/financial/summary` and the daily close (`GET /api/reports/daily-close`) exist only in the cloud binary; on the restaurant's box (edge) they return `404`. Accounting is done in the cloud, and data recorded on the box would never reach it.

## Endpoints

| Method | Endpoint                  | Description                          |
| ------ | ------------------------- | ------------------------------------ |
| GET    | `/api/financial/summary`  | Get financial summary for date range |

---

## Get Financial Summary

`GET /api/financial/summary`

Returns aggregated financial data including revenue, expenses (with category breakdown), purchases, and net income for the specified date range.

### Query Parameters

| Parameter    | Type   | Required | Description                          |
| ------------ | ------ | -------- | ------------------------------------ |
| `start_date` | string | Yes      | Start of date range (RFC3339 format) |
| `end_date`   | string | Yes      | End of date range (RFC3339 format)   |

### Example Request

```
GET /api/financial/summary?start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z
```

### Example Response (200 OK)

```json
{
  "start_date": "2024-01-01T00:00:00Z",
  "end_date": "2024-12-31T23:59:59Z",
  "revenue": {
    "total_amount": "5000000",
    "total_vat": "950000",
    "total_ico": "400000",
    "total_discount": "100000",
    "total_tip": "50000",
    "count": 150,
    "total_delivery_fee": "13000",
    "total_delivery_fee_base": "12037.04",
    "total_delivery_fee_ico": "962.96"
  },
  "expenses": {
    "total_amount": "1500000",
    "by_category": [
      {
        "category_id": "550e8400-e29b-41d4-a716-446655440001",
        "category_name": "Servicios",
        "category_code": "SVC",
        "total_amount": "800000",
        "count": 12
      },
      {
        "category_id": "550e8400-e29b-41d4-a716-446655440002",
        "category_name": "Arriendo",
        "category_code": "RNT",
        "total_amount": "700000",
        "count": 12
      }
    ],
    "count": 24
  },
  "purchases": {
    "total_amount": "2000000",
    "count": 50
  },
  "net_income": "1500000"
}
```

### Error Responses

**400 Bad Request** - Missing parameters

```json
{
  "error": "start_date and end_date are required"
}
```

**400 Bad Request** - Invalid date format

```json
{
  "error": "Invalid start_date format. Use RFC3339 (e.g., 2024-01-01T00:00:00Z)"
}
```

**500 Internal Server Error**

```json
{
  "error": "Failed to get financial summary"
}
```

### Notes

- **Net Income** = Revenue.TotalAmount - Expenses.TotalAmount - Purchases.TotalAmount
- Revenue is aggregated from the `bills` table (paid orders)
- Expenses are aggregated from the `expenses` table (operational costs)
- Purchases are aggregated from the `purchase_entries` table (cost of goods)
- All monetary amounts use `NUMERIC(19,4)` precision
- Expense breakdown by category is sorted by total amount descending
- A negative `net_income` indicates a loss for the period
- `revenue.total_delivery_fee` (with `_base` and `_ico`) is the delivery fees collected in the period. They are **already included** in `total_amount` (the fee's base) and `total_ico` (its ICO), so `net_income` and the other totals reconcile as before; with no delivery fee they are `"0"`.

---

## Daily Close

`GET /api/reports/daily-close?date=YYYY-MM-DD`

The read-only end-of-day money reconciliation for one business day (America/Bogota); `date` defaults to today. Requires `financial:read`.

### Example Response (200 OK)

```json
{
  "start_date": "2026-11-20T05:00:00Z",
  "end_date": "2026-11-21T04:59:59Z",
  "total_orders": 12,
  "total_collected": "1920000",
  "total_net": "1700000",
  "total_vat": "150000",
  "total_ico": "70000",
  "total_discount": "0",
  "total_tip": "0",
  "total_delivery_fee": "13000",
  "total_delivery_fee_base": "12037.04",
  "total_delivery_fee_ico": "962.96",
  "by_payment_method": [
    { "payment_method": "cash", "collected": "1240000", "net": "1100000", "count": 7 },
    { "payment_method": "credit_card", "collected": "680000", "net": "600000", "count": 5 }
  ]
}
```

- `total_collected` is the sum of `by_payment_method[].collected` (gross the customers paid).
- `total_delivery_fee` / `_base` / `_ico` break out the delivery fees of the day. They are **already inside** `total_collected`, `total_net`, `total_ico` and the per-method rows; they are `"0"` on a day without paid delivery fees.
