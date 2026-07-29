# Laguna Escondida MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes
the backend HTTP API as MCP tools, so an MCP client (e.g. Claude Code) can drive
the system conversationally.

- **Binary:** `cmd/mcp-server`
- **Adapter package:** `internal/platform/mcpserver`
- **Transport:** Streamable HTTP (`POST /mcp`)
- **Coverage:** 82 tools — one per request/response endpoint (streaming SSE,
  node-to-node sync, and edge ticket printing are intentionally excluded).

## Architecture

The MCP server is a **primary adapter that is an HTTP client of the backend
API** — it does not touch the database. It signs in with a service account,
caches the JWT, and forwards each tool call to the corresponding endpoint. Point
it at any environment with a single env var (`LAGUNA_API_URL`).

```
Claude Code ──(MCP, Streamable HTTP)──▶ mcp-server ──(HTTP + JWT)──▶ backend API
```

Because every call goes through the backend's own JWT + permission checks, **the
service account's role is your safety boundary**: give it a read-only role and
the assistant cannot mutate anything, regardless of which tools exist.

## Configuration

| Env var                | Required | Default | Description                                                         |
| ---------------------- | -------- | ------- | ------------------------------------------------------------------- |
| `LAGUNA_API_URL`       | yes      | —       | Backend base URL, e.g. `http://localhost:8080`                      |
| `LAGUNA_USERNAME`      | yes      | —       | Service-account username used to obtain a JWT                       |
| `LAGUNA_PASSWORD`      | yes      | —       | Service-account password                                            |
| `MCP_AUTH_TOKEN`       | yes      | —       | Shared secret clients send as `Authorization: Bearer <token>` to `/mcp` |
| `MCP_ADDR`             | no       | `:8090` | Listen address for the MCP HTTP endpoint                            |
| `LAGUNA_ADMIN_API_KEY` | no       | —       | Only needed by the two `update_*_document_urls` admin tools         |

Put these in your shell environment or a `.env` file next to the binary.

## Running

```bash
export LAGUNA_API_URL=http://localhost:8080
export LAGUNA_USERNAME=mcp-service
export LAGUNA_PASSWORD=…
export MCP_AUTH_TOKEN="$(openssl rand -hex 24)"   # any strong shared secret

make run-mcp        # serves http://localhost:8090/mcp  (also GET /health)
# or: make build-mcp && ./bin/mcp-server
```

## Using it in a Claude Code session

1. **Run the backend** and **run the MCP server** (above). Keep the same
   `MCP_AUTH_TOKEN` exported in the shell you launch Claude Code from.
2. The repo ships an `.mcp.json` at its root (project scope, committed):

   ```json
   {
     "mcpServers": {
       "laguna-escondida": {
         "type": "http",
         "url": "http://localhost:8090/mcp",
         "headers": { "Authorization": "Bearer ${MCP_AUTH_TOKEN}" }
       }
     }
   }
   ```

   The `${MCP_AUTH_TOKEN}` is expanded from your environment, so the secret is
   **not** committed.
3. **Restart Claude Code** (it reads `.mcp.json` at startup), then run `/mcp` and
   connect/approve `laguna-escondida`. Tools appear as
   `mcp__laguna-escondida__<tool>`.
4. (Optional) allowlist read tools in `.claude/settings.json` to skip approval
   prompts:

   ```json
   { "permissions": { "allow": ["mcp__laguna-escondida__list_*", "mcp__laguna-escondida__get_*"] } }
   ```

Verify connection status any time with `claude mcp list` or `/mcp` in-session.

## Authentication

- **Client → MCP endpoint:** shared-secret bearer token (`MCP_AUTH_TOKEN`),
  compared in constant time. `/health` is unauthenticated.
- **MCP server → backend:** JWT obtained via `POST /api/auth/signin`, cached and
  automatically refreshed on a `401`.
- **Admin tools:** `update_invoice_document_urls` and
  `update_support_document_urls` send the `X-API-Key` header
  (`LAGUNA_ADMIN_API_KEY`) instead of a JWT.

## Tool catalog (82)

- **Products (7):** `create_product`, `bulk_create_products`, `list_products`, `get_product`, `update_product`, `delete_product`, `list_product_categories`
- **Product responsibilities (4):** `create_product_responsibility`, `get_product_responsibility`, `update_product_responsibility`, `delete_product_responsibility`
- **Product ingredients (4):** `add_product_ingredient`, `list_product_ingredients`, `update_product_ingredient`, `remove_product_ingredient`
- **Orders (12):** `create_order`, `list_active_orders`, `get_order`, `update_order`, `delete_order`, `list_closed_orders_today`, `get_closed_order`, `pay_order`, `complete_order_product`, `uncomplete_order_product`, `set_order_product_in_progress`, `cancel_order_product`
- **Stock (5):** `list_stock`, `create_stock`, `adjust_stock`, `delete_stock`, `bulk_upsert_stock`
- **Suppliers (5):** `create_supplier`, `list_suppliers`, `get_supplier`, `update_supplier`, `delete_supplier`
- **Supplier catalog (5):** `add_supplier_product`, `update_supplier_product`, `remove_supplier_product`, `list_supplier_products`, `list_product_suppliers`
- **Purchase entries (6):** `create_purchase_entry`, `list_purchase_entries`, `get_purchase_entry`, `list_supplier_purchase_entries`, `upload_purchase_entry_document`, `export_purchase_entries_csv`
- **Expenses (11):** `create_expense_category`, `list_expense_categories`, `get_expense_category`, `update_expense_category`, `create_expense`, `list_expenses`, `get_expense`, `update_expense`, `delete_expense`, `upload_expense_document`, `export_expenses_csv`
- **Financial (2):** `get_financial_summary`, `get_daily_close`
- **Invoices (4):** `create_invoice`, `list_invoices`, `export_invoices_csv`, `update_invoice_document_urls`
- **Support documents (4):** `create_support_document`, `list_support_documents`, `export_support_documents_csv`, `update_support_document_urls`
- **Users & roles (8):** `create_user`, `list_users`, `get_user`, `update_user`, `reset_user_password`, `delete_user`, `list_roles`, `get_current_user`
- **Misc (5):** `backend_health`, `get_edge_status`, `get_bill_owner`, `list_pending_products_by_area`, `list_completed_products_by_area`

## Notes & known limitations

- **Tool input schemas are inferred from the backend DTOs.** Fields whose types
  have custom JSON marshaling do not reflect cleanly:
  - `create_product` / `update_product` use tailored inputs so `price` /
    `total_price_with_taxes` are decimal strings and `preparation_responsibility`
    is a plain optional object.
  - `create_invoice` / `create_support_document` reuse the full fiscal DTOs; their
    many money fields infer as opaque objects — send them as decimal
    strings/numbers.
- **Document uploads** (`upload_*_document`) read the file from a path on the MCP
  server host (`file_path`).
- **Excluded endpoints:** SSE streams (`/api/sse/...`), node-to-node sync
  (`/api/sync/...`), and edge ticket printing (`/api/device/print`).
- **Edge vs cloud:** stock writes are edge-only; invoices and support documents
  are cloud-only. Those tools return the backend's error when called in the wrong
  mode.
