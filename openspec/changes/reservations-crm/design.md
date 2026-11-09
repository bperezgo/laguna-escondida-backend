## Context

See `proposal.md` for the motivation. The facts below shape the approach.

- **The WhatsApp agent lives in `../marketing-app/whatsapp-agent`.** It reaches this backend only through the customer MCP toolset (`internal/platform/mcpserver`). The toolset forwards the restaurant's organization API key and takes the customer's phone only from `X-Customer-Phone`, set by the runtime from WhatsApp's verified sender.
- **marketing-app already accepts signed events from this backend** at `POST /backend/events` (`whatsapp-agent/internal/service/backend_events.go`):
  - Signature: `X-Signature-256: sha256=<hex HMAC-SHA256 of the raw body>`, keyed by a secret configured **per WhatsApp channel**. The receiver tries each channel's secret and then requires the body's `channel_id` to equal the channel that matched.
  - Body: `{event_id, type, occurred_at, channel_id, order_id, customer_phone, amount, currency}`, types `order.paid` and `order.cancelled`.
  - Responses: `200` = done or deliberately ignored (including orders the agent did not place), `400` = malformed, `401` = bad signature, `5xx` = retry.
  - The backend side was never built. Nothing in this repo sends events today.
- **marketing-app stores the ad click** (`ad_click`: `conversation_id`, `ctwa_clid`, ad id as `source_id`, `source_type`, `clicked_at`) and its spec forbids giving the click or the ad to the model in any form.
- **Existing building blocks in this repo:**
  - `ports.UnitOfWork` wraps `PayOrder` and `DeleteOrder`.
  - `internal/platform/cron.Scheduler` runs jobs once per organization through `OrganizationRunner`.
  - Organization settings are a JSON document behind `GET/PATCH /api/organization/settings`, with section-level permissions.
  - Organizations have a `timezone`.
- **The `organization-ordering-availability` change is complete but not archived.** It modifies the same two `customer-agent-access` requirements this change modifies. This change's deltas are written against its versions, so **it must be archived first**.

## Goals / Non-Goals

**Goals:**
- One delivery mechanism (an outbox) for every event this backend sends to marketing-app, so `order.*` and `reservation.*` share signing, retries and idempotency.
- Attribution and channel identity that the model cannot see or forge, the same way the phone works today.
- A feedback loop that is policy-safe by construction, so no code path can send the review link selectively.

**Non-Goals:**
- Delivering events anywhere except marketing-app. There's no generic webhook subscription API.
- Staff UI. This change exposes the API and the staff MCP tools only.
- Contact merge or dedupe across phones. A contact is exactly one phone.
- Trimming the new tables from the edge database. Their endpoints are cloud-only; the edge trim can drop them in a later change.

## Decisions

### D1: Contact = (organization, E.164 phone), created as a side effect

The first reservation or agent-created order for a phone creates the contact, and later ones update `last_seen_at`. `name` keeps the latest non-empty value given. `first_touch` (attribution) is written once and never overwritten.

*Alternative: reuse the `customer` aggregate.* Rejected. That aggregate is the DIAN invoicing customer, keyed by document number with a required email. A WhatsApp guest has neither.

### D2: Reservation status is stored, and "visited" is set by a job

```
            staff confirm             starts_at + 2h (job)
 requested -------------> confirmed ------------------------> visited
     |  \                   |   \                              |
     |   \ staff decline    |    \ staff no_show               | staff no_show,
     |    v                 |     v                            | until feedback is requested
     |   declined           |    no_show <---------------------+
     v                      v
 cancelled (customer/staff) cancelled (customer/staff)
```

A cloud job runs every 15 minutes and moves `confirmed` reservations whose `starts_at + 2h` has passed to `visited`. Staff can still turn `visited` into `no_show` until the feedback request goes out, which gives them the night to correct it.

*Alternative: derive "visited" at read time.* Rejected. The feedback job, the staff list and the events all need the same answer, and a stored state is simpler to query and to test.

### D3: The model never handles attribution or channel; the runtime sends headers

The customer MCP tools read these headers, as they already read the phone:

| Header | Meaning |
|---|---|
| `X-Channel-Id` | marketing-app channel the customer wrote on; routes the events back |
| `X-Conversation-Id` | marketing-app conversation; the join key across both repos |
| `X-Attribution-Source` | e.g. `meta_ctwa`; open-ended, so `tiktok` fits later |
| `X-Attribution-Ad-Id` | Meta ad id (`source_id`) |
| `X-Attribution-Clicked-At` | RFC 3339 click time |
| `X-Attribution-Campaign-Id`, `X-Attribution-Campaign-Name` | optional, when marketing-app has resolved them |

The MCP server passes these to the backend as an `origin` object on the create endpoints. The backend accepts `origin` from any caller allowed to create, since staff-set values are harmless. Malformed attribution values are dropped and logged, and **never fail the booking**. `X-Channel-Id` is required by `create_my_reservation` (without it no confirmation could reach the guest), but optional on `create_delivery_order`, so today's order flow keeps working before marketing-app ships.

`ctwa_clid` is never sent or stored here. Only marketing-app reports to Meta, and that's where its retention policy runs.

*Alternative: tool arguments.* Rejected. The model could forge them, and marketing-app's spec forbids showing the ad to the model.

### D4: A transactional outbox and one sender, with two paths

There's a new table, `marketing_event_outbox` (`organization_id`, `event_id`, `path`, `body` bytes, `status`, `attempts`, `next_attempt_at`, `last_error`, `created_at`, `sent_at`). Rows are inserted inside the same `UnitOfWork` as the state change: `PayOrder`, `DeleteOrder`, reservation transitions, and the feedback-request job. A cloud worker claims due rows with `FOR UPDATE SKIP LOCKED`, signs the **stored bytes**, and POSTs them to `MARKETING_APP_EVENTS_URL + path`.

| Response | Action |
|---|---|
| `2xx` | `sent` |
| `400` | `rejected`, logged at error level, not retried |
| `401` | retry slowly (hourly), logged at error level; it's a configuration fault |
| `5xx`, timeout | exponential backoff from 10s, capped at 30 min |
| still unsent after 72h | `expired`, logged at error level |

Events are written only when the subject carries a `channel_id`. Staff-created orders and reservations never reached marketing-app, so it couldn't act on them. When `MARKETING_APP_EVENTS_URL` is unset, nothing is written.

Paths:
- `order.paid`, `order.cancelled` → `/backend/events`, keeping the existing flat body plus `customer_phone`.
- `reservation.confirmed`, `reservation.declined`, `reservation.cancelled`, `feedback.requested` → `/backend/notifications`, using the envelope in `specs/marketing-events`.

*Alternative: Watermill subscriber POSTing after commit.* Rejected. The in-memory bus loses events on a crash between commit and send, and marketing-app's contract requires at-least-once delivery.
*Alternative: one path for everything.* Rejected in exploration. `/backend/events` belongs to ad-conversions, where "unknown → ignore" is correct. For messages, the same rule would silently drop guests' confirmations.

### D5: Signing secret per organization, outside the database

The secret comes from `MARKETING_APP_EVENTS_SECRET_<ORG_SLUG>` (slug upper-cased, `-` → `_`). This mirrors how marketing-app configures `BackendEventsSecret` per channel through its environment. An organization with several channels shares one secret, configured on each of its channels in marketing-app. An organization with no secret gets no outbox rows, and a warning is logged at startup.

*Alternative: store it on the organization row.* Deferred. It needs encryption at rest, which this repo doesn't have yet, and secrets are already managed in the deploy environment.

### D6: Feedback is a request row, and the review link is never conditional on the rating

The feedback job runs hourly. For each organization it finds `visited` reservations with a `channel_id` and no request, whose local feedback time has come (the day after `starts_at`, at 11:00 in the organization's timezone). It creates a `feedback_request` and writes `feedback.requested` in one transaction. Reservations that turned `visited` more than 3 days ago are skipped, so enabling the feature or recovering from an outage never floods old guests.

`record_my_feedback` targets the caller's latest open request (open for 7 days). A second call before it closes replaces the rating and comment. The response carries `review_url` only from the organization setting `feedback.review_link_enabled && review_url != ""`; the rating is never an input to that decision, which is what makes the flow policy-safe. Ratings of 1–3 return `escalate: true`, and the agent hands off through marketing-app's existing human-handoff.

### D7: Permissions

New permissions:
- `reservations:read` and `reservations:manage` for manager and admin.
- The agent gets `reservations:read` and `reservations:create`, plus `feedback:create`. It can cancel only its own phone's reservations, enforced the way `cancel_my_order` is.
- `feedback:read` for manager and admin.
- The `feedback` settings section is changed with `settings:feedback:update` (manager, admin).

## Risks / Trade-offs

- [Archive order with `organization-ordering-availability`] → It's listed as task 0, and this change's `customer-agent-access` delta is written against its versions.
- [Staff forget to mark a no-show, so a no-show gets "how was your visit?"] → The no-show window lasts until the request is sent the next morning. The template copy should allow for "if you couldn't make it…" (marketing-app).
- [marketing-app ships later than the backend] → Events to `/backend/notifications` get `404`, which is treated like `5xx` and retried until expiry at 72h. If needed, keep `MARKETING_APP_EVENTS_URL` unset until both sides are deployed.
- [Agent-created takeaway orders paid on the edge box] → `order.paid` is written wherever the cloud records the payment, including when a sync push applies it. A payment that never reaches the cloud is never reported.
- [Phone-only identity] → A guest with two numbers becomes two contacts. That's accepted for v1.
- [Secrets via environment] → Onboarding a restaurant needs a deploy-time secret, which matches marketing-app's onboarding today.

## Migration Plan

1. Archive `organization-ordering-availability`.
2. Ship the migrations: contacts, reservations, feedback requests, feedback, the outbox, and the `origin` columns on orders. Everything is additive.
3. Deploy the backend with `MARKETING_APP_EVENTS_URL` unset. Reservations work, and no events are sent.
4. Deploy the marketing-app follow-up, then set the URL and per-organization secrets. Order events start flowing immediately; reservation events start with the next transition.
5. Rollback: unset the URL so sending stops and rows stay. All migrations have down files.

## Open Questions

- The WhatsApp template copy and language for `feedback.requested` and the confirmation messages. This lives in marketing-app.
- Whether the staff MCP toolset needs contact search in v1, or only reservations. It can be added without changing the specs here.
