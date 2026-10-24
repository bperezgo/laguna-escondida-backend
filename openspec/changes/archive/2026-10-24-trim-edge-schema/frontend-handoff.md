## Frontend handoff (`laguna-escondida-frontend`)

The box (edge) now serves only the restaurant floor. Ship this with, or before, the edge release, so staff never land on a `404`.

### Mark these cards `cloudOnly`

Gastos · Finanzas · Cierre de Caja · Entradas de Compra · Proveedores · Facturas · Documentos Soporte

On the box their endpoints return `404`:
- `/api/expenses*`, `/api/expense-categories*`
- `/api/financial/summary`, `/api/reports/daily-close`
- `/api/purchase-entries*`
- `/api/suppliers*`, `/api/products/:id/suppliers`
- `/api/invoices*`, `/api/support-documents*` (already cloud only)

### Hide the edit actions on the box

Reads still work on the box, from what it last pulled. Writes are cloud only and return `404` on the box:

| Area | Hide on the box | Still served on the box |
| --- | --- | --- |
| Products | create, bulk create, edit, delete | `GET /api/products`, `/categories`, `/:id` |
| Ingredients / side dishes | add, edit, remove, configure or clear side dish | `GET /api/products/:id/ingredients`, `/side-dishes` |
| Preparation responsibilities | create, edit, delete | `GET /api/product-responsibilities/:id` |
| Users | create, edit, reset password, delete | `GET /api/admin/users`, `/users/:id`, `/roles`, sign-in, `/api/auth/me` |

### Deliveries

A `delivery` order sent to the box is refused before any zone check, and no order is created:

```json
422 {"error": "Deliveries are taken in the cloud", "code": "DELIVERY_CLOUD_ONLY"}
```

`dine_in` and `takeaway` are unchanged on the box. On the box, hide or disable the delivery order type, and show this message if the code comes back.
