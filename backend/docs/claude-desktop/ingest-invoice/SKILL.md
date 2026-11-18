---
name: ingest-invoice
description: Ingest a supplier invoice (a PDF attached to the chat, or a Colombian DIAN .zip on disk) into the Laguna Escondida backend — reconcile or create products and record a purchase entry, which auto-updates stock, then attach the source document. Use when the user provides a supplier invoice/factura and wants it recorded.
---

# Ingest a supplier invoice

Turn a supplier invoice into catalog + stock updates through the
`laguna-escondida` MCP tools, with a **human confirmation gate before any write**.

In a pure Claude Desktop chat there is **no local shell and no local file
reads** — everything on the local machine happens through MCP tools. In a
Cowork/cloud session a shell (Bash) may also be available alongside the same
MCP tools reaching the user's machine:

- **Zip invoices (DIAN):** if a local shell is available, unzip it and read the
  XML directly (faster, no round trip). Otherwise call `extract_archive` (it
  runs on the local MCP host) and read the invoice from the returned
  `xml_contents`.
- **PDF-only invoices:** ask the user to **attach the PDF to this conversation**
  so you can read it, and to give you its **local path** (e.g.
  `/Users/you/Downloads/factura.pdf`) for the upload step.

Do **not** insert products or adjust stock by hand: recording the purchase entry
increases stock automatically — calling `adjust_stock`/`create_stock` too would
double-count.

**This now holds against the cloud too.** Stock used to be owned by the restaurant's
box: a purchase recorded against the cloud raised its number and the box's next stock
snapshot silently erased it. The cloud now owns on-hand and derives it from the
movements it receives, so a purchase entry recorded here increases stock **for real**
and stays increased. Recording an invoice against the cloud is the intended path.

## Invoice format (Colombian FACTURA ELECTRONICA DE VENTA)

DIAN zips contain a **PDF (rendering)** and an **XML (legal UBL 2.1 invoice)**.
**Prefer the XML** — exact structured line items and totals. DIAN often wraps the
real `Invoice` inside an `AttachedDocument`, in a `<![CDATA[...]]>` block under
`cac:Attachment`; unwrap it first.

Per line, extract: name (`cac:Item/cbc:Description`), barcode →`sku`
(`StandardItemIdentification`), `supplier_sku` (`SellersItemIdentification`),
`vat` % (per line: 0/5/19 — absent `TaxTotal` on the line/invoice means 0%),
pre-tax unit cost (`cac:Price/cbc:PriceAmount`), quantity
(`cbc:InvoicedQuantity`); plus invoice number (`cbc:ID`) and date
(`cbc:IssueDate`). Selling price: `total_price_with_taxes = unit_cost * (1 + vat/100)`.

## Steps

1. **Get the source.** If it's a `.zip`, extract it (see above) and use the XML;
   keep the PDF (or the original zip) for the attachment step. If it's a PDF,
   read the attached file and ask for its local path.
2. **Extract lines** (above). Flag rows missing a barcode.
3. **Resolve supplier** with `list_suppliers` (confirm via `get_supplier`). If
   absent, propose `create_supplier` and confirm before creating.
4. **Idempotency:** `list_supplier_purchase_entries(supplier_id)` — if an entry
   already has this invoice number (`invoice_reference`), **STOP** and report it.
5. **Match** each line against `list_supplier_products(supplier_id)` and
   `list_products` by `sku`, then `supplier_sku`, then close name. Split into
   existing vs new.
6. **Plan & confirm — REQUIRED.** Show new products (name, sku, vat, price,
   category, **unit of measure**), existing products receiving stock, and the
   purchase entry (supplier, invoice #, date, line count, total). Confirm
   `category` against `list_product_categories`. Flag low-confidence
   category/vat/unit and missing barcodes. Show the exact MCP request bodies you
   intend to send. **Wait for an explicit "yes" before any write.**
7. **Create new products:** `bulk_create_products` with `body.supplier_id` and per
   item `name`, `sku`, `supplier_sku`, `vat`, `total_price_with_taxes`; defaults
   `product_type=SELLABLE`, `unit_of_measure=unit`, `ico="0"`,
   `taxes_format="percentage"`, `category` = confirmed. `body.supplier_id`
   already links the new product into that supplier's catalog — verify with
   `list_supplier_products(supplier_id)` if useful, but a separate
   `add_supplier_product` call is redundant here (that tool is only for linking
   an *already-existing* product into a supplier's catalog after the fact).
   Capture new IDs (or re-`list_products` by sku).
8. **Record the purchase entry (updates stock):** `create_purchase_entry` with
   `supplier_id`, `invoice_reference`, `entry_date` (see caveat below), and
   `items` = **all** lines as `{product_id, quantity, unit_cost}` (`unit_cost` =
   pre-tax unit as a decimal string, in the product's chosen unit of measure).
9. **Attach the source:** `upload_purchase_entry_document(id, file_path=<pdf or
   zip local path>, file_type="pdf"|"zip")`.
10. **Report:** products created/matched, purchase entry id + total, stock updated,
    source attached, lines skipped.

## Caveats and lessons learned

- **Choose `unit_of_measure` for a new `INGREDIENT` by how recipes will consume
  it, not by how the invoice bills it.** Composite/recipe products at Laguna
  Escondida reference ingredient quantities in **grams** (e.g. "Pollo a la
  plancha" uses 180 g of chicken). An invoice billed in kg should still become
  `unit_of_measure="g"` on the product, with quantity and unit_cost converted
  accordingly (e.g. 10 kg @ $12,000/kg → `quantity="10000"`,
  `unit_cost="12"`). Ask the user which unit their recipes use before defaulting
  to whatever unit the invoice states, if it's not obvious from existing
  `INGREDIENT` products in the same category.
- **`entry_date` needs a full RFC3339 timestamp, not a bare date** —
  `"2026-09-02"` fails with a Go `time.Time` unmarshal error; send
  `"2026-09-02T00:00:00Z"`.
- **`INGREDIENT` products ignore `total_price_with_taxes`** — the API resets
  `unit_price`/`total_price_with_taxes` to `0` for `product_type=INGREDIENT`
  regardless of what's sent (there's no direct sale price; real cost is tracked
  per purchase entry via `unit_cost`). This is expected, not a bug.
- **Stock is integer** — quantities truncate (`2.5`→`2`); surface fractionals.
- **Order matters** — create products (7) before the entry (8).
- **`SELLABLE` default** — for ingredients/supplies, have the user pick `INGREDIENT`.
- **Bucket reachability** — the attachment is stored via the backend, so the MCP
  server's `LAGUNA_API_URL` must point at a backend that can reach the internal
  bucket (the deployed backend, not a local one outside that network).
- **`upload_purchase_entry_document` reads from the local MCP server host's
  filesystem.** An `"operation not permitted"` error on a connected folder
  (Downloads/Documents/Desktop all count) is a macOS Privacy & Security (TCC)
  restriction on the process running the local `laguna-escondida` MCP server —
  unrelated to the in-Claude folder-access grant, and it applies to every
  folder equally, so trying a different one won't help. Tell the user to grant
  that process Full Disk Access (or Files & Folders access) in System Settings →
  Privacy & Security, then retry the exact same call.