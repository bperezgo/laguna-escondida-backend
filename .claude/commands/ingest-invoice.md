---
description: Ingest a supplier invoice (PDF or DIAN zip) → reconcile/create products and record a purchase entry (which auto-updates stock), then attach the source. Colombian FACTURA format.
argument-hint: <invoice.pdf | invoice.zip> [supplier name or id] [default category]
allowed-tools: Read, mcp__laguna-escondida__extract_archive, mcp__laguna-escondida__list_suppliers, mcp__laguna-escondida__get_supplier, mcp__laguna-escondida__create_supplier, mcp__laguna-escondida__list_products, mcp__laguna-escondida__list_product_categories, mcp__laguna-escondida__list_supplier_products, mcp__laguna-escondida__list_supplier_purchase_entries, mcp__laguna-escondida__bulk_create_products, mcp__laguna-escondida__create_purchase_entry, mcp__laguna-escondida__upload_purchase_entry_document
---

# Ingest a supplier invoice

Turn a supplier invoice (a PDF, or a Colombian DIAN **zip** bundle) into catalog +
stock updates through the `laguna-escondida` MCP tools, with a **human
confirmation gate before any write**.

Do **not** insert products or adjust stock by hand: recording the purchase entry
(step 8) increases stock automatically — calling `adjust_stock`/`create_stock`
as well would double-count.

**This now holds against the cloud too.** Stock used to be owned by the restaurant's
box: a purchase recorded against the cloud raised its number and the box's next stock
snapshot silently erased it. The cloud now owns on-hand and derives it from the
movements it receives, so a purchase entry recorded here increases stock **for real**
and stays increased. Recording an invoice against the cloud is the intended path.

## Inputs

- **Invoice source:** `$1` (path to a `.pdf` or `.zip`). If `$ARGUMENTS` is
  empty, ask the user to provide/attach it.
- **Supplier:** `$2` (name or UUID). Resolve a name with `list_suppliers`.
- **Default category:** `$3`. Confirm against `list_product_categories`; ask if
  unsure rather than guessing.

## Invoice format (Colombian FACTURA ELECTRONICA DE VENTA)

DIAN invoices are distributed as a **zip containing both a PDF (human-readable)
and an XML (the legal UBL 2.1 electronic invoice)**. **Prefer the XML** — it has
exact, structured line items and totals; the PDF is a rendering and only a
fallback when no XML is present.

Extract each product row:

| Invoice field (PDF column / XML element)                     | Maps to |
|---|---|
| DESCRIPCION / `cac:Item/cbc:Description`                     | product `name` (keep as-is / uppercase) |
| COD BARRAS / `cac:Item/cac:StandardItemIdentification`       | `sku` (barcode) |
| CODIGO / `cac:Item/cac:SellersItemIdentification/cbc:ID`     | `supplier_sku` |
| IVA (%) / `cac:TaxTotal/cac:TaxSubtotal` percent             | `vat` — per line; may be 0 / 5 / 19 |
| VALOR UNITARIO / `cac:Price/cbc:PriceAmount`                 | pre-tax unit cost → `unit_cost` on the entry, and the price basis |
| CANT/UNID / `cbc:InvoicedQuantity`                           | `quantity` on the purchase entry |
| Invoice number / `cbc:ID`                                    | `invoice_reference` |
| Invoice date / `cbc:IssueDate`                               | `entry_date` |

Note: DIAN often wraps the real invoice inside an `AttachedDocument` — the UBL
`Invoice` lives in a `<![CDATA[...]]>` block under `cac:Attachment`. Unwrap it
before reading the line items.

Product selling price: `total_price_with_taxes = VALOR UNITARIO * (1 + IVA/100)`
(e.g. 5462 @ 19% → `"6499.78"`).

## Steps

1. **Unpack if needed.** If `$1` ends in `.zip`, call `extract_archive` with
   `archive_path=$1`. Use the returned `xml_contents` as the primary data source;
   note the `pdfs[0]` path for the attachment in step 9. If `$1` is a `.pdf`,
   `Read` it directly.

2. **Read & extract.** Pull each line: name, barcode (sku), supplier_sku, vat,
   unit cost, quantity — plus the **invoice number** and **invoice date**. Flag
   any row missing a barcode.

3. **Resolve the supplier.** `list_suppliers`, match `$2` by name or id (use
   `get_supplier` to confirm). If it doesn't exist, propose `create_supplier`
   and confirm before creating.

4. **Idempotency check.** `list_supplier_purchase_entries(supplier_id)`. If an
   entry already exists with this invoice number (`invoice_reference`), **STOP**
   and report it — do not ingest the same invoice twice.

5. **Match to existing products.** Call `list_supplier_products(supplier_id)`
   and `list_products`. Match each line to an existing product by `sku`
   (barcode) first, then `supplier_sku`, then a close name match. Split the lines
   into **existing** (already have a product_id) and **new**.

6. **Plan & confirm — REQUIRED, do not skip.** Present a review:
   - **New products** to create: name, sku, vat, computed price, category, and the applied defaults.
   - **Existing products** that will receive stock (show current stock if available).
   - **Purchase entry** to record: supplier, invoice #, date, line count, total.
   Call out low-confidence fields (**category**, **vat**) and any missing
   barcodes. **Wait for an explicit "yes" before any write tool call.**

7. **Create the new products.** `bulk_create_products` with `body.supplier_id` =
   the supplier and, per item:
   - `name`, `sku` (barcode), `supplier_sku`, `vat` (per line), `total_price_with_taxes` (computed)
   - defaults: `category` = `$3` (confirmed), `product_type` = `SELLABLE`,
     `unit_of_measure` = `unit`, `ico` = `"0"`, `taxes_format` = `"percentage"`
   Capture the new product IDs from the response. If the response doesn't include
   them, re-call `list_products` and match by `sku`.

8. **Record the purchase entry — this updates stock.** `create_purchase_entry`
   with `body`:
   - `supplier_id`, `invoice_reference` = invoice number, `entry_date` = invoice date, optional `notes`
   - `items`: **all** lines (existing + newly created) as
     `{ product_id, quantity, unit_cost }`, where `unit_cost` = VALOR UNITARIO as
     a decimal string.
   This single call increases stock for every line (creating stock rows for
   brand-new products) and links each product to the supplier's catalog.

9. **Attach the source invoice.** `upload_purchase_entry_document` with the new
   purchase entry id, `file_path` = the PDF path (from step 1) or the original
   zip, `file_type` = `"pdf"` or `"zip"`.

10. **Report.** Summarize: products created, products matched, purchase entry id +
    total, stock updated, source attached, and any lines skipped.

## Caveats

- **Stock is integer** — received quantities are truncated to whole units
  (`2.5` → `2`). Surface fractional quantities so the user can decide.
- **Order matters** — new products must be created (step 7) *before* the purchase
  entry (step 8), which validates that every product_id exists.
- **`product_type` defaults to `SELLABLE`** (resale items with barcodes). For
  ingredients/supplies, have the user override to `INGREDIENT`.
- **Bucket reachability** — the attachment lands in the internal bucket via the
  backend, so `LAGUNA_API_URL` must point at a backend that can reach it (the
  deployed backend, not a local one isolated from the bucket network).
