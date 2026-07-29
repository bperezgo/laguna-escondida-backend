---
description: Ingest a supplier invoice PDF → reconcile/create products and record a purchase entry (which auto-updates stock), then attach the PDF. Colombian FACTURA format.
argument-hint: <invoice.pdf> [supplier name or id] [default category]
allowed-tools: Read, mcp__laguna-escondida__list_suppliers, mcp__laguna-escondida__get_supplier, mcp__laguna-escondida__list_products, mcp__laguna-escondida__list_supplier_products, mcp__laguna-escondida__list_supplier_purchase_entries
---

# Ingest a supplier invoice

Turn a supplier invoice PDF into catalog + stock updates through the
`laguna-escondida` MCP tools, with a **human confirmation gate before any write**.

Do **not** insert products or adjust stock by hand: recording the purchase entry
(step 7) increases stock automatically — calling `adjust_stock`/`create_stock`
as well would double-count.

## Inputs

- **Invoice PDF:** `$1` (path). Read it with the Read tool. If `$ARGUMENTS` is
  empty, ask the user to provide/attach the PDF.
- **Supplier:** `$2` (name or UUID). Resolve a name with `list_suppliers`.
- **Default category:** `$3` (defaults to `SNACKS`).

## Invoice format (Colombian FACTURA ELECTRONICA DE VENTA)

Extract each product row:

| Invoice column | Maps to |
|---|---|
| DESCRIPCION | product `name` (keep as-is / uppercase) |
| COD BARRAS | `sku` (barcode) |
| CODIGO | `supplier_sku` |
| IVA (%) | `vat` — per line; may be 0 / 5 / 19 |
| VALOR UNITARIO | pre-tax unit cost → `unit_cost` on the purchase entry, and the basis for the price |
| CANT / UNID | `quantity` on the purchase entry |

Product selling price: `total_price_with_taxes = VALOR UNITARIO * (1 + IVA/100)`
(e.g. 5462 @ 19% → `"6499.78"`).

## Steps

1. **Read & extract.** Read the PDF and pull each line: name, barcode (sku),
   supplier_sku, vat, unit cost, quantity — plus the **invoice number** and
   **invoice date**. Flag any row missing a barcode.

2. **Resolve the supplier.** `list_suppliers`, match `$2` by name or id (use
   `get_supplier` to confirm). If it doesn't exist, propose `create_supplier`
   and confirm before creating.

3. **Idempotency check.** `list_supplier_purchase_entries(supplier_id)`. If an
   entry already exists with this invoice number (`invoice_reference`), **STOP**
   and report it — do not ingest the same invoice twice.

4. **Match to existing products.** Call `list_supplier_products(supplier_id)`
   and `list_products`. Match each line to an existing product by `sku`
   (barcode) first, then `supplier_sku`, then a close name match. Split the lines
   into **existing** (already have a product_id) and **new**.

5. **Plan & confirm — REQUIRED, do not skip.** Present a review:
   - **New products** to create: name, sku, vat, computed price, category, and the applied defaults.
   - **Existing products** that will receive stock (show current stock if available).
   - **Purchase entry** to record: supplier, invoice #, date, line count, total.
   Call out low-confidence fields (**category**, **vat**) and any missing
   barcodes. **Wait for an explicit "yes" before any write tool call.**

6. **Create the new products.** `bulk_create_products` with `body.supplier_id` =
   the supplier and, per item:
   - `name`, `sku` (barcode), `supplier_sku`, `vat` (per line), `total_price_with_taxes` (computed)
   - defaults: `category` = `$3` or `SNACKS`, `product_type` = `SELLABLE`,
     `unit_of_measure` = `unit`, `ico` = `"0"`, `taxes_format` = `"percentage"`
   Capture the new product IDs from the response. If the response doesn't include
   them, re-call `list_products` and match by `sku`.

7. **Record the purchase entry — this updates stock.** `create_purchase_entry`
   with `body`:
   - `supplier_id`, `invoice_reference` = invoice number, `entry_date` = invoice date, optional `notes`
   - `items`: **all** lines (existing + newly created) as
     `{ product_id, quantity, unit_cost }`, where `unit_cost` = VALOR UNITARIO as
     a decimal string.
   This single call increases stock for every line (creating stock rows for
   brand-new products) and links each product to the supplier's catalog.

8. **Attach the source invoice.** `upload_purchase_entry_document` with the new
   purchase entry id, `file_path` = the PDF path, `file_type` = `"pdf"`.

9. **Report.** Summarize: products created, products matched, purchase entry id +
   total, stock updated, PDF attached, and any lines skipped.

## Caveats

- **Stock is integer** — received quantities are truncated to whole units
  (`2.5` → `2`). Surface fractional quantities so the user can decide.
- **Order matters** — new products must be created (step 6) *before* the purchase
  entry (step 7), which validates that every product_id exists.
- **`product_type` defaults to `SELLABLE`** (resale items with barcodes). For
  ingredients/supplies, have the user override to `INGREDIENT`.
- If a Postgres MCP is configured, a `SELECT id,name,sku FROM products WHERE sku
  IN (...)` is a more precise dedup than listing — optional.
