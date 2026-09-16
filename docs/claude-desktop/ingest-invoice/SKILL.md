---
name: ingest-invoice
description: Ingest a supplier invoice (a PDF attached to the chat, or a Colombian DIAN .zip on disk) into the Laguna Escondida backend — reconcile or create products and record a purchase entry, which auto-updates stock, then attach the source document. Use when the user provides a supplier invoice/factura and wants it recorded.
---

# Ingest a supplier invoice (Claude Desktop)

Turn a supplier invoice into catalog + stock updates through the
`laguna-escondida` MCP tools, with a **human confirmation gate before any write**.

This runs in Claude Desktop, which has **no local shell and no local file
reads** — everything on the local machine happens through MCP tools:

- **Zip invoices (DIAN):** call `extract_archive` (it runs on the local MCP host)
  and read the invoice from the returned `xml_contents`.
- **PDF-only invoices:** ask the user to **attach the PDF to this conversation**
  so you can read it, and to give you its **local path** (e.g.
  `/Users/you/Downloads/factura.pdf`) for the upload step.

Do **not** insert products or adjust stock by hand: recording the purchase entry
increases stock automatically — calling `adjust_stock`/`create_stock` too would
double-count.

## Invoice format (Colombian FACTURA ELECTRONICA DE VENTA)

DIAN zips contain a **PDF (rendering)** and an **XML (legal UBL 2.1 invoice)**.
**Prefer the XML** — exact structured line items and totals. DIAN often wraps the
real `Invoice` inside an `AttachedDocument`, in a `<![CDATA[...]]>` block under
`cac:Attachment`; unwrap it first.

Per line, extract: name (`cac:Item/cbc:Description`), barcode →`sku`
(`StandardItemIdentification`), `supplier_sku` (`SellersItemIdentification`),
`vat` % (per line: 0/5/19), pre-tax unit cost (`cac:Price/cbc:PriceAmount`),
quantity (`cbc:InvoicedQuantity`); plus invoice number (`cbc:ID`) and date
(`cbc:IssueDate`). Selling price: `total_price_with_taxes = unit_cost * (1 + vat/100)`.

## Steps

1. **Get the source.** If it's a `.zip`, call `extract_archive(archive_path=…)`
   and use `xml_contents`; keep `pdfs[0]` for the attachment. If it's a PDF, read
   the attached file and ask for its local path.
2. **Extract lines** (above). Flag rows missing a barcode.
3. **Resolve supplier** with `list_suppliers` (confirm via `get_supplier`). If
   absent, propose `create_supplier` and confirm before creating.
4. **Idempotency:** `list_supplier_purchase_entries(supplier_id)` — if an entry
   already has this invoice number (`invoice_reference`), **STOP** and report it.
5. **Match** each line against `list_supplier_products(supplier_id)` and
   `list_products` by `sku`, then `supplier_sku`, then close name. Split into
   existing vs new.
6. **Plan & confirm — REQUIRED.** Show new products (name, sku, vat, price,
   category), existing products receiving stock, and the purchase entry (supplier,
   invoice #, date, line count, total). Confirm `category` against
   `list_product_categories`. Flag low-confidence category/vat and missing
   barcodes. **Wait for an explicit "yes" before any write.**
7. **Create new products:** `bulk_create_products` with `body.supplier_id` and per
   item `name`, `sku`, `supplier_sku`, `vat`, `total_price_with_taxes`; defaults
   `product_type=SELLABLE`, `unit_of_measure=unit`, `ico="0"`,
   `taxes_format="percentage"`, `category` = confirmed. Capture new IDs (or
   re-`list_products` by sku).
8. **Record the purchase entry (updates stock):** `create_purchase_entry` with
   `supplier_id`, `invoice_reference`, `entry_date`, and `items` = **all** lines as
   `{product_id, quantity, unit_cost}` (`unit_cost` = pre-tax unit as a decimal
   string).
9. **Attach the source:** `upload_purchase_entry_document(id, file_path=<pdf or
   zip local path>, file_type="pdf"|"zip")`.
10. **Report:** products created/matched, purchase entry id + total, stock updated,
    source attached, lines skipped.

## Caveats

- **Stock is integer** — quantities truncate (`2.5`→`2`); surface fractionals.
- **Order matters** — create products (7) before the entry (8).
- **`SELLABLE` default** — for ingredients/supplies, have the user pick `INGREDIENT`.
- **Bucket reachability** — the attachment is stored via the backend, so the MCP
  server's `LAGUNA_API_URL` must point at a backend that can reach the internal
  bucket (the deployed backend, not a local one outside that network).
