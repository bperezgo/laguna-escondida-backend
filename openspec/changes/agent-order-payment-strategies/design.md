## Context

See proposal.md for the motivation, and the specs for the requirements.

Constraints from the current code and from Wompi's docs:

- A payment link is created after the order commits, and a Wompi failure never fails the order
  (`payment_link_service.go`). The webhook finds the order through the transaction's
  `payment_link_id` (`payment_webhook_service.go`).
- A Wompi link cannot be limited to chosen payment methods. The QR is a separate call:
  `POST /v1/transactions` with `payment_method.type = BANCOLOMBIA_QR`.
- Wompi produces the QR asynchronously. The transaction is polled until
  `payment_method.extra.qr_image` appears: a base64 SVG of a few KB.
- The QR is paid from Bancolombia and Nequi accounts only, by natural persons.
- WhatsApp does not display SVG, and it fetches an image from a public URL.

## Goals / Non-Goals

**Goals:**

- The customer can be sent the QR as an image, without the backend keeping image files.
- No failure on the QR path fails the order or leaves it without a way to pay.
- Confirming a payment never depends on the image having been produced or delivered.

**Non-Goals:**

- Object storage, a CDN or any file on disk for QR images.
- Retrying a failed QR in the background.

## Decisions

### D1. The QR is best effort; the link is always created

Under the Wompi strategy every agent order gets its link exactly as today. The QR transaction is
attempted after it. Any QR failure leaves the order with the link only, and is logged.

Alternative considered: QR only, with the link created when the customer asks. Rejected: a QR
failure would leave an order with no way to pay, and the link code already exists.

### D2. The QR is stored as Wompi's SVG, in the row of its transaction

The row of the order's QR transaction keeps the SVG text Wompi returned. Nothing is written to a
bucket or to disk, so there is nothing to clean up.

Alternative considered: store nothing and read the QR from Wompi on every request. Rejected: the
image would depend on Wompi being up each time WhatsApp fetches it.

### D3. The PNG is rendered on request, at a public URL with an unguessable token

A public cloud endpoint converts the stored SVG to PNG on each request. The URL carries a random
token, not the order id, so it can be fetched by Meta without exposing orders. It answers
`404` once the order is paid, changed, voided or expired: the moments a link is deactivated.

Alternative considered: render once and store the PNG. Rejected: more bytes stored for an image
fetched once or twice.

### D4. What each failure does

| Failure | Result |
|---|---|
| Wompi refuses the QR transaction | Order created with the link only; logged |
| The QR is not ready within a short wait (a few seconds) | Link only on this response; the QR is returned on a later order read if it has arrived |
| SVG to PNG conversion fails | Link only; logged |
| QR created at Wompi but not stored | Nobody received it, so nobody can pay it; logged |
| WhatsApp cannot fetch or send the image | The agent sends the link as text (marketing-app's change) |

### D5. A QR payment is matched by reference, not by the image

The QR transaction is created with a reference that identifies the order and its version. The
webhook matches an approved transaction by that reference when it has no `payment_link_id`, and
applies the same amount, currency and open-order checks as for a link. A payment therefore
counts even when the image step failed after the transaction was created.

### D6. The QR follows the order's expiry

The QR is valid for as long as the order's link: until the order's payment expiry. When Wompi
lets the transaction carry an expiry it is set to the order's. Either way, an approved payment
for an order that is no longer open is stored as unmatched, as for a link.

### D7. A changed order cancels its QR and gets a new one

A change to the order cancels the pending QR transaction at Wompi when Wompi allows it, and
always retires it on our side (its image URL answers `404`, its reference no longer matches).
The new version gets a new transaction with a new reference: Wompi does not let a reference be
reused.

### D8. The QR needs two more Wompi credentials; without them the strategy is link-only

Creating a transaction needs the organization's public key (to read its acceptance tokens from
`GET /merchants/info`) and its integrity secret (the transaction's `signature` is SHA-256 of
reference, amount, currency and that secret). Neither is stored today. Ajustes gains both as
optional fields of the Wompi integration. An organization that has not entered them keeps working
with the link only, so no organization breaks when this ships.

### D9. The transaction's email is the restaurant's, not the customer's

A WhatsApp customer gives no email, and the agent does not ask for one. If Wompi requires the
field, the transaction carries the email of the organization's owner or admin. To confirm in the
sandbox that Wompi requires it at all.

### D10. Manual orders expire on their own window

An unpaid manual order is voided after a window set in Ajustes (default 60 minutes), by the same
expiry job and with the same `order.expired` notification. It is not voided while staff have
marked it under review.

### D11. An order keeps the strategy it was created under

Changing the setting affects new orders only. An order created as manual stays manual through
its changes and its expiry, and likewise for Wompi.

### D12. The customer accepts Wompi's terms by confirming the order

Wompi requires the payer to accept its terms and its personal-data authorization before a
transaction is created by API. The order summary the agent sends carries one line with both
documents' links, and the customer's confirmation of that summary is the acceptance. The backend
gives the agent the two links (the permalinks `GET /merchants/info` returns) before the order
is created, and reads fresh acceptance tokens when it creates the transaction. An organization
on the manual strategy, or on Wompi without the QR credentials, shows no such line.

Alternative considered: a separate question before the QR is sent. Rejected: one more message on
every order.

### D13. The strategy is always one of two, and defaults to `wompi`

There is no "not chosen" state. An organization without a stored setting reads as `wompi`, which
is today's behaviour, so no data migration is needed for existing organizations and the refusal
`PAYMENT_LINKS_NOT_CONFIGURED` keeps its meaning and its text. `manual` cannot be saved without
instructions, so a manual organization can always take orders.

Alternative considered: a third "none" state with its own refusal code. Rejected: it changes the
error the deployed agent already handles, for no gain.

### D14. The settings are the organization's, not an integration

Strategy, instructions and window are stored in their own row per organization and served at
`/api/organization/payment-settings`, under the existing `integrations:read` and
`integrations:manage` permissions. They are not secrets and are not a provider, so they do not go
through the encrypted integration record.

### D15. The order records its strategy; manual expiry is a second query of the same job

The order gains `payment_strategy` (null for staff and dine-in orders), `payment_expires_at`
and `payment_review_started_at`. `wompi` orders keep expiring through their link, as today. The
expiry job adds one query for manual orders past `payment_expires_at`, unpaid and not under
review, and voids them through the same `ExpireOrder` path, so the void reason and
`order.expired` are identical.

Alternative considered: move every agent order's expiry onto the order. Rejected for this
change: it rewrites working, tested link expiry for no behaviour gain.

### D16. QR transactions have their own table; the webhook looks up by link, then by reference

`payment_qr_transactions` holds, per QR: order, Wompi transaction id, reference, amount,
currency, the SVG, the URL token, status (`active`, `retired`) and timestamps. It is kept apart
from `payment_link_transactions`, which stores what Wompi's events reported. The webhook keeps
its link lookup; a transaction without `payment_link_id` is looked up by `reference`. The
existing pending check for expiry reads only event-reported transactions, so a QR pending since
its creation does not hold the order.

### D17. `get_ordering_availability` carries the terms; the create tool carries the acceptance

The two document permalinks are read from Wompi's `GET /merchants/info` and cached per
organization for a short time (one hour), since the agent calls availability before every order.
The acceptance tokens are never cached: they are read when the QR transaction is created. The
backend cannot see the conversation, so `payment_terms_accepted` on the create tool is the
agent's statement that the confirmed summary showed the terms; without it no QR is created.

## Implementation map

Backend (`backend/`):

- `internal/domain/dto/payment_link.go`, `organization_integration.go`; a new `payment_settings.go`
- `internal/domain/ports/payment_links.go`; new ports for payment settings and QR transactions
- `internal/domain/service/order_service.go` (create, update, read, void, pay, expire)
- `internal/domain/service/payment_link_service.go`, `payment_webhook_service.go`,
  `payment_expiry_service.go`, `organization_integration_service.go`; new
  `payment_settings_service.go`, `payment_qr_service.go`
- `internal/domain/error/order_error.go`
- `internal/platform/httpclient/wompi_client.go` (transactions, merchant info, cancel)
- `internal/platform/handler/organization_integration_handler.go`, `payment_webhook_handler.go`,
  `order_handler.go`; new handlers for payment settings and the QR image
- `internal/platform/postgres/repository/payment_link_repository.go`; new repositories and a
  cloud migration (order columns, `organization_payment_settings`, `payment_qr_transactions`)
- `internal/platform/config/payment_links.go` (QR wait)
- `internal/platform/mcpserver/tools_customer.go`
- `internal/bootstrap/cloud.go`, `routes_cloud.go`
- `test/acceptance/payments/`, `test/acceptance/testsupport/wompi.go`
- `docs/api/orders.md`, `organization-integrations.md`, `wompi-webhook.md`, a new
  `payment-settings.md`; `docs/examples/curl-examples.md`

Frontend (`frontend/`):

- `components/settings/WompiIntegrationCard.tsx`, `IntegrationsPageClient.tsx`, `SettingsHub.tsx`
- `lib/api/organizationIntegrations.ts`, `types/organizationIntegrations.ts`,
  `lib/settings/integrations.ts`; new payment settings API, types and card
- The order views that show an open order and its pay action (to locate at the start of the
  frontend tasks)

## Risks / Trade-offs

- [An order left under review never expires] → It stays in the list of orders awaiting
  confirmation until staff pay it, void it or clear the mark.
- [Manual instructions changed while orders wait] → Open orders show the new text; accepted, as
  the new text is where the restaurant now wants the money.
- [A new dependency to turn SVG into PNG] → Wompi's QR is a simple SVG; a pure-Go rasterizer is
  enough. Chosen in the first backend task, with a test on a real sandbox QR.
- [Acceptance inside the summary may not be explicit enough for Wompi] → Ask Wompi's support
  before production. If they refuse it, fall back to the separate question; only the agent's
  wording changes.

- [A changed order's old QR may still be payable] → If a pending QR transaction cannot be voided
  at Wompi, a customer could pay the old amount. It is stored as unmatched and refunded by staff,
  as an unmatched link payment is today. The spike decides whether we can void it.
- [The customer pays both the QR and the link] → The first approved payment pays the order; the
  second is stored as unmatched and refunded by hand.
- [The QR arrives after the short wait] → The customer gets the link first. Accepted: the order
  is never held back for the image.
- [PNG rendering on every request] → The image is small and fetched a handful of times per order.

## Migration Plan

1. Archive `organization-integrations` and `wompi-payment-links` first: this change modifies
   requirements they introduce.
2. Ship the backend and frontend. Every organization reads as `wompi` and no integration has QR
   keys, so behaviour is unchanged.
3. Ship marketing-app's `payment-qr-and-manual-instructions`.
4. Per restaurant: add the QR keys in Ajustes, or choose `manual`.

Rollback: remove the QR keys (link only) or set the strategy back to `wompi`. The new columns
and tables can stay unused.

## Assumptions to verify in the sandbox

Taken as true for the design. Each has a fallback already in the decisions above, so none
blocks the specs; the first task of the change verifies them.

1. `transaction.updated` fires for a `BANCOLOMBIA_QR` transaction, and its transaction carries
   our `reference` (Wompi's example event does). If it does not fire, the Wompi strategy stays
   link-only.
2. A QR's expiry can be set to the order's (D6 covers the case where it cannot).
3. A pending QR transaction can be cancelled (D7 covers the case where it cannot). Wompi's docs
   show `VOIDED` for cards only, so this one is in doubt.
4. `customer_email` is required, and the restaurant's email is accepted (D9).
5. Which key authenticates `POST /v1/transactions`.

6. `GET /merchants/info` returns a permalink for each of the two documents (D12).

### Sandbox results (2026-11-24, test keys)

1. **Event: verified.** The sandbox finalizes a `BANCOLOMBIA_QR` transaction at once
   (`sandbox_status` APPROVED). With the sandbox events URL pointing at a dev backend,
   `transaction.updated` arrived about 4 seconds after creation, passed the checksum and was
   answered `200`. It has no `payment_link_id`, so today it is stored as paying no order
   ("not one of this organization's links"), which is what matching by reference fixes. That
   the event carries our `reference` was not read from the log; it is the same transaction object
   Wompi returned at creation, and the first QR test on a real order confirms it.
2. **Expiry: accepted, not observable.** `expiration_time` (ISO 8601) is accepted when it is
   part of the signature (`reference + amount + currency + expiration_time + integrity secret`)
   and rejected in the past. Wompi does not echo it back, and the sandbox finalizes at once, so
   enforcement is unverified.
3. **Cancel: not possible to test.** `POST /transactions/{id}/void` on a finalized QR answers
   `unprocessable`. A pending one cannot be produced in the sandbox. Treat D7 as "cannot cancel":
   retire on our side only.
4. **`customer_email` is required** (`422` without it). Any address is accepted. D9 stands.
5. **Either key authenticates `POST /v1/transactions`**, public or private (sandbox).
   `sandbox_status` (APPROVED, DECLINED or ERROR) is mandatory in the sandbox payment method.
6. **`GET /merchants/{public_key}` returns both permalinks** and both acceptance tokens. The
   tokens are single use: a second transaction with the same token is refused, so read them
   fresh for every transaction (D12, D17 already say so).
7. **A reference cannot be reused** (`422`), as D7 assumed.
8. **The QR is ready in the creation poll**, not asynchronous in the sandbox. Wompi's docs say
   production can take a moment; keep the wait of D4.
9. **The SVG is about 111 KB**, not a few KB: a 97x101 grid of 4,770 one-module squares plus a
   logo and a footer using percentage sizes. Stored as text it is fine; D2 stands.

### PNG rendering (D3)

No SVG library is used. A pure-Go SVG library (`oksvg`) fails on the percentage sizes. The QR
modules are `M{x},{y}h1v1h-1z` squares in the first `fill="#000000"` path, so the PNG is drawn
from them with the standard library at 8 px per module (6 KB, 17 ms). A QR reader decodes the
result without the logo. If Wompi changes the path format the render fails, and the order keeps
its link (D4).

