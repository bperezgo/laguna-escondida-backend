## 0. Before starting

- [ ] 0.1 In Wompi's sandbox, confirm how a link is deactivated (`PATCH /v1/payment_links/:id` with `active: false`) and which field of a `transaction.updated` transaction names its link (`payment_link_id`). Note the answers in design's Open Questions and adjust D3/D4 if they differ.

## 1. Configuration and storage

- [x] 1.1 `internal/platform/config`: `WOMPI_API_URL`, `WOMPI_PRIVATE_KEY_<SLUG>`, `WOMPI_EVENTS_SECRET_<SLUG>`, `PAYMENT_LINK_TTL` (default 30m), following `marketing_events.go`; startup log per active organization. Verify with config tests: both keys, one key, none.
- [x] 1.2 Cloud migration: `payment_links` (id, organization_id, order_id, wompi_link_id, amount_in_cents, currency, expires_at, status `active|replaced|expired|cancelled`, timestamps; one `active` per order), `payment_link_transactions` (wompi_transaction_id unique per organization, link, status, amount, currency, payment_method_type, `matched` flag, raw event), and nullable `void_reason` on orders. RLS like the other tenant tables. Verify with `make migrate-up` / `migrate-down`.

## 2. Domain

- [x] 2.1 DTOs and ports: `PaymentLinkProvider` (create, deactivate), `PaymentLinkRepository`, `PaymentLinkConfig` (configured?, secrets per organization). `make generate-mocks`.
- [x] 2.2 `PaymentLinkService`: create after an order commits, replace on change, deactivate on void (design D2, D3). Tests: amount in cents, expiry = creation + TTL kept on replace, Wompi failure leaves the order without a link and returns no error.
- [x] 2.3 `OrderService`: refuse agent orders with `PAYMENT_LINKS_NOT_CONFIGURED` after availability, before the zone check (D8); call the link service after create and update commit; deactivate on `DeleteOrder`. Tests per scenario in `delivery-orders` and `customer-agent-access`; staff-created takeaway unaffected.
- [x] 2.4 Webhook use case (D4): checksum, `transaction.updated` only, idempotent insert, match active link + unpaid + amount/currency, `PayOrder` in the organization's context with the mapped payment code, unmatched stored and logged, `PENDING` stored. Tests for every `payment-links` scenario.
- [x] 2.5 Expiry job (D5): per organization every minute, skip pending, void with `payment_expired`, record `order.cancelled` and `order.expired`. Tests: expired, pending held, pending then declined voided next run, no link never expires.
- [x] 2.6 Marketing events (D6): notification types for `order.paid` (every agent payment) and `order.expired`, bodies in `marketing_event/`. Tests: body shape, distinct event ids, recorded in the payment/void transaction.

## 3. Platform

- [ ] 3.1 Wompi adapter under `internal/platform/` (create link, deactivate), with an `httptest` fake. Test against sandbox once manually.
- [x] 3.2 Repositories for `payment_links` and `payment_link_transactions`; order reads expose `void_reason` and the active link.
- [x] 3.3 `POST /api/webhooks/wompi/:organization_id` handler, public, cloud only; `401` on bad checksum, `200` otherwise.
- [x] 3.4 Expiry job wired in `internal/bootstrap/cloud.go`.
- [x] 3.5 `mcpserver/tools_customer.go`: `payment_url` on create/update/list, `payment_state: expired` for orders voided with `payment_expired` today, the refusal message, updated tool descriptions (wording in marketing-app's `evals/ordersfake`). Tests.

## 4. Docs

- [x] 4.1 `docs/api/orders.md` (refusal, `payment_url`, `void_reason`), `docs/api/marketing-events.md` (two notifications), new `docs/api/wompi-webhook.md`, and a curl example.

## 5. Verification

- [x] 5.1 `make lint`, `go build ./...`, `go test ./...` from `backend/`.
- [ ] 5.2 Local backend with sandbox keys and a short `PAYMENT_LINK_TTL`: pay a link in Wompi's sandbox and see the order released to the kitchen and both `order.paid` events; let another expire.
- [ ] 5.3 marketing-app L2 contract: `ORDERS_MCP_URL=… ORDERS_MCP_TOKEN=… ORDERS_PAYMENT_TTL=<ttl> ORDERS_MCP_TOKEN_NO_PAYMENTS=<key of an org without Wompi> CONTRACT_REQUIRE=1 go test ./whatsapp-agent/internal/orderscontract/ -run TestBackendContract -v` passes the payment scenarios.
