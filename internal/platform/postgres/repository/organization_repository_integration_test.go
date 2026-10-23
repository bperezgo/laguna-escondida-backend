package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOrganization(t *testing.T, db *Database, name string) *dto.Organization {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC().Truncate(time.Microsecond)
	footer := "Gracias por su visita"
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM invoice_sequences WHERE organization_id = ?", id)
		db.DB.Exec("DELETE FROM organization_einvoicing WHERE organization_id = ?", id)
		db.DB.Exec("DELETE FROM organizations WHERE id = ?", id)
	})
	return &dto.Organization{
		ID: id, Name: name, Slug: "org-" + id, Status: dto.OrganizationStatusActive,
		Timezone: "America/Bogota", Country: "CO", Locale: "es-CO", Currency: "COP",
		TicketFooter: &footer, CreatedAt: now, UpdatedAt: now,
	}
}

func TestOrganizationRepository_CreateAndGet_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	ctx := context.Background()
	org := newTestOrganization(t, db, "Create and get")

	require.NoError(t, repo.Create(ctx, org))

	got, err := repo.Get(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.Slug, got.Slug)
	assert.Equal(t, "America/Bogota", got.Timezone)
	require.NotNil(t, got.TicketFooter)
	assert.Equal(t, "Gracias por su visita", *got.TicketFooter)
	assert.Nil(t, got.LegalName)
	assert.Nil(t, got.EInvoicing, "no prefixes configured")
	assert.True(t, org.CreatedAt.Equal(got.CreatedAt))
}

func TestOrganizationRepository_Create_DuplicateSlug_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	ctx := context.Background()
	org := newTestOrganization(t, db, "Duplicate slug")
	org.Slug = "laguna-escondida"

	err := repo.Create(ctx, org)

	assert.ErrorIs(t, err, domainError.ErrOrganizationSlugTaken)
	_, err = repo.Get(ctx, org.ID)
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationRepository_Get_NotFound_Integration(t *testing.T) {
	db := newApplierTestDB(t)

	repo := NewOrganizationRepository(db.DB)

	_, err := repo.Get(context.Background(), uuid.NewString())
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)

	_, err = repo.Get(context.Background(), "not-a-uuid")
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound, "a malformed id is not found, not a database error")

	name := "X"
	_, err = repo.Update(context.Background(), "not-a-uuid", &dto.UpdateOrganizationRequest{Name: &name})
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationRepository_Get_LagunaWithEInvoicing_Integration(t *testing.T) {
	db := newApplierTestDB(t)

	laguna, err := NewOrganizationRepository(db.DB).Get(context.Background(), lagunaOrgID)

	require.NoError(t, err)
	assert.Equal(t, "laguna-escondida", laguna.Slug)
	require.NotNil(t, laguna.EInvoicing)
	assert.Equal(t, "LAG", laguna.EInvoicing.InvoicePrefix)
	assert.Equal(t, "DS", laguna.EInvoicing.SupportDocumentPrefix)
}

func TestOrganizationRepository_Update_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	ctx := context.Background()
	org := newTestOrganization(t, db, "Update")
	require.NoError(t, repo.Create(ctx, org))

	displayName, suspended := "Patched", dto.OrganizationStatusSuspended
	got, err := repo.Update(ctx, org.ID, &dto.UpdateOrganizationRequest{DisplayName: &displayName, Status: &suspended})
	require.NoError(t, err)
	require.NotNil(t, got.DisplayName)
	assert.Equal(t, "Patched", *got.DisplayName)
	assert.Equal(t, dto.OrganizationStatusSuspended, got.Status)
	assert.Equal(t, org.Name, got.Name, "fields not in the request are unchanged")
	require.NotNil(t, got.TicketFooter)
	assert.True(t, got.UpdatedAt.After(org.UpdatedAt))

	empty := ""
	cleared, err := repo.Update(ctx, org.ID, &dto.UpdateOrganizationRequest{TicketFooter: &empty})
	require.NoError(t, err)
	assert.Nil(t, cleared.TicketFooter, "an empty optional field is stored as NULL")
	require.NotNil(t, cleared.DisplayName)

	_, err = repo.Update(ctx, uuid.NewString(), &dto.UpdateOrganizationRequest{DisplayName: &displayName})
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationRepository_List_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	settingsRepo := NewEInvoicingSettingsRepository(db.DB)
	ctx := context.Background()
	withPrefixes := newTestOrganization(t, db, "AAA list with prefixes")
	suspended := newTestOrganization(t, db, "AAB list suspended")
	suspended.Status = dto.OrganizationStatusSuspended
	require.NoError(t, repo.Create(ctx, withPrefixes))
	require.NoError(t, repo.Create(ctx, suspended))
	require.NoError(t, settingsRepo.Upsert(ctx, &dto.EInvoicingSettings{
		OrganizationID: withPrefixes.ID, InvoicePrefix: "LST", SupportDocumentPrefix: "LSD",
	}))

	orgs, err := repo.List(ctx)
	require.NoError(t, err)

	byID := map[string]*dto.Organization{}
	for i, org := range orgs {
		byID[org.ID] = org
		if i > 0 {
			assert.LessOrEqual(t, orgs[i-1].Name, org.Name, "ordered by name")
		}
	}
	require.Contains(t, byID, withPrefixes.ID)
	require.Contains(t, byID, suspended.ID)
	require.Contains(t, byID, lagunaOrgID)
	require.NotNil(t, byID[withPrefixes.ID].EInvoicing)
	assert.Equal(t, "LST", byID[withPrefixes.ID].EInvoicing.InvoicePrefix)
	assert.Nil(t, byID[suspended.ID].EInvoicing)
	assert.Equal(t, dto.OrganizationStatusSuspended, byID[suspended.ID].Status)
}

func TestEInvoicingSettingsRepository_Upsert_ReplaceKeepsOldCounter_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	settingsRepo := NewEInvoicingSettingsRepository(db.DB)
	billRepo := NewBillRepository(db.DB)
	ctx := context.Background()
	orgID := seedOrganization(t, db)

	require.NoError(t, settingsRepo.Upsert(ctx, &dto.EInvoicingSettings{OrganizationID: orgID, InvoicePrefix: "OLD", SupportDocumentPrefix: "DSO"}))
	_, err := billRepo.GetNextConsecutive(ctx, orgID, "OLD")
	require.NoError(t, err)
	_, err = billRepo.GetNextConsecutive(ctx, orgID, "OLD")
	require.NoError(t, err)

	require.NoError(t, settingsRepo.Upsert(ctx, &dto.EInvoicingSettings{OrganizationID: orgID, InvoicePrefix: "NEW", SupportDocumentPrefix: "DSN"}))

	got, err := settingsRepo.Get(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "NEW", got.InvoicePrefix)
	assert.Equal(t, "DSN", got.SupportDocumentPrefix)
	first, err := billRepo.GetNextConsecutive(ctx, orgID, "NEW")
	require.NoError(t, err)
	assert.Equal(t, 1, first, "a new prefix starts at 1")
	var oldLast int
	require.NoError(t, db.DB.Raw("SELECT last_consecutive FROM invoice_sequences WHERE organization_id = ? AND prefix = 'OLD'", orgID).Scan(&oldLast).Error)
	assert.Equal(t, 2, oldLast, "the old prefix's counter is kept")
}

func TestEInvoicingSettingsRepository_Upsert_OnPinnedConnection_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	settingsRepo := NewEInvoicingSettingsRepository(db.DB)
	orgID := seedOrganization(t, db)
	ctx, release := postgres.WithSession(context.Background(), db.DB, orgID)
	defer release()

	rollback := errors.New("rollback")
	err := postgres.NewUnitOfWork(db.DB).Do(ctx, func(ctx context.Context) error {
		require.NoError(t, settingsRepo.Upsert(ctx, &dto.EInvoicingSettings{OrganizationID: orgID, InvoicePrefix: "TX", SupportDocumentPrefix: "DTX"}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	_, err = settingsRepo.Get(ctx, orgID)
	assert.ErrorIs(t, err, domainError.ErrEInvoicingNotConfigured, "the upsert ran inside the unit of work")

	require.NoError(t, settingsRepo.Upsert(ctx, &dto.EInvoicingSettings{OrganizationID: orgID, InvoicePrefix: "PIN", SupportDocumentPrefix: "DPN"}))
	got, err := settingsRepo.Get(context.Background(), orgID)
	require.NoError(t, err)
	assert.Equal(t, "PIN", got.InvoicePrefix, "a write on the pinned connection is committed")
}

func TestOrganizationRepository_CreateWithInvalidPrefixesRollsBack_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOrganizationRepository(db.DB)
	settingsRepo := NewEInvoicingSettingsRepository(db.DB)
	ctx := context.Background()
	org := newTestOrganization(t, db, "Rolled back")

	err := postgres.NewUnitOfWork(db.DB).Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, org); err != nil {
			return err
		}
		return settingsRepo.Upsert(ctx, &dto.EInvoicingSettings{OrganizationID: org.ID, InvoicePrefix: "SAME", SupportDocumentPrefix: "SAME"})
	})

	require.ErrorContains(t, err, "organization_einvoicing_distinct_prefixes")
	_, err = repo.Get(ctx, org.ID)
	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound, "the organization is not stored without its prefixes")
}
