## 0. Before starting

- [x] 0.1 Archive `organization-integrations` and `wompi-payment-links` (and `marketing-events-integration` if it still modifies `organization-integrations`), so the requirements this change modifies exist in `openspec/specs/`. Verify with `openspec validate agent-order-payment-strategies` showing no "archive would refuse" notes.
- [x] 0.2 In Wompi's sandbox, with test keys, create one `BANCOLOMBIA_QR` transaction and answer design's "Assumptions to verify": does `transaction.updated` arrive and carry our `reference`; can the expiry be set; can a pending transaction be cancelled; is `customer_email` required; which key authenticates `POST /v1/transactions`; does `GET /merchants/info` return both permalinks. Write each answer under that heading in design.md. If the event does not arrive, stop and drop sections 3 and the QR parts of 4 to 6.
- [x] 0.3 Choose the SVG to PNG library with a test that renders the sandbox QR from 0.2 into a PNG a phone's bank app can scan. Note the choice in design.md.
- [ ] 0.4 Ask Wompi's support whether acceptance of the terms inside the order summary is explicit enough (design D12). Note the answer in design.md; it does not block the build.

## 1. Storage and settings

- [x] 1.1 Cloud migration: `payment_strategy`, `payment_expires_at`, `payment_review_started_at` on orders (nullable); `organization_payment_settings` (strategy, manual instructions, manual window, one row per organization); `payment_qr_transactions` (order, Wompi transaction id, reference unique per organization, amount, currency, SVG, URL token unique, status `active|retired`, timestamps; one `active` per order). RLS like the other tenant tables. Verify with `make migrate-up` and `make migrate-down`.
- [x] 1.2 DTOs and ports for payment settings and QR transactions; `WompiCredentials` gains optional `PublicKey` and `IntegritySecret`. Run `make generate-mocks` and verify `go build ./...`.
- [x] 1.3 `PaymentSettingsService`: read with the `wompi` default and a 60 minute window, save with the validation of `payment-strategies`. Verify with service tests for every refusal and the default.
- [x] 1.4 `OrganizationIntegrationService`: accept, validate, encrypt and hint the two optional Wompi values; reads carry `qr_enabled`. Verify with service tests for each scenario in `organization-integrations`, including an integration saved without them.
- [x] 1.5 Repositories for the settings and QR transactions; order reads expose the three new order columns. Verify with repository integration tests.

## 2. Manual strategy

- [x] 2.1 `OrderService` create: read the organization's strategy, record it on agent orders, and under `manual` skip the link, set `payment_expires_at` and skip the payment-links refusal (design D13, D15). Verify with tests: manual order without Wompi is created, `wompi` without keys still refused with `PAYMENT_LINKS_NOT_CONFIGURED`, staff order has no strategy, the strategy survives a later change of the setting.
- [x] 2.2 `OrderService` update and reads: a manual order keeps its strategy and expiry and returns the current instructions and `payment_under_review`. Verify with tests for a changed manual order.
- [x] 2.3 Mark and clear payment review on an order (`payment-strategies`): idempotent, `409` for paid, voided or non-manual orders. Verify with service tests for each case.
- [x] 2.4 `PaymentExpiryService`: a second query for manual orders past their expiry, unpaid and not under review, voided through `ExpireOrder` (design D15). Verify with tests: expired, under review held, review cleared after expiry voided on the next run, `order.expired` recorded.

## 3. QR

- [x] 3.1 Wompi client: read merchant info (permalinks, acceptance tokens), create a `BANCOLOMBIA_QR` transaction with the integrity signature, read a transaction, cancel one if 0.2 found a way. Verify with `httptest` fakes in `wompi_client_test.go` and one manual run against the sandbox.
- [x] 3.2 `PaymentQRService`: create after the link when the organization can offer a QR and the terms were accepted; wait up to `PAYMENT_QR_WAIT` (default 5s); store the SVG and a random token; retire on change, pay, void and expire; read a late QR once per order read (design D1 to D7). Verify with tests for every scenario of the two QR requirements in `payment-links`, each failure leaving the order with its link.
- [x] 3.3 `OrderService`: call the QR service after create and update commit, and on pay, void and expire; an updated order that had a QR gets a new one without the flag. Verify with tests in `order_service_payment_links_test.go`.
- [x] 3.4 `PaymentWebhookService`: a transaction without a link is matched by reference against the active QR; paying deactivates the link and retires the QR; a failed transaction with the QR's reference retires it (design D16). Verify with tests for every scenario of the two modified `payment-links` requirements, including link then QR.
- [x] 3.5 Public `GET /api/payments/qr/:token`, cloud only: PNG rendered from the stored SVG, `404` for unknown, retired or not yet arrived. Verify with handler tests and a route test that it needs no credentials and does not exist on the edge.

## 4. HTTP and agent tools

- [x] 4.1 `GET|PUT /api/organization/payment-settings` with `integrations:read` and `integrations:manage`, cloud only. Verify with handler tests and `integration_routes_test.go`.
- [x] 4.2 `PUT|DELETE /api/orders/:id/payment-review` with the permission paying an order needs. Verify with handler tests.
- [x] 4.3 Integration handlers accept and return the QR fields. Verify with `organization_integration_handler_test.go`.
- [x] 4.4 `mcpserver/tools_customer.go`: `payment_terms_accepted` on create; `payment_strategy`, `payment_qr_url`, `payment_instructions`, `payment_expires_at`, `payment_under_review` on the order tools; `payment_strategy` and `payment_terms` (cached one hour, design D17) on `get_ordering_availability`; updated descriptions. Verify with `tools_customer_test.go` for every scenario in `customer-agent-access`.
- [x] 4.5 Wire the new services, job query and routes in `internal/bootstrap/cloud.go` and `routes_cloud.go`. Verify with `go build ./...` and the bootstrap route tests.

## 5. Frontend

- [x] 5.1 Wompi card: optional public key and integrity secret, their hints, the QR enabled state and the "link only" note. Verify with `pnpm test` on `lib/settings/integrations.test.ts` and `pnpm lint`.
- [ ] 5.2 Ajustes section for the payment of WhatsApp orders: strategy, instructions, window, read-only without `integrations:manage`, the notice when `wompi` has no integration. Verify with component tests and by saving each strategy against a local backend.
- [ ] 5.3 Locate the open-order views, then show an unpaid manual order as awaiting confirmation with its expiry and the mark and unmark review action. Verify with component tests and by reviewing and paying a manual order against a local backend.

## 6. Docs

- [x] 6.1 `backend/docs/api/orders.md` (new order fields, payment review), `organization-integrations.md` (QR fields), `wompi-webhook.md` (matching by reference), a new `payment-settings.md`, and curl examples. Verify each documented request against the handler tests' payloads.

## 7. Verification

- [x] 7.1 From `backend/`: `make lint`, `go build ./...`, `go test ./...`, and `DB_HOST= make test-acceptance` with new acceptance tests in `test/acceptance/payments/` for a manual order's life and a QR payment through the fake Wompi.
- [x] 7.2 From `frontend/`: `pnpm lint` and `pnpm test`.
- [ ] 7.3 Local backend with sandbox keys: create an agent order with the terms accepted, open its `payment_qr_url`, approve it in the sandbox and see the order released to the kitchen; change another order and see the old URL answer `404`.
- [ ] 7.4 Local backend on `manual`: create an agent order, mark it under review past its expiry, pay it and see `order.paid`; let another expire and see `order.expired`.
- [ ] 7.5 marketing-app's `make test-orders-contract` passes its QR, manual and terms scenarios against this backend.
