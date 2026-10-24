package migration

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The cloud keeps sending suppliers for boxes that haven't updated. The same edge build runs on a
// box whose trim was blocked (suppliers kept) and on a trimmed one, and must apply the pull on both.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const edgePullDBName = "laguna_accept_edge_pull"

type stubPullClient struct{ resp *dto.SyncPullResponse }

func (c stubPullClient) Pull(context.Context, time.Time) (*dto.SyncPullResponse, error) {
	return c.resp, nil
}

func pullResponse(cursor time.Time) *dto.SyncPullResponse {
	productID, userID, supplierID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	return &dto.SyncPullResponse{
		Products: []dto.ProductSyncPayload{{
			ID: productID, Name: "Limonada", Category: "drinks", ProductType: "SELLABLE", UnitOfMeasure: "unit",
			Version: 1, UnitPrice: decimal.NewFromInt(8000), VAT: decimal.Zero, VATAmount: decimal.Zero,
			ICO: decimal.Zero, ICOAmount: decimal.Zero, SKU: "SKU-" + productID[:8],
			TotalPriceWithTaxes: decimal.NewFromInt(8000), CreatedAt: cursor, UpdatedAt: cursor,
		}},
		Users: []dto.UserSyncPayload{{
			ID: userID, Username: "mesero-" + userID[:8], Name: "Mesero", Password: "$2a$10$hash",
			RoleIDs: []int{2}, CreatedAt: cursor, UpdatedAt: cursor,
		}},
		Suppliers: []dto.SupplierSyncPayload{{ID: supplierID, Name: "Frutas SAS", CreatedAt: cursor, UpdatedAt: cursor}},
		Cursor:    cursor,
	}
}

func TestEdgePull_AppliesOnTrimmedAndUntrimmedBoxes(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	cases := []struct {
		name          string
		version       uint
		keepsSupplier bool
	}{
		{"untrimmed box", migrations.SharedVersion, true},
		{"trimmed box", trimVersion, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := migrateTo(t, base, edgePullDBName, migrations.Edge, tc.version)
			defer closeGorm(db)

			cloudNodeID := uuid.NewString()
			cursor := time.Now().UTC().Truncate(time.Microsecond)
			resp := pullResponse(cursor)
			state := repository.NewSyncStateRepository(db)
			pull := service.NewSyncPullService(postgres.NewUnitOfWork(db), stubPullClient{resp},
				repository.NewSyncReferenceRepository(db), state,
				dto.SyncIdentity{NodeID: uuid.NewString(), CloudNodeID: cloudNodeID}, slog.New(slog.DiscardHandler))
			orgCtx := tenant.WithOrg(ctx, lagunaOrgID)

			_, err := pull.PullChanges(orgCtx)
			require.NoError(t, err)

			assert.EqualValues(t, 1, countRows(t, db, "products", resp.Products[0].ID))
			assert.EqualValues(t, 1, countRows(t, db, "users", resp.Users[0].ID))
			if tc.keepsSupplier {
				assert.EqualValues(t, 1, countRows(t, db, "suppliers", resp.Suppliers[0].ID))
			}
			pulled, err := state.GetPulledCursor(orgCtx, cloudNodeID)
			require.NoError(t, err)
			require.NotNil(t, pulled, "the cursor advances past the suppliers")
			assert.True(t, pulled.Equal(cursor), "cursor %s, want %s", pulled, cursor)
		})
	}
}

func countRows(t *testing.T, db *gorm.DB, table, id string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Where("id = ?", id).Count(&n).Error)
	return n
}
