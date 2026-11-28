## Purpose

Lets an organization build its catalog (ingredients, sellables, plates with recipes and option slots, preparation areas and opening stock) from one document that is checked as a whole and then applied all at once, so onboarding a restaurant never leaves a half-built catalog.

## ADDED Requirements

### Requirement: Catalog import document

The system SHALL accept a catalog import document made of optional `defaults`, `existing`, `ingredients`, `sellables`, `plates` and `opening_stock` sections. Every entry that defines or references a product SHALL carry a `ref` unique within the document, and entries SHALL refer to each other only by `ref`, never by product id. Ingredients SHALL be of type `INGREDIENT` or `BOTH`. Sellables SHALL NOT carry a recipe or option slots. Plates SHALL be created as `COMPOSITE` and MAY carry a recipe and option slots. A document with unknown fields SHALL be rejected. The order of sections and of entries within a section SHALL NOT affect validation or the result of apply: an entry MAY reference a ref defined later in the document.

#### Scenario: Plate references an ingredient by ref
- **WHEN** a document defines ingredient `arroz` and a plate whose recipe lists `{ "ingredient": "arroz", "quantity": 200 }`
- **THEN** validation reports no problem for that recipe line

#### Scenario: Plate defined before its ingredient
- **WHEN** a document lists its `plates` section before its `ingredients` section, and a plate's recipe names the ref `frijol` of an ingredient that appears after it
- **THEN** validation reports no problem for that recipe line, and apply links the recipe line to the created ingredient

#### Scenario: Duplicate ref
- **WHEN** two entries in the document use the ref `papa`
- **THEN** validation reports a `duplicate_ref` problem with the path of the second entry

#### Scenario: Unknown ref
- **WHEN** a recipe line names the ref `yuca` and no entry or `existing` item defines it
- **THEN** validation reports an `unknown_ref` problem with the path of that recipe line

#### Scenario: Unknown field
- **WHEN** a document carries a field the format does not define, such as `"product_id"` on a recipe line
- **THEN** the request is rejected as malformed and nothing is validated or written

### Requirement: Dry-run validation

The system SHALL provide a validation endpoint that checks a whole document without writing anything. It SHALL return whether the document is valid, every problem found (not only the first), each with a JSON-pointer `path`, a stable `code` and a message, a summary with the count of products, recipe lines, option slots, preparation areas and opening-stock entries the document would create, the SKUs it would generate, and a `digest` of the document.

#### Scenario: Several problems reported at once
- **WHEN** a document has an unknown ref in one plate and a slot without a default in another
- **THEN** the response lists both problems with their paths, and no product, recipe, slot or stock movement exists afterwards

#### Scenario: Valid document
- **WHEN** a document has no problem
- **THEN** the response is valid, carries the summary and a digest, and the catalog is unchanged

### Requirement: Atomic apply

The system SHALL provide an apply endpoint, separate from the validation endpoint, that takes the same document plus `expected_digest`. It SHALL validate the document again and, only if no problem is found and the digest matches, write every product, recipe line, option slot, preparation responsibility and opening-stock movement in one transaction. If any problem is found or any write fails, it SHALL write nothing. It SHALL return the ids and SKUs of the created products keyed by ref.

#### Scenario: Successful apply
- **WHEN** a valid document with 3 ingredients, 2 sellables and 1 plate with a recipe and one slot is applied with the digest returned by validation
- **THEN** the 6 products, the recipe, the slot and its choices exist, and the response maps each ref to its product id and SKU

#### Scenario: Apply with a problem writes nothing
- **WHEN** a document whose last plate has a slot with two defaults is applied
- **THEN** the response reports `slot_default_count` and none of the document's products exist

#### Scenario: Digest mismatch
- **WHEN** a document is validated, a price is then changed, and the changed document is applied with the original digest
- **THEN** the apply is refused with `digest_mismatch` and nothing is written

#### Scenario: Repeated apply does not duplicate
- **WHEN** the same valid document is applied twice
- **THEN** the second apply reports a `name_conflict` for each product and writes nothing

### Requirement: Create and reference, never edit

An import SHALL only create products. A ref SHALL point at an existing product of the caller's organization only through an `existing` entry that names it by exact SKU or by name compared trimmed and case-insensitively. An `existing` entry that matches no product SHALL be reported as `existing_not_found`. A new entry whose name or SKU matches an existing product of the organization, or another new entry of the document, SHALL be reported as `name_conflict` or `sku_conflict`. An import SHALL NOT change any field, recipe, option slot, preparation responsibility or stock of an existing product.

#### Scenario: Plate reuses an existing ingredient
- **WHEN** the organization has the product "Limón" and a document lists `existing: [{ "ref": "limon", "name": "limón " }]` and a plate whose recipe uses `limon`
- **THEN** the plate's recipe line links to the existing "Limón", and "Limón" itself is unchanged

#### Scenario: New entry collides with an existing product
- **WHEN** the organization has "Coca-Cola 400ml" and a document defines a new sellable named "coca-cola 400ml"
- **THEN** validation reports `name_conflict` with the path of that sellable

#### Scenario: Another organization's product cannot be referenced
- **WHEN** an admin of organization A applies a document whose `existing` entry names the SKU of a product that exists only in organization B
- **THEN** validation reports `existing_not_found`, nothing is written, and nothing in the response reveals organization B's product

### Requirement: Product links follow product types

Every recipe line and option-choice item SHALL resolve to a product of type `INGREDIENT` or `BOTH`. Recipe quantities SHALL be positive. Option-choice item quantities SHALL be positive integers. Every option slot SHALL have a name, at least one choice, and exactly one default choice. A choice with no items SHALL be allowed (the plate served without that slot).

#### Scenario: Plate inside a plate
- **WHEN** a plate's recipe names the ref of another plate
- **THEN** validation reports `invalid_link_type` with the path of that recipe line

#### Scenario: Fractional choice quantity
- **WHEN** an option-choice item has quantity `0.5`
- **THEN** validation reports `invalid_quantity` with the path of that item

#### Scenario: Slot without a default
- **WHEN** an option slot has two choices and neither is the default
- **THEN** validation reports `slot_default_count` with the path of that slot

### Requirement: Defaults and generated SKUs

Values left out of an entry for VAT, ICO, tax format, unit of measure and preparation area and priority SHALL be taken from the document's `defaults`. A value missing from both SHALL be a validation problem naming the field. An entry without a SKU SHALL get one generated with the SKU generation rule of the `product-sku` capability, from its name, type and preparation area, unique among the organization's products and the document's entries. A provided SKU SHALL follow the SKU format of the `product-sku` capability.

#### Scenario: Default tax applied
- **WHEN** `defaults` sets ICO 8 and a sellable leaves ICO out
- **THEN** the created sellable has ICO 8

#### Scenario: Required value missing everywhere
- **WHEN** neither `defaults` nor an ingredient sets `unit_of_measure`
- **THEN** validation reports a problem naming `unit_of_measure` with that ingredient's path

#### Scenario: Generated SKU avoids collisions
- **WHEN** the organization has a product with SKU `ING-ARR-INT` and a document adds new ingredients "Arroz integral" and "Arroz de coco" without SKUs or prep area
- **THEN** they get `ING-ARR-INT-2` and `ING-ARR-COC`, and both appear in the validation summary

### Requirement: Opening stock for imported products

`opening_stock` entries SHALL name refs of products the same document creates, with a non-negative counted amount in the product's unit. Each entry SHALL be recorded on apply as a stock count movement, the same way the cloud records any count, so on-hand equals the counted amount. An entry naming an `existing` ref SHALL be reported as `stock_on_existing`.

#### Scenario: Opening count recorded
- **WHEN** a document creates ingredient `arroz` (unit g) with `opening_stock: [{ "product": "arroz", "counted": 25000 }]` and is applied
- **THEN** arroz's on-hand is 25000 and a count movement of +25000 is recorded for it

#### Scenario: Count on an existing product refused
- **WHEN** `opening_stock` names a ref that comes from `existing`
- **THEN** validation reports `stock_on_existing` and the existing product's stock is unchanged

### Requirement: Who may import

Both import endpoints SHALL require an authenticated user holding the `catalog:import` permission, which only the admin role holds. They SHALL act only on the caller's organization, taken from the authenticated session and never from the document. They SHALL be served only by a node running in cloud mode.

#### Scenario: Unauthenticated caller
- **WHEN** either endpoint is called without a valid token
- **THEN** the response is HTTP 401 and nothing is read or written

#### Scenario: Role without the permission
- **WHEN** a manager, cook or `agent` user calls either endpoint
- **THEN** the response is HTTP 403 and nothing is written

#### Scenario: Edge node
- **WHEN** either endpoint is called on a node running in edge mode
- **THEN** the response is HTTP 404

### Requirement: Size limits

The import endpoints SHALL refuse a request body larger than 2 MiB with HTTP 413. Validation SHALL report `limit_exceeded` when a document has more than 1,000 products in total, a plate has more than 50 recipe lines or 20 option slots, a slot has more than 30 choices, a choice has more than 50 items, or there are more than 1,000 opening-stock entries.

#### Scenario: Oversized body
- **WHEN** a 3 MiB body is sent to either endpoint
- **THEN** the response is HTTP 413 and nothing is written

#### Scenario: Too many products
- **WHEN** a document of 1,001 products is validated
- **THEN** validation reports `limit_exceeded` and nothing is written

### Requirement: MCP import tools

The MCP server SHALL register `validate_catalog_import` and `apply_catalog_import` in the full toolset only. They SHALL forward to the import endpoints and return the response unchanged. `apply_catalog_import` SHALL require `expected_digest`. Each tool SHALL take either an inline `document` or a `file_path` to a JSON file on the MCP host. A `file_path` SHALL be accepted only when the server runs on the local stdio transport. Over HTTP it SHALL fail with a tool error before any file is read. A file larger than 2 MiB SHALL fail with a tool error.

#### Scenario: Customer toolset
- **WHEN** the MCP server starts with the customer toolset
- **THEN** neither import tool is registered

#### Scenario: File path over HTTP
- **WHEN** a caller of the HTTP transport calls `validate_catalog_import` with `file_path` `/etc/passwd`
- **THEN** the tool fails with an error saying file paths are only accepted on stdio, no file is read, and the backend is not called

#### Scenario: File path over stdio
- **WHEN** the server runs on stdio and `validate_catalog_import` gets the path of a valid 500 KiB JSON document
- **THEN** the tool returns the validation response for that document

### Requirement: Onboarding playbook

The MCP server SHALL serve an `onboard-catalog` playbook that guides an operator's AI client from a menu or spreadsheet to an applied import. It SHALL instruct the client to propose `defaults` and get the operator's confirmation, to ask for recipes and option slots plate by plate, to normalize quantities to g, ml or units, to validate and fix problems before applying, to show the summary and wait for the operator's explicit approval, and to apply with the digest returned by validation. The MCP stock tool descriptions SHALL state that stock writes are available only in cloud mode.

#### Scenario: Playbook listed
- **WHEN** a client calls `list_playbooks`
- **THEN** `onboard-catalog` is listed with its description, and `get_playbook` returns its instructions

#### Scenario: Stock tool descriptions
- **WHEN** a client lists the tools of the full toolset
- **THEN** the stock write tools describe themselves as cloud-only, and none says "edge mode only"
