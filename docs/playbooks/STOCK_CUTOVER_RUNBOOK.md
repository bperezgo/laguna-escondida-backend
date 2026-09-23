# Stock Cutover Runbook — moving on-hand ownership to the cloud

Until this cutover, the Windows box owns on-hand stock: it writes the number and the cloud
assigns its mirror from whatever snapshot arrives last. After it, the **cloud** owns on-hand
— it is the sum of the movements the cloud has folded — and the office can record purchases,
corrections and counts directly.

The change is deliberately sequenced so the riskiest part runs in production at zero risk
before anything depends on it. Work through the steps in order; each one states what to
verify before moving on.

> **Rollback boundary:** steps 1–3 are reversible. **Step 4 is the point of no return.**
> Once the write endpoints move, the office records movements the restaurant never sees, and
> re-enabling the old snapshot applier would let the restaurant's absolute overwrite them.
> After step 4, roll *forward*.

---

## Before you start

| What | Where |
| ---- | ----- |
| Local two-node rig | `docs/playbooks/SYNC_LOCAL_TESTING.md` |
| Invariants and their tests | `docs/playbooks/SYNC_ACCEPTANCE_SPEC.md` |
| Endpoint reference | `docs/api/stock.md` |

Do a **full dry run on the local rig first** (see "Dry run" at the bottom). Every step below
has a local equivalent.

Two psql shortcuts, used throughout:

```bash
alias cloudsql='docker compose -f docker-compose.sync.yml exec cloud-db psql -U postgres -d laguna_escondida'
alias edgesql='docker compose -f docker-compose.sync.yml exec edge-db psql -U postgres -d laguna_escondida'
```

---

## Step 1 — Schema, inert

Deploy the migrations. `historic_stock.kind` and `sync_state.last_stock_pulled_cursor` are
both additive with defaults, so nothing changes behavior on either node.

```bash
make migrate-up   # or let the binary run them on boot
```

**Verify**

```sql
-- kind exists, existing rows adopted 'unknown', nothing was lost
SELECT kind, count(*) FROM historic_stock GROUP BY kind;
```

- [ ] Every pre-existing ledger row is present and reads `unknown`.
- [ ] Both nodes still boot and sync exactly as before.

**Rollback:** `make migrate-down` twice. No data depends on either column yet.

---

## Step 2 — Enable the fold while the assignment is still live

Deploy the new `HistoricStockSyncApplier` (which folds `amount += change`) **while the
snapshot applier still assigns the amount**, and with the edge still emitting snapshots.

This is the safe rehearsal. Because a movement op always carries a lower `seq` than the
snapshot op from the same write, the cloud applies the fold and then overwrites it with the
edge's absolute. The cloud's numbers end up byte-for-byte what they are today, while the
fold runs against real production traffic.

Run the reconciliation job in report-only mode and confirm it finds nothing:

```bash
# it is already report-only; it never corrects
docker compose -f docker-compose.sync.yml logs cloud | grep reconcileStock
```

**Verify**

- [ ] `reconcileStock` runs on its cron (`STOCK_RECONCILE_CRON`, default `15 3 * * *`).
- [ ] It reports **zero** diverged products across at least one full day of trading.
- [ ] Spot-check a busy product: its cloud amount still matches the restaurant's.

If divergences appear here, **stop**. The fold is wrong and nothing downstream is safe.

**Rollback:** redeploy the previous applier. The ledger rows written meanwhile stay valid
under either owner.

---

## Step 3 — Flip ownership

Three things happen together, in this order.

**3a. Drain the edge's outbox completely.** Nothing may be in flight when ownership moves.

```sql
-- on the edge
SELECT count(*) FROM sync_outbox WHERE synced_at IS NULL;
```

- [ ] The count is **0**. If it is not, the restaurant is offline or the push loop is
      failing — resolve that before continuing, or those movements are lost from the
      cloud's number.

**3b. Deploy the applier that stops assigning.** `StockSyncApplier` keeps only its delete
path; a replicated snapshot is acked and ignored. The cloud's amounts do not move at this
point: the mirror already equals the restaurant's numbers by construction.

**3c. Write the opening balances.** Movement history from before replication was never sent
to the cloud, so amounts are real but the movements explaining them are missing. This writes
the one reconciling `opening_balance` movement per product that closes the gap:

```bash
curl -s -X POST https://<cloud-host>/api/stock/opening-balances \
  -H "X-API-Key: $ADMIN_API_KEY"
```

```json
{ "checked_products": 214, "written_balances": 198 }
```

The command is safe to re-run: a second call finds no gap and writes nothing
(`"written_balances": 0`).

**Verify**

```sql
-- on the cloud: every product's amount equals the sum of its movements
SELECT s.product_id, s.amount, COALESCE(SUM(h.change), 0) AS ledger_sum
FROM stock s
LEFT JOIN historic_stock h ON h.product_id = s.product_id
WHERE s.deleted_at IS NULL
GROUP BY s.product_id, s.amount
HAVING s.amount <> COALESCE(SUM(h.change), 0);
```

- [ ] That query returns **no rows**.
- [ ] Re-running the opening-balance command reports `written_balances: 0`.
- [ ] A sale at the restaurant still reduces the cloud's number by exactly the amount sold.

**Rollback:** re-enable the snapshot applier. The restaurant's absolute reasserts itself per
product on its next movement, and the opening-balance rows remain valid — they describe real
history either way.

---

## Step 4 — Move authoring  ⚠️ point of no return

Deploy the binary that registers `POST /api/stock`, `PUT /api/stock/:product_id/add-or-decrease`,
`DELETE /api/stock/:product_id` and `POST /api/stock/bulk` in **cloud** mode and removes them
from **edge** mode, then point the web app's stock screens at the cloud.

**Verify**

```bash
# cloud: served (401 without a token, not 404)
curl -s -o /dev/null -w '%{http_code}\n' -X POST https://<cloud-host>/api/stock
# edge: gone
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://<edge-host>/api/stock   # expect 404
```

- [ ] All four write endpoints answer on the cloud and return `404` on the edge.
- [ ] `GET /api/stock` still answers on **both**.
- [ ] `GET /api/stock/sync-freshness` answers on the cloud and the batch-count screen reads it.
- [ ] Recording a purchase entry in the office raises the cloud's on-hand — and it *stays*
      raised after the restaurant's next push. This is the bug the whole change exists to fix.

**From here, roll forward, not back.**

---

## Step 5 — Edge cleanup

Deploy the edge build that stops emitting the `stock` snapshot op and runs the daily stock
refresh (`STOCK_PULL_CRON`, default `0 5 * * *` — before service, so the numbers stay stable
through the day).

**Verify**

```sql
-- on the edge: only movement ops are queued, never a snapshot
SELECT entity_type, count(*) FROM sync_outbox GROUP BY entity_type;
```

- [ ] No new `stock` rows appear in the edge's outbox; `historic_stock` rows still do.
- [ ] The edge logs `Edge stock refresh job completed` once per day.
- [ ] After a refresh, the edge's amounts match the cloud's, including office-side purchases.
- [ ] `sync_state.last_stock_pulled_cursor` advances independently of `last_pulled_cursor`.

---

## Dry run on the local rig

Do this whole sequence locally before touching production. The rig's edge is configured with
`STOCK_PULL_CRON: "* * * * *"` so the refresh is observable in a minute rather than a day.

```bash
make sync-up
make sync-logs   # in another terminal
```

1. **Boot** — cloud logs `Running in CLOUD mode` and `Registered cron job: reconcileStock`;
   edge logs `Running in EDGE mode` and `Edge sync scheduler started` with a `stock_pull_cron`.
2. **Seed and sell** — create a product and stock on the cloud, let the edge pull it, then
   record an order on the edge.
3. **Watch the fold** — after a push cycle, `cloudsql -c "SELECT amount FROM stock"` shows the
   amount reduced by what was sold, and `historic_stock` holds the movement with `kind='sale'`.
4. **Record a purchase in the office** — raise the cloud's stock, then push more edge sales
   and confirm the purchase is still there.
5. **Take the cloud down** — `docker compose -f docker-compose.sync.yml stop cloud`. Keep
   selling on the edge; the sales complete and the movements queue. Bring the cloud back and
   confirm every queued movement folds exactly once.
6. **Watch the refresh** — the edge logs `Edge stock refresh job completed` and its amounts
   match the cloud's.
7. **Run the opening-balance command twice** — the second run reports `written_balances: 0`.

```bash
make sync-reset   # tear down and wipe when finished
```

The same ground is covered automatically by `RUN_ACCEPTANCE_TESTS=true make test-acceptance`
(`test/acceptance/sync/stock_test.go`); the dry run is for watching it happen over real
HTTP and real crons.

If Docker Hub is unreachable and the rig image cannot be rebuilt, run the same sequence with
two local binaries against two databases — it exercises the identical code paths:

```bash
go build -o /tmp/laguna ./cmd
# cloud
APP_MODE=cloud PORT=8080 DB_NAME=laguna_rig_cloud STOCK_RECONCILE_CRON="* * * * *" ... /tmp/laguna
# edge
APP_MODE=edge PORT=8082 DB_NAME=laguna_rig_edge CLOUD_SYNC_URL=http://localhost:8080 \
  SYNC_PUSH_CRON="* * * * *" SYNC_PULL_CRON="* * * * *" STOCK_PULL_CRON="* * * * *" ... /tmp/laguna
```
