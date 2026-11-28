---
name: onboard-catalog
description: Build a new restaurant's catalog (ingredients, sellables, plates with recipes and option slots, preparation areas, opening stock) from a menu photo, PDF or spreadsheet, as one validated import applied atomically. Use when an operator wants to load or onboard a menu or catalog through the laguna-escondida MCP tools.
---

# Onboard a restaurant's catalog

Turn a menu or spreadsheet into a working catalog with **one** document, checked
with `validate_catalog_import` and written with `apply_catalog_import`. Never build
the catalog with `create_product`, `add_product_ingredient` and
`set_plate_option_slots` one call at a time: an interruption would leave half a
catalog, and the import does it all at once or not at all.

The tools work **only against the cloud** and **only for an admin** user. Read
[`docs/api/catalog-import.md`](../../docs/api/catalog-import.md) for the full
document format and every problem code.

## 1. Read the source

- Read the menu (photo or PDF attached to the chat) or the spreadsheet. List every
  item the restaurant **sells** and every ingredient it **buys and counts**.
- Decide each item's kind:
  - `sellables`: sold as is, no recipe (a soda, a beer, a bottle of water).
  - `plates`: made in the kitchen from ingredients (a fish plate, a bandeja).
  - `ingredients`: bought and used, not sold (rice, oil, raw fish). Use
    `product_type: "BOTH"` for something both sold and used (patacón sold as a
    side and served inside a plate). An `INGREDIENT` takes no price, taxes or prep.
- Call `list_products` first. If the organization already has products, **do not
  recreate them**: list each one you need under `existing` (by `sku` when you have
  it, otherwise by `name`) and link to its ref. An import never edits an existing
  product. A new entry with an existing product's name is refused as `name_conflict`.

## 2. Propose defaults and get them confirmed

Propose a `defaults` block and **ask the operator to confirm or correct it before
going on**. The usual values for a Colombian restaurant are:

```json
{ "vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0 }
```

Ask which items go to another preparation area (drinks to `bar`, for example) and
set `prep: { "area": "bar" }` on those entries. The server never guesses: a value
missing from both the entry and `defaults` is reported as `missing_field`.

## 3. Ask for recipes and options, plate by plate

For **each plate**, ask the operator, one plate at a time:

1. **Recipe:** which ingredients go into one portion, and how much of each.
2. **Options:** does the customer choose something (the rice, the side, the
   salad)? Each choice is an option **slot** with **exactly one default** choice.
   A choice may have no items ("Sin arroz").

Keep a choosable part **out of the recipe**: it goes in the slot, or it would be
deducted twice.

**Normalize quantities.** Write recipe and choice quantities in the ingredient's
unit, as `g`, `ml` or `unit`: "media libra de arroz" becomes `250` g, "un vaso"
becomes a number of `ml`. Choice quantities must be **whole numbers**. A recipe
quantity may be a fraction (`0.5` of a lemon).

Don't invent recipes. If the operator does not know one yet, import the plate
without a recipe and say so in the summary.

## 4. Build the document

- Give every entry a short, unique `ref` (`arroz-blanco`, `tilapia`) and link only
  by ref, never by product id.
- Leave `sku` out unless the operator has its own codes. The server generates a
  readable one (`COC-BAN-PAI`) and lists it in the summary.
- Opening stock is optional. Add `opening_stock` only for products **this import
  creates**, as whole numbers in the product's unit, and only if the operator
  counted them.
- For a large menu, when running on Claude Desktop (stdio), you may write the
  document to a JSON file and pass `file_path` instead of `document`. Over the
  HTTP transport `file_path` is refused: pass the document inline.

## 5. Validate and fix

Call `validate_catalog_import`. It writes nothing. If `valid` is false, fix
**every** problem (each has a JSON-pointer `path` and a `code`) and validate
again. Typical fixes:

| Code                 | Fix                                                                     |
| -------------------- | ----------------------------------------------------------------------- |
| `name_conflict`      | Rename the entry, or move it to `existing` and link to it               |
| `unknown_ref`        | Define the ingredient, or fix the ref's spelling                        |
| `invalid_link_type`  | A recipe or choice can only use an ingredient, never a plate or a sellable |
| `slot_default_count` | Mark exactly one choice of the slot as `default: true`                  |
| `invalid_quantity`   | Whole positive numbers in g, ml or units for choices                    |
| `missing_field`      | Set the named field on the entry or in `defaults`                       |
| `existing_ambiguous` | Name the existing product by `sku` instead of by name                   |

Ask the operator when a fix needs a decision (a different name, a missing
price). Never fix a price or a tax on your own.

## 6. Show the summary and wait for approval

When the document is valid, show the operator, in plain words:

- how many ingredients, sellables and plates will be created, and the recipe
  lines, option slots and opening counts;
- the defaults that were applied;
- the generated SKUs;
- every plate created without a recipe.

Then **stop and wait for the operator's explicit approval** ("sí, cárgalo"). Do
not apply on a vague or implied answer.

## 7. Apply with the digest

Call `apply_catalog_import` with **the same document** and the `digest` the last
validation returned, as `expected_digest`.

- `digest_mismatch`: the document changed after the operator saw the summary.
  Validate again, show the new summary and ask again.
- Problems on apply (for example a product created by someone else in the
  meantime): fix them, validate, show the summary and ask again.
- Success returns each ref's new product `id` and `sku`. Nothing is half-written
  on any failure, so a retry is always safe after a fresh validation.

## 8. Opening stock later

If the operator counts stock after the import, record those counts from the
stock screen or with `bulk_upsert_stock`. Don't re-import to change anything:
an import only creates. Change existing products with `update_product`,
`set_plate_option_slots` and the ingredient tools.
