# Laguna Escondida MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes
the backend HTTP API as MCP tools, so an MCP client (e.g. Claude Code) can drive
the system conversationally.

- **Binary:** `cmd/mcp-server`
- **Adapter package:** `internal/platform/mcpserver`
- **Transport:** Streamable HTTP (`POST /mcp`) for remote clients, or stdio
  (`--stdio`) for a local client that launches the binary directly (Claude Desktop)
- **Coverage:** two toolsets, chosen at startup with `MCP_TOOLSET`:
  - `full` (default): 89 staff tools — one per request/response endpoint (streaming SSE,
    node-to-node sync, and edge ticket printing are intentionally excluded), plus one
    host-local file utility (`extract_archive`) and two read-only playbook discovery tools
    (`list_playbooks`, `get_playbook` — see [Playbooks](#playbooks)).
  - `customer`: only the 8 customer tools, for the customer-facing WhatsApp agent — see
    [Customer toolset](#customer-toolset-whatsapp-agent).

## Architecture

The MCP server is a **primary adapter that is an HTTP client of the backend
API** — it does not touch the database. With the full toolset it signs in with a
service account, caches the JWT, and forwards each tool call to the corresponding
endpoint. With the customer toolset it holds no identity of its own and forwards
each caller's organization API key instead (see
[One deployment, a key per organization](#one-deployment-a-key-per-organization)).
Point it at any environment with a single env var (`LAGUNA_API_URL`).

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
| `LAGUNA_EMAIL`         | full: one of | —   | Service-account email, when `LAGUNA_API_URL` is the cloud (the cloud signs in by email, and only cloud roles such as manager or agent). Ignored by the customer toolset |
| `LAGUNA_USERNAME`      | full: one of | —   | Service-account username, when `LAGUNA_API_URL` is an edge box. Set exactly one of `LAGUNA_EMAIL` and `LAGUNA_USERNAME`. Ignored by the customer toolset |
| `LAGUNA_PASSWORD`      | full     | —       | Service-account password. Ignored by the customer toolset          |
| `MCP_AUTH_TOKEN`       | full, HTTP only | — | Shared secret clients send as `Authorization: Bearer <token>` to `/mcp`. Not needed with `--stdio` (the transport is local). Ignored by the customer toolset |
| `MCP_ADDR`             | no       | `:8090` | Listen address for the MCP HTTP endpoint                            |
| `LAGUNA_ADMIN_API_KEY` | no       | —       | Only needed by the two `update_*_document_urls` admin tools         |
| `MCP_TOOLSET`          | no       | `full`  | `full` (staff tools) or `customer` (only the customer tools; HTTP only — startup fails with `--stdio`). Any other value stops startup. |

Put these in your shell environment or a `.env` file next to the binary.

## Running

```bash
export LAGUNA_API_URL=http://localhost:8080
export LAGUNA_USERNAME=mcp-service    # an edge box; use LAGUNA_EMAIL=… for the cloud
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

## Using it in Claude Desktop (stdio)

Claude Desktop launches MCP servers as subprocesses over stdio, so point it at
the built binary with `--stdio` — no HTTP listener, no `MCP_AUTH_TOKEN`. Run the
server locally (not on ECS) so the `upload_*_document` tools can read invoice
files from paths on your own machine.

1. `make build-mcp` → `./bin/mcp-server`.
2. Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

   ```json
   {
     "mcpServers": {
       "laguna-escondida": {
         "command": "/absolute/path/to/bin/mcp-server",
         "args": ["--stdio"],
         "env": {
           "LAGUNA_API_URL": "http://localhost:8080",
           "LAGUNA_USERNAME": "mcp-service",
           "LAGUNA_PASSWORD": "…"
         }
       }
     }
   }
   ```

   Use absolute paths (Desktop's `PATH` is minimal). Point `LAGUNA_API_URL` at a
   local or deployed backend. Pair with the official `@modelcontextprotocol/server-filesystem`
   server (scoped to e.g. `~/Downloads`) so Claude can locate invoice files itself.
3. Fully quit and reopen Claude Desktop; the tools appear under the connectors icon.

## Authentication

- **Client → MCP endpoint (full toolset):** shared-secret bearer token (`MCP_AUTH_TOKEN`),
  compared in constant time. `/health` is unauthenticated.
- **MCP server → backend (full toolset):** JWT obtained via `POST /api/auth/signin`, cached and
  automatically refreshed on a `401`.
- **Customer toolset:** see [One deployment, a key per organization](#one-deployment-a-key-per-organization).
- **Admin tools:** `update_invoice_document_urls` and
  `update_support_document_urls` send the `X-API-Key` header
  (`LAGUNA_ADMIN_API_KEY`) instead of a JWT.

## Tool catalog (full toolset, 89)

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
- **Delivery zones (4, cloud only):** `list_delivery_zones`, `create_delivery_zone`, `update_delivery_zone`, `delete_delivery_zone` — `boundary` is the GeoJSON `Polygon`/`MultiPolygon` geometry passed **as a JSON string**, positions `[lng, lat]` (see [`docs/api/delivery-zones.md`](api/delivery-zones.md)). After uploading, check the restaurant's own pin with `GET /api/delivery-zones/check`.
- **Misc (5):** `backend_health`, `get_edge_status`, `get_bill_owner`, `list_pending_products_by_area`, `list_completed_products_by_area`
- **Files (1):** `extract_archive` — unzip a `.zip` on the MCP host (e.g. a DIAN invoice bundle), returning file paths and inlined XML. Host-local, not a backend call.
- **Playbooks (2):** `list_playbooks`, `get_playbook` — discover and read the server's bundled procedures. Static content embedded in the binary, not a backend call. See [Playbooks](#playbooks).

## Customer toolset (WhatsApp agent)

`MCP_TOOLSET=customer` registers **only** these tools; every staff tool (`pay_order`, `delete_order`, product writes, …) is unknown on that server:

| Tool | What it does | Backend calls |
| ---- | ------------ | ------------- |
| `list_menu` | Products for sale (sellable, composite, both — never ingredient-only): id, name, category, description, price with taxes, `has_side_dish_options` | `GET /api/products?product_type=SELLABLE,COMPOSITE,BOTH`, plus `GET /api/products/:id/side-dishes` per composite |
| `get_side_dish_options(product_id)` | A product's side-dish choices with min, max and default | `GET /api/products/:id/side-dishes` |
| `get_ordering_availability` | Whether delivery and takeaway are being taken right now: `available`, `reason` (`paused`/`outside_hours`), `paused_reason`, `closes_at`, `next_opens_at`, plus `timezone`. Needs no header; the description tells the agent to call it before taking an order. Creates nothing | `GET /api/organization/settings` (returns only the availability) |
| `check_delivery_location` | Is the shared pin deliverable, and in which zone. Creates nothing | `GET /api/delivery-zones/check` |
| `create_delivery_order(order_type, lines, customer_name?, delivery_address?)` | Creates a `takeaway` or `delivery` order (never `dine_in`) for the header phone and, for delivery, the header pin. All ids are generated by the server. Returns it as `awaiting_payment`. A paused type or one outside its hours fails with a tool error carrying the reason or the next opening | `POST /api/orders` |
| `update_my_order(order_id, lines)` | Replaces the lines of one of the caller's **unpaid** orders | ownership check, `GET /api/orders/:id`, `PUT /api/orders/:id` |
| `cancel_my_order(order_id)` | Voids one of the caller's **unpaid** orders (stock returned) | ownership check, `DELETE /api/orders/:id` |
| `get_my_orders` | The caller's unpaid orders and those paid today, newest first: payment state (`awaiting_payment`/`paid`) and each line's kitchen state. Voided orders are left out | `GET /api/orders?customer_phone=`, `GET /api/orders/closed?customer_phone=`, then each order's detail |

### Identity and location come from headers, never from the model

The agent runtime sets these headers on **every** MCP request:

| Header | Format | Source |
| ------ | ------ | ------ |
| `Authorization` | `Bearer lgk_…`, the restaurant's organization API key | Stored per organization in the agent runtime (marketing-app's tool-server `token`) |
| `X-Customer-Phone` | The sender's international number, digits only, e.g. `573005551234` (a leading `+` is accepted). Always read as international, so `34612345678` is Spain's `+34612345678` | WhatsApp verified sender (`wa_id`) |
| `X-Customer-Location` | `<latitude>,<longitude>` in decimal degrees, e.g. `4.5981,-74.076` | The location pin the customer shared in the chat; omit until one is shared |

No customer tool takes a phone or coordinates as an argument, so the model cannot be talked into acting for another number or inventing a pin inside the zone. A call that needs a header and finds it missing or invalid fails with a tool error naming the problem, **before** any backend request.

`update_my_order` and `cancel_my_order` take an order id, but first check it against `GET /api/orders?customer_phone=<header>` (the **active** list). Another customer's order, a paid order and a voided one all get the same `order not found` error, and nothing is changed.

### One deployment, a key per organization

One customer MCP deployment serves every restaurant. It has no identity of its own: it never signs in, and nothing in its configuration names an organization. Each request to `/mcp` carries the calling restaurant's **organization API key** (`Authorization: Bearer lgk_…`, see [Organization API Keys](./api/api-keys.md)), and every backend call a tool makes carries that same key and no other credential. The backend, not the MCP server, decides whether the key is valid and which organization it acts for. Never point the WhatsApp agent at the staff server.

- A request without a bearer, or with anything other than an `lgk_` key (including a staff JWT), is refused with `401` before any tool runs.
- An MCP session is bound to the key that opened it: the same session continued with another key is refused.
- A key the backend refuses (revoked, its user deactivated, its organization suspended) makes the tool fail with `credential refused`. Nothing is retried under another credential.

Each key belongs to an agent user: a cloud user with **only** the `agent` role (role id 6). It can read products and ordering availability (`ordering:read`) and create/read/update/delete orders; it gets `403` on pay, kitchen states, changing ordering settings, products, stock, zones, users, suppliers, expenses and invoices. This is the second barrier behind the toolset. The backend also keeps it to takeaway and delivery orders it created (`403 AGENT_ORDER_TYPE_NOT_ALLOWED` / `ORDER_NOT_OWNED`, see [orders](./api/orders.md#the-agent-role)), so even a leaked key cannot touch a dine-in table, a staff-taken order or another organization. Telling one customer from another still rests on the toolset's phone check, since every customer of a restaurant shares its one agent user.

#### Configuration (customer toolset)

| Env var                | Required | Default | Description                                                     |
| ---------------------- | -------- | ------- | --------------------------------------------------------------- |
| `MCP_TOOLSET`          | yes      | —       | `customer`                                                      |
| `LAGUNA_API_URL`       | yes      | —       | The **cloud** backend base URL. Edge boxes refuse every API key |
| `MCP_ADDR`             | no       | `:8090` | Listen address for `/mcp`                                       |

Over HTTP only (`--stdio` is refused: stdio has no bearer and no headers).

#### Adding a restaurant

1. In the cloud, create the restaurant's agent user with only the `agent` role.
2. Upload at least one delivery zone (full toolset or `POST /api/delivery-zones`) and check it with the restaurant's own pin. No active zone means every delivery is refused.
3. Issue its key: `POST /api/platform/organizations/:id/api-keys` with the admin API key, `{"user_id": "<agent user>", "name": "whatsapp-agent"}`. The plaintext is shown once.
4. In the marketing-app console, point the restaurant's orders tool server at the one MCP URL (`https://<mcp host>/mcp`) and set its token to the key. The agent runtime sends it as the bearer with `X-Customer-Phone` and, once shared, `X-Customer-Location`. Give the prompt the payment instructions (transfer details); staff record the payment with `pay_order` from the staff tools or the cloud page, which releases the order to the kitchen.

Revoke a leaked key with `DELETE /api/platform/organizations/:id/api-keys/:keyId` and issue a new one; deactivating every zone stops new deliveries without a deploy.

#### Rollout from the shared-secret setup

The previous customer deployment signed in as one agent account and checked one shared secret. This is a hard cutover: there is no compatibility setting.

1. Deploy the backend with the `api_keys` migration. The routes and the `lgk_` bearer path are additive; existing JWT clients are unaffected.
2. Issue Laguna's key for its `agent` user (`POST /api/platform/organizations/:id/api-keys`).
3. Deploy the MCP server with `MCP_TOOLSET=customer` and `LAGUNA_API_URL` only.
4. In the marketing-app console, point Laguna's orders tool server at the MCP URL and set its token to the key. The console's server status should show `connected`.
5. Remove the old agent account's password and shared secret from the MCP deployment's secrets.

**Rollback:** redeploy the previous MCP image with its old environment. The backend change can stay, since it is additive.

## Playbooks

A **playbook** is a versioned, step-by-step procedure for correctly performing a
multi-step task with this server's tools — its trigger conditions, ordered steps
with example request shapes, confirmation gates, and a running list of gotchas
learned in production (e.g. how to ingest a supplier invoice end to end). The
low-level tools are correct but leave the *sequence* up to the caller; a playbook
captures that sequence once so every connected client (Claude Desktop, Claude
Code, a custom agent) can read it over MCP instead of each keeping its own copy.

- **Where they live:** `internal/playbooks/*.md`. Each file is one playbook with
  YAML frontmatter (`name`, `description`) followed by a Markdown body. The
  `name` must match the filename stem (`ingest-invoice.md` → `name: ingest-invoice`).
- **Shipped with the binary:** the directory is embedded via `go:embed`, so a
  playbook and the tool behavior it documents live in the same repo, are reviewed
  in the same PRs, and can't drift apart silently. A malformed or misnamed
  playbook fails at startup, not on first request.
- **Two read-only tools expose them:**
  - `list_playbooks` → the catalog (`name` + `description` only, so listing stays
    cheap).
  - `get_playbook(name)` → one playbook's `name`, `description`, and full Markdown
    `content` (frontmatter stripped). An unknown name returns an error that lists
    the valid names, so a wrong guess self-corrects without another round trip.

**Add a playbook whenever you work out a new multi-step or domain procedure** for
this system — write it here as `internal/playbooks/<name>.md` rather than leaving
it as prose in a chat or a one-off Claude Skill, so there is exactly one place the
knowledge is edited going forward. There are no write tools; playbooks change via
normal code review.

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
- **Edge vs cloud:** stock writes (`create_stock`, `adjust_stock`, `delete_stock`,
  `bulk_upsert_stock`) are **cloud-only**, as are invoices and support documents;
  `list_stock` answers on both. Those tools return the backend's error when called in
  the wrong mode. The cloud owns on-hand and derives it from the movements it receives
  (`docs/api/stock.md`), so point the MCP server at the cloud to manage stock — an
  edge node returns `404` for every stock write.
