## 1. Roles, permissions and schema

- [x] 1.1 Add the shared migration inserting role `courier` (id 7) and `permissions.RoleCourier`; add it to `CloudRoles`; verify with the migration embed test and a role test that a courier passes `HasCloudRole`
- [x] 1.2 Add `deliveries:read`, `deliveries:manage`, `deliveries:carry`; map read+manage to admin and manager, carry to courier, nothing else to courier; verify with `role_permissions_test.go`
- [x] 1.3 Add the cloud migration: delivery columns and `ready_at` on `open_bills`, the `(organization_id, delivery_status)` index, and the tenant-scoped `order_delivery_events` table (up and down); verify `make test` migration tests apply up, down, up on a testcontainers DB
- [x] 1.4 Confirm the sync applier's on-conflict column list leaves the new columns untouched; verify with an acceptance test that re-pushing an order keeps its delivery status

## 2. Domain policy

- [x] 2.1 Add delivery DTOs (statuses, failure reasons, courier) and errors (`INVALID_DELIVERY_TRANSITION` 409 with the current status, forbidden 403, validation for the courier, reason and note); verify with unit tests on the error mapping
- [x] 2.2 Implement the pure transition policy (design D2) covering every row of the spec's transition and "who may" tables, including takeaway, unpaid, voided, dine-in, claim, another courier's order and re-dispatch; verify with a table-driven unit test, one case per spec scenario

## 3. Persistence

- [x] 3.1 Repository: the conditional status update plus the history append (design D3), returning "not changed" on zero rows; verify with an acceptance test where two concurrent claims leave exactly one winner and one history entry
- [x] 3.2 Repository reads: the dashboard list (not delivered of any day, plus delivered this business day, with status and type filters and derived `preparing`/`ready`), the history with `paid_at` and `ready_at`, the courier's claimable and on-the-road lists in spec order, and the active couriers; verify with acceptance tests mirroring the spec scenarios ("Yesterday's undelivered order", "Juan's queue")

## 4. Services

- [x] 4.1 `DeliveryService` dispatch, deliver and fail: load, apply the policy, persist and record the marketing notification in one unit of work, and reload to answer 409 on a lost race; verify with service tests for each transition, the 403 and 409 paths, and a rollback leaving neither history nor event
- [x] 4.2 Kitchen path: on the order becoming finished, paid and `ready_at` null, set `ready_at` and, for takeaway, record `order.ready` in the same transaction (design D4); verify with tests that undo then re-complete records only one `order.ready`, and that a delivery order records none

## 5. Marketing notifications

- [x] 5.1 Add the four notification types and a builder taking `order_id`, `order_type` and an optional `courier_name` or `reason`, with no note parameter; verify with unit tests on the exact `data` of each type, including that a failed one carries `reason` and no note
- [x] 5.2 Verify end to end that a claimed agent delivery order produces one signed `/backend/notifications` POST of `order.out_for_delivery` and a till order produces none (acceptance test against a stub receiver)

## 6. HTTP

- [x] 6.1 `DeliveryHandler` and the routes of design D7 in `routes_cloud.go`, with request validation and the courier's implicit self on dispatch; verify with route tests for each permission (admin, manager, courier, server → 403) and for the dashboard answering 403 to a courier
- [x] 6.2 Verify with an edge route test that every `/api/deliveries/...` route answers 404 on the box
- [ ] 6.3 Write `backend/docs/api/deliveries.md` (endpoints, bodies, errors, curl examples); update `marketing-events.md` (four types and their `data`) and `users.md` (courier role, email required); verify the curl examples against a local cloud run

## 7. Frontend

- [x] 7.1 Types, API client and proxy routes under `app/api/deliveries/...`; add `DELIVERIES_*` permission constants; verify with `pnpm test` client tests
- [x] 7.2 "Pedidos" tab in `DeliveriesPageClient`, first and gated by `deliveries:read`: status columns, time in status, 10 s polling while visible, refetch after actions; verify with component tests for tab gating and column grouping
- [x] 7.3 Dispatch dialog (courier select or third-party name), deliver / "Recogido" by order type, fail dialog (reason and note), re-dispatch, history drawer; verify with component tests for validation and for showing the 409 message
- [ ] 7.4 Bag slip print component on the `browserPrint` path, with the QR code drawn client-side using `qrcode`; verify by printing a delivery order in a local run and scanning the QR to the map link
- [ ] 7.5 `/my-deliveries` courier page (claimable and mine, map and `tel:` links, deliver and fail, "Otro repartidor ya lo tomó" on 409) and a home tile gated by `deliveries:carry`; verify with component tests and on a phone-width viewport
- [x] 7.6 User form: "Repartidor" label, `courier` in `CLOUD_ROLE_NAMES`, email helper and error text; verify with the existing user form tests extended
- [ ] 7.7 Run `pnpm lint`, `pnpm test` and `pnpm build` in `frontend/` and verify they pass

## 8. Integration and rollout

- [x] 8.1 Run `make lint` and `make test` in `backend/` and verify they pass
- [ ] 8.2 Manual run against a local cloud: an agent delivery order goes paid, ready, claimed by a courier, failed, re-dispatched and delivered, and an agent takeaway goes ready and "Recogido"; verify the dashboard, the courier page, the history and the outbox rows at each step
- [ ] 8.3 Confirm marketing-app handles (or answers 2xx to) the four new types before the cloud deploy, and record its contract update; verify with a signed test POST to its staging endpoint
