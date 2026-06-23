package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pull replicates cloud-owned reference data (products, users, suppliers) down to the
// edge as a cursor diff over updated_at/deleted_at. These tests seed on the cloud, run
// the edge pull loop synchronously, and assert the edge converged.

// SYNC-INV-03 — a new cloud product appears on the edge after a pull.
func TestSync_Pull_ReplicatesNewProduct(t *testing.T) {
	r := newRig(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	p := newProduct("SKU-NEW-1", "Cafe", now)

	r.seedCloudProducts(p)
	res := r.pull()

	assert.Equal(t, 1, res.Products, "one product pulled")
	got, ok := r.edgeProductByID(p.ID)
	require.True(t, ok, "product replicated to edge")
	assert.Equal(t, "Cafe", got.Name)
	assert.Equal(t, "SKU-NEW-1", got.SKU)
}

// SYNC-INV-04 — an update to a cloud product reaches the edge on the next pull.
func TestSync_Pull_ReplicatesUpdate(t *testing.T) {
	r := newRig(t)
	t0 := time.Now().UTC().Truncate(time.Microsecond)
	p := newProduct("SKU-UPD-1", "Cafe", t0)

	r.seedCloudProducts(p)
	r.pull()

	// Same id, new name, later timestamp — the cursor diff must catch it.
	p.Name = "Cafe Renamed"
	p.UpdatedAt = t0.Add(time.Second)
	r.seedCloudProducts(p)
	res := r.pull()

	assert.Equal(t, 1, res.Products, "only the updated product is pulled")
	got, ok := r.edgeProductByID(p.ID)
	require.True(t, ok)
	assert.Equal(t, "Cafe Renamed", got.Name)
}

// SYNC-INV-05 — a soft-delete (tombstone) propagates: the edge row gets deleted_at set,
// it is not hard-deleted.
func TestSync_Pull_ReplicatesTombstone(t *testing.T) {
	r := newRig(t)
	t0 := time.Now().UTC().Truncate(time.Microsecond)
	p := newProduct("SKU-DEL-1", "Cafe", t0)

	r.seedCloudProducts(p)
	r.pull()

	deletedAt := t0.Add(time.Second)
	p.DeletedAt = &deletedAt
	p.UpdatedAt = deletedAt
	r.seedCloudProducts(p)
	r.pull()

	got, ok := r.edgeProductByID(p.ID)
	require.True(t, ok, "row still present (soft-delete, not removed)")
	require.NotNil(t, got.DeletedAt, "edge row carries deleted_at")
	assert.WithinDuration(t, deletedAt, *got.DeletedAt, time.Millisecond)
	// And it is genuinely still a row, not a hard delete.
	assert.Equal(t, int64(1), r.edgeCount("products", "id = ?", p.ID))
}

// SYNC-INV-06 — the replicated user carries its password hash, so the edge can
// authenticate that user with no cloud round-trip (the headline offline guarantee).
func TestSync_Pull_UserCarriesPasswordHash(t *testing.T) {
	r := newRig(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	const hash = "$2a$10$abcdefghijklmnopqrstuvexamplehashvalue000000000000000"
	u := newUser("cashier1", hash, now)

	r.seedCloudUsers(u)
	res := r.pull()

	assert.Equal(t, 1, res.Users, "one user pulled")
	got, ok := r.edgeUserByID(u.ID)
	require.True(t, ok, "user replicated to edge")
	assert.Equal(t, hash, got.Password, "password hash replicated intact")
}

// SYNC-INV-07 — the cursor is monotonic: a pull when nothing changed is a no-op and
// never rewinds last_pulled_cursor.
func TestSync_Pull_CursorDoesNotRewindOnNoOp(t *testing.T) {
	r := newRig(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	r.seedCloudProducts(newProduct("SKU-CUR-1", "Cafe", now))
	r.pull()
	before := r.edgePulledCursor()
	require.NotNil(t, before, "cursor set after first pull")

	res := r.pull() // nothing changed on the cloud
	after := r.edgePulledCursor()

	assert.Equal(t, 0, res.Products, "no-op pull applies nothing")
	require.NotNil(t, after)
	assert.True(t, before.Equal(*after), "cursor unchanged: %s == %s", before, after)
}

// SYNC-INV-08 — pull is incremental: a second pull fetches only rows changed after the
// stored cursor, not the whole table again.
func TestSync_Pull_SecondPullIsIncremental(t *testing.T) {
	r := newRig(t)
	t0 := time.Now().UTC().Truncate(time.Microsecond)

	r.seedCloudProducts(
		newProduct("SKU-INC-1", "Cafe", t0),
		newProduct("SKU-INC-2", "Te", t0),
	)
	first := r.pull()
	require.Equal(t, 2, first.Products, "first pull replicates both")

	later := newProduct("SKU-INC-3", "Jugo", t0.Add(time.Second))
	r.seedCloudProducts(later)
	second := r.pull()

	assert.Equal(t, 1, second.Products, "second pull fetches only the new row")
	got, ok := r.edgeProductByID(later.ID)
	require.True(t, ok)
	assert.Equal(t, "Jugo", got.Name)
}
