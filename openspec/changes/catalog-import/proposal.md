## Why

Onboarding a restaurant is the bottleneck to adding customers. A real catalog is a graph (ingredients, plates with recipes, option slots, sellables, prep areas, opening stock), and today it can only be built one id-linked call at a time: create ingredients, read their ids, create plates, attach recipes, replace slots, then count stock. Doing it through the MCP tools from Claude Desktop works for the operator, but it takes dozens of calls, and an interruption leaves a half-built catalog with no undo.

This change is Phase 1 of an AI-guided onboarding: make the catalog importable as one checked, atomic document, so an operator (and later an in-app agent) turns a menu or spreadsheet into a working catalog safely.

## What Changes

- New endpoint that accepts a **catalog import document**: ingredients, sellables, composite plates with recipes and option slots, preparation areas and an optional opening stock count. Entries reference each other by a local `ref`, never by id.
- A **dry run** mode that validates the whole document and returns every problem found, without writing anything.
- An **apply** mode that writes the whole document in one transaction, or nothing.
- **Re-import rule:** a document only creates products. A ref can point at a product that already exists (by name or SKU), but an import never edits an existing product. A new entry whose name or SKU collides with an existing product is reported as a conflict.
- Missing SKUs are generated with the rule from `sku-suggestion` (for example `COC-BAN-PAI`), which this change depends on. Tax, unit and prep-area defaults can be set once per document.
- Opening stock is recorded as count movements, the same way the cloud records any other count.
- Two new MCP tools, `validate_catalog_import` and `apply_catalog_import`, in the full toolset only, plus an `onboard-catalog` playbook that guides the operator from menu or spreadsheet through dry run to apply.
- Fix the MCP stock tool descriptions, which say "edge mode only" while stock writes are now cloud-only.

Out of scope (later changes): the in-app ADK-Go onboarding agent and review grid with a persisted draft (Phase 2); invoice-driven purchase entries matched to the catalog; reading menus or spreadsheets in the backend (the operator's AI client reads them).

## Capabilities

### New Capabilities

- `catalog-import`: validating and applying a catalog import document atomically, with ref resolution, the create-and-reference rule, generated SKUs, defaults, opening stock and the MCP tools that expose it.

### Modified Capabilities

None. Opening stock reuses the count behavior of `cloud-stock-ownership` without changing its requirements.

## Impact

- Backend: new DTOs, service, handler and cloud route for the import; reuse of the product, ingredient, option-slot, responsibility and stock-count write paths inside one unit of work. No new tables.
- MCP server: two new tools in the full toolset, a new embedded playbook, corrected stock tool descriptions.
- Docs: new `backend/docs/api/catalog-import.md` and a curl example.
- Frontend: none in this phase.
- **Trust boundary: yes.** It adds an authenticated write endpoint behind roles and tenant (organization) scoping, a large untrusted request body produced by an LLM, and new MCP tools, one of which can read a file on the MCP host.
