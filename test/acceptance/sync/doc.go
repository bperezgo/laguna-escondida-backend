// Package sync holds the Tier-1 acceptance tests for the offline-first sync engine:
// an in-process two-node rig (a real cloud stack + a real edge stack, each on its own
// Postgres) wired over a real httptest HTTP boundary. Sync is driven synchronously by
// calling the push/pull service methods directly — no cron, no sleeps — so the tests
// assert correctness, never timing.
//
// Each test maps to an invariant in docs/playbooks/SYNC_ACCEPTANCE_SPEC.md (SYNC-INV-NN)
// and is gated behind RUN_ACCEPTANCE_TESTS=true (see Makefile: `make test-acceptance`).
package sync
