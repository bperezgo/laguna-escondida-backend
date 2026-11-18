## Why

Guests ask for tables on WhatsApp, often after a Meta or TikTok ad, but nothing records the request, the guest, the ad, or how the visit went. A small in-house CRM lets the WhatsApp agent take reservations, staff confirm them, and closes the loop by asking for feedback.

## What Changes

- **Contacts**: one per organization and phone, keeping the first ad touch that brought them.
- **Reservations**: name, party size, start time, notes, source, attribution. Lifecycle `requested → confirmed → visited`, or `declined`, `cancelled`, `no_show`. v1 records and staff confirm; no tables, slots or capacity checks.
- **Staff management**: list, confirm, decline, cancel and mark no-show, via the cloud API and staff MCP tools.
- **Customer MCP tools**: `create_my_reservation`, `get_my_reservations`, `cancel_my_reservation`, `record_my_feedback`, scoped to the `X-Customer-Phone` header like the order tools.
- **Ad attribution replicated from the agent runtime**: `X-Conversation-Id` and `X-Attribution-*` headers on customer MCP calls are stored on reservations and agent-created orders. The model never sees or sets them. The click identifier is not stored here.
- **Feedback**: a confirmed reservation counts as visited two hours after its start unless marked no-show; a cron then asks for feedback the next day. `record_my_feedback` stores a 1–5 rating and comment and flags 1–3 for manager escalation. Sharing the Google review link is an organization switch, off by default; when on, every guest gets it, any rating. No incentives, no review gating.
- **Signed outbound events to marketing-app**: a transactional outbox, HMAC-signed, retried until acknowledged.
  - `order.paid`, `order.cancelled` → `/backend/events` (the contract marketing-app's ad-conversions already expects).
  - `reservation.confirmed`, `.declined`, `.cancelled`, `feedback.requested` → new `/backend/notifications`.
- **Settings**: a `feedback` section in the organization settings (review URL, review-link switch); the events URL and per-organization signing secret come from the environment.

## Capabilities

### New Capabilities
- `reservations`: contacts, the reservation lifecycle, staff management and the visit rule.
- `guest-feedback`: the feedback request schedule, recording ratings, the optional review link and low-score escalation.
- `marketing-events`: the signed outbox and the event contract delivered to marketing-app.
- `ad-attribution`: accepting and storing campaign attribution from the agent runtime on reservations, orders and contacts.

### Modified Capabilities
- `customer-agent-access`: the customer toolset gains the reservation and feedback tools, and the agent role gains reservation permissions.

## Non-goals

- Tables, time slots, capacity or availability checks.
- Sending WhatsApp messages from the backend; marketing-app owns the number and the templates.
- Resolving ad ids to campaign names, or TikTok attribution detection (both marketing-app).
- Campaigns, segments or broadcasts.

## Impact

- **Code**: new domain and adapters for contacts, reservations, feedback and the outbox; new cron jobs; `pay_order` and void write outbox rows in their transactions; new MCP tools.
- **Data**: new cloud-only tables; organization settings columns.
- **APIs**: new reservation endpoints, `docs/api/reservations.md`, `docs/mcp-server.md`.
- **Dependency — marketing-app follow-up (out of repo)**: send the attribution headers, add `/backend/notifications` and WhatsApp template sending, and give the agent the reservation and feedback tools. This change defines the contract.
