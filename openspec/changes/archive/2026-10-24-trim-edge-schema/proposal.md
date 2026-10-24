## Why

The edge box runs the restaurant floor: taking orders, the kitchen, printing, stock, and syncing with the cloud. Even so, it carries the whole cloud schema and API: expenses, purchase entries, suppliers, the financial summary and daily close, e-invoicing, support documents and delivery zones. None of these are used on the box. Accounting runs in the cloud and can wait while the cloud is unreachable. Every cloud migration still runs on the box, and any of these endpoints called on the box creates data that never reaches the cloud. Expenses aren't synced at all, and edits to products, suppliers or users on the box are overwritten by the next pull. With `split-cloud-edge-binaries` in place, the edge binary can carry just the floor.

## What Changes

- **BREAKING (edge API):** the box stops serving the following. They stay in the cloud.
  - expenses and expense categories
  - purchase entries
  - suppliers and the supplier catalog (including `GET /api/products/:id/suppliers`)
  - the financial summary (`/api/financial/summary`)
  - the daily close (`/api/reports/daily-close`)
- **BREAKING (edge API):** the box becomes read-only for data the cloud owns: products, product ingredients and side-dish settings, product responsibilities, and users. Every read stays, and so do sign-in and `/api/auth/me`. Creating, editing, deleting, bulk-creating and resetting passwords move to the cloud. That includes the admin-key `POST /api/users` bootstrap; the box gets its users from the pull.
- **BREAKING (edge API):** a delivery order created on the box is refused with an explicit "deliveries are taken in the cloud" error. Today it fails with "outside delivery zone", because the box has no zones. Dine-in and takeaway orders on the box are unchanged.
- The edge's schema drops the tables it no longer uses:
  - `expenses`, `expense_categories`
  - `purchase_entries`, `purchase_entry_items`
  - `suppliers`, `supplier_catalog`
  - `support_documents`, `support_document_products`
  - `organization_einvoicing`, `invoice_sequences`, `delivery_zones`
  - `nodes`, `sync_inbox`

  The foreign key from `open_bills.delivery_zone_id` is dropped on the edge, and the column stays.
- The drop refuses to run while the box holds data that exists nowhere else:
  - any expense;
  - an expense category beyond the seeded ones;
  - a purchase entry still waiting to be pushed.

  The box then keeps running on its current schema, and the upgrade reports why it stopped.
- The edge's migrations split from the cloud's after the shared history (v1 to v71). Each binary runs the shared history plus its own set. A CI check compares the tables both sides keep, so a change to one side that the other lacks fails before it can break sync.
- Compatibility with boxes that haven't updated:
  - the cloud keeps sending suppliers in the pull, and keeps accepting purchase-entry pushes;
  - the new edge ignores suppliers in the pull.
- Out of scope: the frontend. A handoff lists the cards to mark `cloudOnly`: Gastos, Finanzas, Cierre de Caja, Entradas de Compra, Proveedores, Facturas, Documentos Soporte. It also lists the catalog and user edit actions to hide on the box.

## Capabilities

### New Capabilities
- `edge-schema`: what the edge box stores and serves. It keeps only the restaurant floor's tables and routes, is read-only for cloud-owned data, protects data that exists only on the box when tables are dropped, and stays in sync with boxes that haven't updated.

### Modified Capabilities
- `delivery-orders`: a delivery order is refused on the box with an explicit error, instead of failing the delivery-zone check.

## Impact

- **Depends on** `split-cloud-edge-binaries`. The edge binary must be separate before its routes, services and migrations can differ from the cloud's.
- **Code:** `internal/bootstrap` (edge routes and services), migration embedding and the runner (one shared set plus one per binary), the edge's pull applier (ignores suppliers), the order service or its edge wiring (refuses delivery), and a new CI check comparing the two schemas.
- **Data:** an irreversible drop on Laguna's box, guarded as described above. The cloud's schema doesn't change.
- **Clients:** the frontend served from the box loses the routes above (handoff to `laguna-escondida-frontend`). The MCP server talks to the cloud and isn't affected.
- **Docs:** `docs/api/*.md` notes which endpoints are cloud-only; the sync playbooks; `ARCHITECTURE.md`.
