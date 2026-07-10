# Sync Acceptance Spec — What Must Be True, and Why

**Read this first when you come back to the offline-first sync engine after time away.**
It is the durable record of *what the engine promises* and *why each promise matters* — not
how to run a particular command. It is the spec; the tests and the manual playbook are its
two proofs.

Three companion documents, one source of truth:

| Document | Role | Audience |
| --- | --- | --- |
| **This file** (`SYNC_ACCEPTANCE_SPEC.md`) | The invariants — what must hold and why | Future-you, reviewers |
| `SYNC_LOCAL_TESTING.md` | Manual procedure to verify the same invariants by hand on the docker rig | Whoever is debugging live |
| `test/acceptance/sync/` (Tier‑1 tests) | Automated, deterministic proof of the same invariants | CI, regression safety |

Each invariant below has a stable ID (`SYNC-INV-NN`). The manual playbook's checklists
(A–F) and every automated test reference these IDs, so the three stay traceable: if an
invariant changes, you can find its checklist line and its test by ID.

---

## The model in 60 seconds

One **cloud** node and one or more **edge** nodes, each with its own Postgres. The edge is
offline-first: it serves entirely from local data and never hard-depends on the cloud.

Two data flows, deliberately asymmetric:

- **Pull (cloud → edge), reference data.** `products`, `users`, `suppliers`, and
  `product_preparation_responsibilities` are cloud-owned. The edge pulls a **cursor diff**
  over `updated_at`/`deleted_at`: the cloud returns rows changed after the edge's
  `last_pulled_cursor`, the edge upserts them. Payloads carry `deleted_at` (so soft-deletes
  propagate) and the user payload carries the **password hash** (so the edge can authenticate
  while offline). Responsibilities carry their own `updated_at`, so a responsibility-only
  edit replicates even when its product is unchanged. Idempotent and monotonic.
- **Push (edge → cloud), op-log.** `open_bill` (orders) and `purchase_entry` are
  edge-owned. The business change and a `sync_outbox` row are written **in the same
  transaction**. The edge pushes pending outbox ops to the cloud, which applies and acks
  them; the edge advances `last_pushed_seq` and stamps `synced_at`. Dedup is by `op_id`.

Three sync tables on each node: `sync_outbox` (queued local changes), `sync_inbox`
(received remote ops, for dedup), `sync_state` (per-peer high-water marks:
`last_pushed_seq`, `last_pulled_cursor`).

Deterministic entry points (the cron jobs are thin wrappers around these — tests call them
directly, no waiting on cron):

- edge push → `SyncPushService.PushPending(ctx)`
- edge pull → `SyncPullService.PullChanges(ctx)`
- cloud receive → `SyncService.ApplyPush(ctx, req)`

---

## Test tiers

**Tier 1 — in-process two-node tests (the default; ~90% of this spec).**
Real edge stack + real cloud stack in one Go test process, each on its own Postgres
(reuse the `RUN_INTEGRATION_TESTS` gate and the `seedUser` / `seedProduct` / `seedSupplier`
helpers). The cloud's sync handler sits behind an `httptest.Server` so the edge's **real**
HTTP push/pull clients exercise serialization, the `X-Node-Key` auth, the handler, and
inbox dedup. Sync is driven **synchronously** by calling `PushPending` / `PullChanges` in
order — **no `sleep`, no cron, no polling.** Determinism is the maintainability contract: a
Tier‑1 test that needs a timer is a bug in the test.

**Tier 2 — black-box smoke over the real docker rig (a handful only).**
Reserved for what is *only* true of the real topology and cannot be shown in-process:
boot/migration per `APP_MODE`, cron actually firing on cadence, and offline via real
`docker compose stop cloud` / crash via `kill edge`. Uses deadline-bounded `require.Eventually`,
never bare sleeps. Kept few and clearly labeled slow.

Gating & home: Tier‑1 lives in `test/acceptance/sync/`, runs behind
`RUN_ACCEPTANCE_TESTS=true` via `make test-acceptance`, and stays out of the fast
`make test -race` loop (each test self-skips when the gate is off). The harness
(`harness_test.go`) creates and migrates two throwaway databases —
`laguna_accept_cloud` / `laguna_accept_edge` — on the local Postgres given by
`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`, truncating them between tests for isolation.

```bash
# against any local Postgres (e.g. the sync rig's cloud-db on 5433):
RUN_ACCEPTANCE_TESTS=true DB_PORT=5433 make test-acceptance
```

---

## Invariant catalog

Each entry: **Guarantees** (the promise) · **Why** (what breaks for the business if
violated) · **Verify** (tier + how) · **Maps to** (playbook checklist → planned test name).

### Identity & authentication

#### SYNC-INV-01 — Node identities derive deterministically
- **Guarantees:** Both nodes share `ORGANIZATION_ID` + `NODE_SYNC_KEY`, so the cloud's
  `NodeID` equals the edge's derived `CloudNodeID` with no manual `NODE_ID` wiring.
- **Why:** A misderived peer ID means the edge tracks high-water marks against a peer that
  never answers — sync silently no-ops and looks healthy. This is the hardest failure to
  notice in production.
- **Verify:** Tier 1. Derive both identities from the same config inputs; assert
  `cloud.NodeID == edge.CloudNodeID`.
- **Maps to:** (rig setup, implicit) → `TestSync_Identity_DeriveMatchesAcrossNodes`.

#### SYNC-INV-02 — The push endpoint fails closed
- **Guarantees:** `POST /api/sync/push` with a missing or wrong `X-Node-Key` returns **401**
  and mutates nothing.
- **Why:** The sync ingestion path accepts row writes from another node. A missing or
  spoofable key would let anyone inject `open_bill`/`purchase_entry` rows into the cloud.
- **Verify:** Tier 1 (already partly covered by `node_auth_middleware_test.go`). Assert 401
  for no-key and wrong-key, and that `sync_inbox` / `open_bill` counts are unchanged.
- **Maps to:** Checklist **B** → `TestSync_Auth_PushRejectsMissingOrWrongKey`.

### Pull — reference data (cloud → edge)

#### SYNC-INV-03 — New reference rows replicate down
- **Guarantees:** A product/user/supplier created on the cloud appears on the edge on the
  next `PullChanges`.
- **Why:** The edge cannot sell a product or authenticate a user it has never seen. This is
  the baseline "the edge knows about cloud data" promise.
- **Verify:** Tier 1. Seed on cloud → `edge.PullChanges` → assert row present on edge.
- **Maps to:** Checklist **E.1** → `TestSync_Pull_ReplicatesNewProduct` (and `_User`,
  `_Supplier`, `_ProductResponsibility`).

#### SYNC-INV-04 — Updates replicate down
- **Guarantees:** An update to a cloud reference row (e.g. rename) reaches the edge on the
  next pull.
- **Why:** Stale prices/names on the edge produce wrong bills. The cursor diff must catch
  `updated_at` bumps, not just inserts.
- **Verify:** Tier 1. Update on cloud → pull → assert edge reflects the new value.
- **Maps to:** Checklist **E.2** → `TestSync_Pull_ReplicatesUpdate`,
  `TestSync_Pull_ReplicatesProductResponsibility` (responsibility-only change with the
  product row untouched).

#### SYNC-INV-05 — Soft-deletes (tombstones) replicate down
- **Guarantees:** Setting `deleted_at` on a cloud row propagates; the edge row gets
  `deleted_at` set (soft-delete), it is **not** hard-deleted and not left dangling.
- **Why:** A discontinued product must stop being sellable on the edge, but history
  (past bills referencing it) must survive — hence soft-delete, not removal.
- **Verify:** Tier 1. Soft-delete on cloud → pull → assert edge row's `deleted_at` is set.
- **Maps to:** Checklist **E.3** → `TestSync_Pull_ReplicatesTombstone`,
  `TestSync_Pull_ReplicatesResponsibilityTombstone`.

#### SYNC-INV-06 — User payload carries the password hash (offline auth)
- **Guarantees:** The replicated `users` row includes the password hash, so the edge can
  authenticate that user with no cloud round-trip.
- **Why:** This is the headline offline promise — staff must log in at the edge during an
  internet outage. If the hash doesn't replicate, the edge is unusable offline.
- **Verify:** Tier 1. Create user on cloud → pull → assert edge row has a non-empty
  `password`, and that a sign-in on the edge API succeeds while the cloud is unreachable.
- **Maps to:** Checklist **E.4** → `TestSync_Pull_UserHashEnablesOfflineSignin`.

#### SYNC-INV-07 — The pull cursor is monotonic
- **Guarantees:** A pull when nothing changed is a no-op and never rewinds
  `last_pulled_cursor`.
- **Why:** A rewinding cursor re-pulls the world every cycle (bandwidth on a flaky link) or,
  worse, reorders applies. Monotonicity is what makes re-running pull safe.
- **Verify:** Tier 1. Pull, record cursor, pull again with no cloud change, assert cursor
  unchanged and zero rows applied.
- **Maps to:** Checklist **E note** → `TestSync_Pull_CursorDoesNotRewindOnNoOp`.

#### SYNC-INV-08 — Pull is incremental *(gap — not in current playbook)*
- **Guarantees:** A second pull fetches only rows changed *after* the stored cursor, not the
  full table again.
- **Why:** Correctness of the cursor math and edge link cost. A bug that re-sends everything
  is invisible to a "did the row arrive?" check but expensive in production.
- **Verify:** Tier 1. Seed N rows, pull (applies N), seed 1 more, pull again, assert exactly
  1 row applied on the second pull.
- **Maps to:** *(new)* → `TestSync_Pull_SecondPullIsIncremental`.

### Push — edge-owned data (edge → cloud)

#### SYNC-INV-09 — Business change and outbox row are atomic
- **Guarantees:** Creating an order through the **edge API** writes the `open_bill` and its
  `sync_outbox` row in the **same transaction** — both commit or both roll back. (A raw DB
  insert into `open_bill` would create no outbox row and therefore never sync.)
- **Why:** This is the no-silent-loss guarantee. A committed order without an outbox row is
  data that never reaches the cloud and no one notices; an outbox row without the order is a
  phantom on the cloud.
- **Verify:** Tier 1. Create order via edge API → assert exactly one matching pending
  `sync_outbox` row (`origin_node_id` = edge, `synced_at` NULL) in the same commit.
- **Maps to:** Checklist **C.2** → `TestSync_Push_OrderWritesOutboxAtomically`.

#### SYNC-INV-10 — Pushed orders land on the cloud and the edge advances
- **Guarantees:** After `PushPending`, the order exists on the cloud `open_bill`, the edge
  stamped `synced_at`, and `sync_state.last_pushed_seq` advanced.
- **Why:** The core edge→cloud delivery promise. The cloud is the system of record for
  reporting/invoicing; orders that never arrive are lost revenue data.
- **Verify:** Tier 1. Create on edge → `PushPending` → assert cloud has the row, edge outbox
  `synced_at` set, `last_pushed_seq` advanced.
- **Maps to:** Checklist **C.3** → `TestSync_Push_OrderLandsOnCloudAndMarksSynced`.

#### SYNC-INV-11 — The cloud records each op in the inbox
- **Guarantees:** The cloud writes a `sync_inbox` row keyed by `op_id` for each applied op.
- **Why:** The inbox is the dedup ledger that makes retries safe (see SYNC-INV-12). Without
  it, a retried push double-applies.
- **Verify:** Tier 1. After push, assert `sync_inbox` has the op's `op_id`.
- **Maps to:** Checklist **C.3** → `TestSync_Push_CloudRecordsInboxOp`.

#### SYNC-INV-12 — purchase_entry follows the same edge→cloud path *(gap)*
- **Guarantees:** A purchase entry created on the edge replicates to the cloud with the same
  atomic-outbox, ack, dedup behavior as orders.
- **Why:** The playbook exercises orders end-to-end but only mentions purchase entries.
  `purchase_entry` is a distinct entity type and applier; it can regress independently.
- **Verify:** Tier 1. Mirror the order push test for purchase entries.
- **Maps to:** *(new)* → `TestSync_Push_PurchaseEntryReplicates`.

### Idempotency & ordering

#### SYNC-INV-13 — Replaying a push applies once (lost-ack retry)
- **Guarantees:** Re-sending the same `SyncPushRequest` (the "ack got lost, edge retries"
  case) returns an ack both times, but the cloud `open_bill` count is unchanged — deduped by
  `sync_inbox.op_id`.
- **Why:** Acks get lost on flaky links constantly. Without idempotency, every retry
  duplicates an order on the cloud.
- **Verify:** Tier 1. Capture a real `SyncPushRequest`, `ApplyPush` it twice, assert one row
  and one inbox entry.
- **Maps to:** Checklist **D** → `TestSync_Idempotency_DuplicatePushAppliesOnce`.

#### SYNC-INV-14 — Ops apply in seq order; the high-water mark only advances
- **Guarantees:** Ops are applied in `seq` order and `last_pushed_seq` never regresses.
- **Why:** Order matters for update-after-create on the same entity. A regressing HWM
  re-sends or skips ops.
- **Verify:** Tier 1. Create several ordered changes, push, assert cloud applied them in
  `seq` order and the HWM equals the max acked seq.
- **Maps to:** Checklist **F.3** (no-duplicates/in-order) → `TestSync_Ordering_AppliesInSeqAndMonotonicHWM`.

#### SYNC-INV-15 — Partial acks are handled exactly *(gap)*
- **Guarantees:** If the cloud acks a subset of a batch, the edge advances only to the acked
  seqs and re-sends the unacked remainder — no drop, no double-count.
- **Why:** Batch boundaries and interrupted responses are normal. This is the seam where
  "no duplicates" and "no loss" are both easiest to break.
- **Verify:** Tier 1. Force a partial ack (stub the cloud to ack a prefix), assert edge
  re-sends exactly the remainder on the next `PushPending`.
- **Maps to:** *(new)* → `TestSync_Ordering_PartialAckResendsRemainder`.

### Offline & resilience (the main event)

#### SYNC-INV-16 — The edge keeps serving while the cloud is unreachable
- **Guarantees:** With the cloud down, orders created on the edge still succeed locally; the
  outbox accumulates with `synced_at` NULL; the edge process does not crash.
- **Why:** The entire reason the edge exists. An internet outage must not stop the point of
  sale.
- **Verify:** Tier 1 (point the edge push client at a closed/erroring server) for the engine
  behavior; Tier 2 (`docker compose stop cloud`) for the real topology.
- **Maps to:** Checklist **F.2** → `TestSync_Offline_EdgeServesAndQueues` (T1) +
  smoke (T2).

#### SYNC-INV-17 — On reconnect, the backlog drains exactly once
- **Guarantees:** When the cloud returns, the full backlog drains: pending → 0, the cloud has
  every queued order in `seq` order, `last_pushed_seq` advanced, **no duplicates**.
- **Why:** The recovery promise. Hours of offline orders must all arrive, once each, in
  order — this is where money is either reconciled or lost.
- **Verify:** Tier 1. Queue N orders against an erroring server, "reconnect" (restore the
  server), `PushPending`, assert all N on cloud once, in order, pending = 0.
- **Maps to:** Checklist **F.3** → `TestSync_Offline_BacklogDrainsOnceInOrder`.

#### SYNC-INV-18 — Crash during drain is safe
- **Guarantees:** Killing the edge mid-drain and restarting re-sends only the unsynced rows;
  still no duplicates (op_id idempotency).
- **Why:** Power loss / restart during the catch-up burst after a long outage is exactly when
  the system is under stress and most likely to double-send.
- **Verify:** Tier 1 (simulate: ack half, drop the process state, resume — assert the rest
  send once); Tier 2 (`kill edge` mid-drain) for the real crash.
- **Maps to:** Checklist **F.4** → `TestSync_Offline_CrashDuringDrainNoDuplicates`.

### Boundaries — assert the negatives (these are *by design*)

#### SYNC-INV-19 — Storage-backed features are online-only; the edge never blocks on storage
- **Guarantees:** Sync replicates rows only; blobs (documents, PDFs, electronic invoices,
  support documents) do not sync. The edge's sync path never contacts object storage, so a
  missing/placeholder `STORAGE_*` config does not break the edge.
- **Why:** This is a deliberate scope line. A future change that accidentally couples sync to
  storage would make the edge fail offline — the exact thing the edge exists to avoid. The
  test guards the boundary.
- **Verify:** Tier 1. Run a full pull+push cycle with an edge storage client that fails on
  any call; assert sync completes and the storage client was never invoked.
- **Maps to:** Known limitations (playbook) → `TestSync_Boundary_SyncNeverTouchesStorage`.

#### SYNC-INV-20 — Both nodes self-migrate and own the sync tables *(Tier 2)*
- **Guarantees:** On boot each node runs migrations and ends with `sync_outbox`,
  `sync_inbox`, `sync_state` present, regardless of `APP_MODE`.
- **Why:** A node that boots without its sync tables fails opaquely on first sync. Catching
  it at boot is cheaper than at first outage.
- **Verify:** Tier 2 smoke against the docker rig (boot both, assert tables + health 200).
- **Maps to:** Checklist **A** → `TestSync_Boot_BothNodesHaveSyncTables` (smoke).

#### SYNC-INV-21 — Mode wiring fails loud, not silent *(Tier 2)*
- **Guarantees:** An edge missing `CLOUD_SYNC_URL`/`NODE_SYNC_KEY` logs `Edge sync push
  disabled` and does not attempt to push (it does not crash, but it does not pretend to sync).
- **Why:** A silently disabled push loop looks identical to a working one until orders pile
  up undelivered. The loud warning is the early signal.
- **Verify:** Tier 2 / config test. Boot edge without the vars, assert the warning and that
  no push is attempted.
- **Maps to:** Checklist **A** → `TestSync_Config_EdgeWithoutCloudUrlDisablesPush`.

---

## Traceability matrix

Status: ✅ implemented · 🟡 partial · ⬜ not yet.

| Invariant | Concern | Playbook | Tier | Status | Test |
| --- | --- | --- | --- | --- | --- |
| SYNC-INV-01 | Identity derivation | setup | 1 | ⬜ | `TestSync_Identity_DeriveMatchesAcrossNodes` |
| SYNC-INV-02 | Auth fails closed | B | 1 | ⬜ | `TestSync_Auth_PushRejectsMissingOrWrongKey` (also `node_auth_middleware_test.go`) |
| SYNC-INV-03 | Pull: new rows | E.1 | 1 | ✅ | `TestSync_Pull_ReplicatesNewProduct`, `TestSync_Pull_ReplicatesProductResponsibility` |
| SYNC-INV-04 | Pull: updates | E.2 | 1 | ✅ | `TestSync_Pull_ReplicatesUpdate`, `TestSync_Pull_ReplicatesProductResponsibility` (responsibility-only) |
| SYNC-INV-05 | Pull: tombstones | E.3 | 1 | ✅ | `TestSync_Pull_ReplicatesTombstone`, `TestSync_Pull_ReplicatesResponsibilityTombstone` |
| SYNC-INV-06 | Pull: offline auth | E.4 | 1 | 🟡 | `TestSync_Pull_UserCarriesPasswordHash` (hash replicates; live offline sign-in TODO) |
| SYNC-INV-07 | Pull: cursor monotonic | E note | 1 | ✅ | `TestSync_Pull_CursorDoesNotRewindOnNoOp` |
| SYNC-INV-08 | Pull: incremental | *(new)* | 1 | ✅ | `TestSync_Pull_SecondPullIsIncremental` |
| SYNC-INV-09 | Push: atomic outbox | C.2 | 1 | ⬜ | `TestSync_Push_OrderWritesOutboxAtomically` (needs OrderService) |
| SYNC-INV-10 | Push: lands + advances | C.3 | 1 | ✅ | `TestSync_Push_OrderLandsOnCloudAndMarksSynced` |
| SYNC-INV-11 | Push: inbox record | C.3 | 1 | ✅ | `TestSync_Push_OrderLandsOnCloudAndMarksSynced` |
| SYNC-INV-12 | Push: purchase_entry | *(new)* | 1 | ⬜ | `TestSync_Push_PurchaseEntryReplicates` |
| SYNC-INV-13 | Idempotent replay | D | 1 | ✅ | `TestSync_Idempotency_DuplicatePushAppliesOnce` |
| SYNC-INV-14 | Seq order + HWM | F.3 | 1 | ✅ | `TestSync_Ordering_AppliesInSeqAndMonotonicHWM` |
| SYNC-INV-15 | Partial ack | *(new)* | 1 | ⬜ | `TestSync_Ordering_PartialAckResendsRemainder` |
| SYNC-INV-16 | Offline: serves + queues | F.2 | 1+2 | ⬜ | `TestSync_Offline_EdgeServesAndQueues` |
| SYNC-INV-17 | Reconnect: drains once | F.3 | 1 | ⬜ | `TestSync_Offline_BacklogDrainsOnceInOrder` |
| SYNC-INV-18 | Crash during drain | F.4 | 1+2 | ⬜ | `TestSync_Offline_CrashDuringDrainNoDuplicates` |
| SYNC-INV-19 | Storage boundary | limits | 1 | ⬜ | `TestSync_Boundary_SyncNeverTouchesStorage` |
| SYNC-INV-20 | Boot/migrations | A | 2 | ⬜ | `TestSync_Boot_BothNodesHaveSyncTables` |
| SYNC-INV-21 | Mode wiring loud | A | 2 | ⬜ | `TestSync_Config_EdgeWithoutCloudUrlDisablesPush` |

---

## Conventions

- **Test names:** `TestSync_<Area>_<Behavior>` (`Pull`, `Push`, `Offline`, `Idempotency`,
  `Ordering`, `Auth`, `Identity`, `Boundary`, `Boot`, `Config`). The name should read as the
  promise, so a failing test names the broken guarantee.
- **Trace by ID:** put the `SYNC-INV-NN` in the test's doc comment. Grep an ID to jump
  spec ↔ playbook ↔ test.
- **Determinism (Tier 1):** drive sync by calling `PushPending` / `PullChanges` /
  `ApplyPush` in order. No `time.Sleep`, no cron, no polling. If a test seems to need a
  timer, the trigger is being modeled wrong.
- **Isolation:** each test seeds its own data and cleans up via `t.Cleanup` (the existing
  `seedUser`/`seedProduct`/`seedSupplier` pattern), so tests are order-independent.
- **Gating:** Tier 1 behind `RUN_ACCEPTANCE_TESTS=true` (`make test-acceptance`), excluded
  from the fast `make test -race`. Tier 2 documented as slow/manual or its own CI stage.

## Out of scope (by design — do not write tests expecting these to work)

- **Blob replication.** Documents/PDFs/invoices do not sync; storage features are
  online-only (see SYNC-INV-19).
- **Sub-cron latency.** In production, hops are bounded by the 1-minute cron floor. Tier‑1
  tests bypass cron entirely, so they assert *correctness*, never *timing*.
- **Multi-edge conflict resolution.** Current entities are single-writer per direction
  (reference = cloud-authored, orders/entries = edge-authored), so there is no concurrent
  conflict to resolve. Revisit this section if an entity ever becomes writable on both sides.

## How to extend this spec

When you make an entity sync-backed, or change the contract:

1. Add a `SYNC-INV-NN` entry here (guarantees + why + verify + maps-to).
2. Add the row to the traceability matrix.
3. Add the Tier‑1 test (and a playbook checklist line if it needs manual verification).
4. If it's a new edge→cloud entity, it needs the atomic-outbox guarantee (SYNC-INV-09)
   restated for it; if cloud→edge, the cursor/tombstone guarantees (SYNC-INV-04/05/07).
