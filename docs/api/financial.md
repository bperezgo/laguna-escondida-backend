# Financial Summary

Financial summary endpoint that aggregates revenue (bills), expenses, and purchase entries for a given date range, providing a centralized view of the company's financial status.

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
    "count": 150
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
