# Local Sync Testing — Two-Node Rig (cloud + edge)

How to bring up a **cloud** instance and an **edge** instance locally and verify the
offline-first sync engine end to end, including the **edge-completely-offline** scenario.

Backed by `docker-compose.sync.yml` at the repo root. No application code changes are
required — the binary branches on `APP_MODE`, auto-runs migrations on boot, and the edge
push/pull loops are pure env wiring.

---

## Topology

| Component   | Container             | Host address                  | Notes                              |
| ----------- | --------------------- | ----------------------------- | ---------------------------------- |
| Cloud app   | `laguna-sync-cloud`   | http://localhost:8080         | `APP_MODE=cloud`, serves sync API  |
| Cloud DB    | `laguna-sync-cloud-db`| `localhost:5433`              | Postgres (own volume)              |
| Edge app    | `laguna-sync-edge`    | http://localhost:8082         | `APP_MODE=edge`, push+pull loops   |
| Edge DB     | `laguna-sync-edge-db` | `localhost:5434`              | Postgres (own volume)              |
| MinIO       | `laguna-sync-minio`   | http://localhost:9101 (console)| **Cloud only** — edge has none     |

Both apps share `ORGANIZATION_ID=test-org` and `NODE_SYNC_KEY=test-sync-key`, so node IDs
derive consistently (the cloud's `NodeID` equals the edge's derived `CloudNodeID`) with no
manual `NODE_ID`/`CLOUD_NODE_ID` wiring.

**Sync cadence:** every minute (5-field cron floor). Expect up to ~60s of latency per hop.

---

## 1. Start the rig

```bash
docker compose -f docker-compose.sync.yml up --build -d
```

First run builds the image once and starts everything. Then watch both boot:

```bash
docker compose -f docker-compose.sync.yml logs -f cloud edge
```

Two psql shortcuts you'll reuse (run from the repo root):

```bash
alias cloudsql='docker compose -f docker-compose.sync.yml exec cloud-db psql -U postgres -d laguna_escondida'
alias edgesql='docker compose -f docker-compose.sync.yml exec edge-db psql -U postgres -d laguna_escondida'
```

---

## 2. Boot verification (checklist A)

- [ ] Cloud log shows `Running in CLOUD mode`.
- [ ] Edge log shows `Running in EDGE mode` and does **NOT** show
      `Edge sync push disabled` (that warning means `CLOUD_SYNC_URL`/`NODE_SYNC_KEY` is missing).
- [ ] Both logged `Migrations completed successfully` (or `No new migrations`).
- [ ] Health: both return 200.
  ```bash
  curl -s localhost:8080/api/health   # cloud
  curl -s localhost:8082/api/health   # edge
  ```
- [ ] Sync tables exist in **both** DBs:
  ```bash
  cloudsql -c "\dt sync_*"
  edgesql  -c "\dt sync_*"     # expect sync_outbox, sync_inbox, sync_state
  ```

## 3. Auth fails closed (checklist B)

- [ ] Push without / with a wrong node key → **401**:
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/api/sync/push \
    -H 'Content-Type: application/json' -d '{"node_id":"x","ops":[]}'           # 401 (no key)
  curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/api/sync/push \
    -H 'X-Node-Key: wrong' -H 'Content-Type: application/json' -d '{"node_id":"x","ops":[]}'  # 401
  ```

---

## 4. Pull path — the fast test (cloud → edge), no auth needed (checklist E)

Reference data (products / users / suppliers) is **cloud-owned** and flows cloud→edge.
Pull is a cursor diff over `updated_at`/`deleted_at`, so you can seed straight into the
cloud DB with `psql` and watch it replicate — no API/login required.

1. Insert a product on the **cloud**:
   ```bash
   cloudsql -c "INSERT INTO products (id, name, category, product_type, unit_of_measure, version, unit_price, vat, vat_amount, ico, ico_amount, sku, total_price_with_taxes, created_at, updated_at) VALUES (gen_random_uuid(), 'Cafe Sync Test', 'bebidas', 'SELLABLE', 'unit', 1, 5000, 0, 0, 0, 0, 'SKU-SYNC-1', 5000, now(), now());"
   ```
   - [ ] Within ~60s the product appears on the **edge**:
     ```bash
     edgesql -c "SELECT id, name, sku FROM products WHERE sku = 'SKU-SYNC-1';"
     ```
   - [ ] Edge advanced its cursor:
     ```bash
     edgesql -c "SELECT peer_node_id, last_pulled_cursor FROM sync_state;"
     ```
2. **Update** propagates:
   ```bash
   cloudsql -c "UPDATE products SET name = 'Cafe Renamed', updated_at = now() WHERE sku = 'SKU-SYNC-1';"
   ```
   - [ ] Edge reflects the new name after the next pull.
3. **Soft-delete (tombstone) propagates:**
   ```bash
   cloudsql -c "UPDATE products SET deleted_at = now(), updated_at = now() WHERE sku = 'SKU-SYNC-1';"
   ```
   - [ ] Edge row gets `deleted_at` set:
     ```bash
     edgesql -c "SELECT name, deleted_at FROM products WHERE sku = 'SKU-SYNC-1';"
     ```
4. **Offline auth (headline):** create a user on the cloud (note the password is a hash in
   real use; for this check insert any row), confirm it replicates **with its password
   column** so the edge could authenticate while offline:
   ```bash
   cloudsql -c "SELECT count(*) FROM users;"   # before
   # create a user via the cloud API or seed directly, then:
   edgesql  -c "SELECT username, (password IS NOT NULL) AS has_hash FROM users ORDER BY created_at DESC LIMIT 3;"
   ```

> The cursor only moves forward: a pull when nothing changed is a no-op and never rewinds
> `last_pulled_cursor`.

---

## 5. Push path (edge → cloud) — must go through the API (checklist C)

Orders (`open_bill`) and purchase entries are **edge-owned** and flow edge→cloud. The
outbox row is written **inside the same transaction** as the business change by the
service — so push must be exercised through the **edge API**, not a raw DB insert (a raw
insert into `open_bill` would never create a `sync_outbox` row and would not sync).

1. Get a token from the **edge** (user must exist on the edge — pull one down first, or
   seed it). Adjust the body to your sign-in payload:
   ```bash
   TOKEN=$(curl -s -X POST localhost:8082/api/auth/signin \
     -H 'Content-Type: application/json' \
     -d '{"username":"<user>","password":"<password>"}' | jq -r .token)
   ```
2. Create an order on the **edge** (needs a product that exists on the edge — pull one
   from step 4 first):
   ```bash
   curl -s -X POST localhost:8082/api/orders \
     -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
     -d '{"products":[{"product_id":"<edge-product-id>","quantity":1}]}'
   ```
   - [ ] Edge wrote an outbox row (origin = edge node, `synced_at` NULL, `seq=1`):
     ```bash
     edgesql -c "SELECT seq, entity_type, operation, synced_at FROM sync_outbox ORDER BY seq;"
     ```
3. Wait ≤60s, then verify the order landed on the **cloud** and the edge marked it synced:
   - [ ] Cloud has the `open_bill`:
     ```bash
     cloudsql -c "SELECT id, status FROM open_bill ORDER BY created_at DESC LIMIT 3;"
     ```
   - [ ] Edge stamped `synced_at` and advanced its push high-water mark:
     ```bash
     edgesql  -c "SELECT seq, synced_at FROM sync_outbox ORDER BY seq;"
     edgesql  -c "SELECT peer_node_id, last_pushed_seq FROM sync_state;"
     ```
   - [ ] Cloud deduped by op_id (inbox):
     ```bash
     cloudsql -c "SELECT op_id, entity_type FROM sync_inbox ORDER BY received_at DESC LIMIT 3;"
     ```

## 6. Idempotency (checklist D)

- [ ] Re-POST the **same** push payload to the cloud twice (capture a real
      `SyncPushRequest` from the edge logs or rebuild one). Both return acks, but the cloud
      `open_bill` row count is unchanged — dedup via `sync_inbox.op_id`. This is the
      "a lost ack got retried" case.

---

## 7. Offline → reconnect (checklist F — the main event)

1. Take the cloud offline (the edge depends only on its own DB, so it keeps serving):
   ```bash
   docker compose -f docker-compose.sync.yml stop cloud
   ```
2. Create 3–5 orders on the **edge** API (as in step 5). Each should succeed locally.
   - [ ] Outbox accumulates with `synced_at` NULL:
     ```bash
     edgesql -c "SELECT count(*) FILTER (WHERE synced_at IS NULL) AS pending FROM sync_outbox;"
     ```
   - [ ] Edge logs show push errors (connection refused) but the **edge does not crash**
         and keeps accepting orders.
3. Bring the cloud back:
   ```bash
   docker compose -f docker-compose.sync.yml start cloud
   ```
   - [ ] Within ~1–2 min the whole backlog drains: `pending` → 0, the cloud now has every
         order in `seq` order, `last_pushed_seq` advanced, **no duplicates**.
4. **Crash-during-drain:** `docker compose -f docker-compose.sync.yml kill edge` mid-drain,
   then `... start edge`. Unsynced rows are retried, still no duplicates (op_id idempotency).
5. *(Harder offline, optional)* cut the edge off the network entirely so even DNS fails:
   ```bash
   docker network disconnect laguna-sync_laguna-sync-network laguna-sync-edge
   # ... reconnect:
   docker network connect laguna-sync_laguna-sync-network laguna-sync-edge
   ```

---

## 8. Reset / teardown

```bash
docker compose -f docker-compose.sync.yml down       # stop, keep data
docker compose -f docker-compose.sync.yml down -v     # stop and wipe both DBs + MinIO
```

---

## Known limitations (by design, for now)

- **Latency:** ~60s per hop (1-minute cron floor). Fine for correctness testing.
- **Blobs don't sync:** only rows replicate. Documents/PDFs uploaded on the cloud are not
  available on the edge, and the edge cannot upload while offline. Storage-backed features
  (document upload, electronic invoices, support documents) are **online-only** — the
  frontend should surface an "offline — feature unavailable" message for them. The edge's
  `SPACES_*` values in the compose are inert placeholders (the S3 client is lazy and never
  contacted by sync).
- **Env management follow-up:** a small future change can make `SPACES_*` required only in
  `cloud` mode (and wire an offline storage adapter on the edge), so the edge needs no
  storage env at all. Deferred to keep this rig zero-code.
