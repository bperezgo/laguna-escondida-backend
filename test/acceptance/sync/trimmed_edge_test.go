package sync

import (
	"encoding/json"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rig's edge runs the edge migration set, so it is trimmed: no suppliers or purchase
// entries. The cloud keeps serving boxes that haven't updated, and this box must sync with it.

func skipUnlessTrimmed(t *testing.T) {
	t.Helper()
	if edgeUntrimmed() {
		t.Skip("the edge database is untrimmed (ACCEPTANCE_EDGE_UNTRIMMED)")
	}
}

func (r *rig) edgeHasTable(name string) bool {
	r.t.Helper()
	var present bool
	require.NoError(r.t, r.edgeDB.Raw(`SELECT to_regclass(?) IS NOT NULL`, "public."+name).Scan(&present).Error)
	return present
}

func (r *rig) seedCloudSupplier(changedAt time.Time) string {
	r.t.Helper()
	id := uuid.NewString()
	require.NoError(r.t, r.cloudRef.UpsertSuppliers(r.ctx, []dto.SupplierSyncPayload{{
		ID: id, Name: "Frutas SAS", CreatedAt: changedAt, UpdatedAt: changedAt,
	}}), "seed cloud supplier")
	return id
}

func TestSync_TrimmedEdge_PullIgnoresSuppliersAndAppliesTheRest(t *testing.T) {
	skipUnlessTrimmed(t)
	r := newRig(t)
	require.False(t, r.edgeHasTable("suppliers"), "the rig's edge is trimmed")
	now := time.Now().UTC().Truncate(time.Microsecond)
	r.seedCloudSupplier(now)
	product := newProduct("SKU-TRIM", "Limonada", now)
	user := newUser("mesero", "$2a$10$abcdefghijklmnopqrstuvexamplehashvalue00000000000000", now)
	r.seedCloudProducts(product)
	r.seedCloudUsers(user)

	res := r.pull()

	assert.Equal(t, 1, res.Suppliers, "the cloud still sends suppliers")
	_, hasProduct := r.edgeProductByID(product.ID)
	assert.True(t, hasProduct)
	_, hasUser := r.edgeUserByID(user.ID)
	assert.True(t, hasUser)
	require.NotNil(t, r.edgePulledCursor(), "the cursor advances")
}

func TestSync_TrimmedEdge_PushesAPurchaseEntryQueuedBeforeTheUpdate(t *testing.T) {
	skipUnlessTrimmed(t)
	r := newRig(t)
	require.False(t, r.edgeHasTable("purchase_entries"), "the rig's edge is trimmed")
	now := time.Now().UTC().Truncate(time.Microsecond)
	supplierID := r.seedCloudSupplier(now)
	product := newProduct("SKU-PE", "Limones", now)
	r.seedCloudProducts(product)

	// The outbox row carries the whole entry, so it pushes although its source table is gone.
	entryID := uuid.NewString()
	raw, err := json.Marshal(dto.PurchaseEntry{
		ID: entryID, SupplierID: supplierID, TotalAmount: decimal.NewFromInt(12000), EntryDate: now, CreatedAt: now,
		Items: []*dto.PurchaseEntryItem{{
			ID: uuid.NewString(), PurchaseEntryID: entryID, ProductID: product.ID,
			Quantity: decimal.NewFromInt(3), UnitCost: decimal.NewFromInt(4000), TotalCost: decimal.NewFromInt(12000),
		}},
	})
	require.NoError(t, err)
	r.appendEdgeOutbox(&dto.SyncOutboxEntry{
		OpID: uuid.NewString(), OriginNodeID: r.identity.NodeID, EntityType: dto.SyncEntityPurchaseEntry,
		EntityID: entryID, Operation: dto.SyncOperationCreate, Payload: raw,
	})

	res := r.push()

	assert.Equal(t, 1, res.PushedOps)
	assert.EqualValues(t, 1, r.cloudCount("purchase_entries", "id = ?", entryID), "the cloud applies it as before")
	assert.EqualValues(t, 1, r.cloudCount("purchase_entry_items", "purchase_entry_id = ?", entryID))
	assert.EqualValues(t, 0, r.edgeCount("sync_outbox", "entity_id = ? AND synced_at IS NULL", entryID), "the op is confirmed")
}

func TestSync_TrimmedEdge_OrganizationLookupSkipsEInvoicing(t *testing.T) {
	skipUnlessTrimmed(t)
	r := newRig(t)
	require.False(t, r.edgeHasTable("organization_einvoicing"), "the rig's edge is trimmed")
	repo := repository.NewEdgeOrganizationRepository(r.edgeDB)

	byID, err := repo.Get(r.ctx, lagunaOrgID)
	require.NoError(t, err, "sign-in without a slug loads the box's organization")
	assert.Nil(t, byID.EInvoicing)

	bySlug, err := repo.GetBySlug(r.ctx, "laguna-escondida")
	require.NoError(t, err, "sign-in with a slug looks the organization up")
	assert.Equal(t, lagunaOrgID, bySlug.ID)

	orgs, err := repo.List(r.ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, orgs)
}
