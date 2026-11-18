package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLead(t *testing.T, db *Database, phone string, createdAt time.Time) *dto.Lead {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	t.Cleanup(func() { db.DB.Exec("DELETE FROM leads WHERE id = ?", id) })
	next := createdAt
	return &dto.Lead{
		ID: id, Name: "Ana Ruiz", RestaurantName: "El Fogón", Phone: phone, Status: dto.LeadStatusNew,
		ConsentAt: createdAt, CreatedAt: createdAt, UpdatedAt: createdAt,
		LeadAttribution:    dto.LeadAttribution{UTMSource: "facebook", UTMCampaign: "lanzamiento", FBCLID: "abc", EventID: "0b9c1c3e-6a1f"},
		AlertStatus:        dto.LeadAlertPending,
		AlertNextAttemptAt: &next,
	}
}

func TestLeadRepository_CreateAndGet_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	lead := newTestLead(t, db, "+573001230001", time.Now().UTC().Truncate(time.Microsecond))

	require.NoError(t, repo.Create(ctx, lead))

	got, err := repo.Get(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, "Ana Ruiz", got.Name)
	assert.Equal(t, "+573001230001", got.Phone)
	assert.Equal(t, "facebook", got.UTMSource)
	assert.Equal(t, "abc", got.FBCLID)
	assert.Equal(t, "0b9c1c3e-6a1f", got.EventID)
	assert.Empty(t, got.GCLID)
	assert.Equal(t, dto.LeadStatusNew, got.Status)
	assert.True(t, lead.CreatedAt.Equal(got.CreatedAt))
}

func TestLeadRepository_Get_NotFound_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)

	_, err := repo.Get(context.Background(), uuid.Must(uuid.NewV7()).String())

	assert.ErrorIs(t, err, domainError.ErrLeadNotFound)
}

func TestLeadRepository_FindRecentByPhone_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.Create(ctx, newTestLead(t, db, "+573001230002", now.Add(-2*time.Hour))))
	require.NoError(t, repo.Create(ctx, newTestLead(t, db, "+573001230003", now.Add(-48*time.Hour))))

	recent, err := repo.FindRecentByPhone(ctx, "+573001230002", now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.NotNil(t, recent)

	old, err := repo.FindRecentByPhone(ctx, "+573001230003", now.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Nil(t, old)

	none, err := repo.FindRecentByPhone(ctx, "+573009999999", now.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestLeadRepository_CountSince_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	future := time.Now().UTC().Add(100 * 24 * time.Hour).Truncate(time.Microsecond)
	before, err := repo.CountSince(ctx, future.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, newTestLead(t, db, "+573001230004", future)))
	require.NoError(t, repo.Create(ctx, newTestLead(t, db, "+573001230005", future.Add(-3*time.Hour))))

	after, err := repo.CountSince(ctx, future.Add(-time.Hour))
	require.NoError(t, err)

	assert.Equal(t, before+1, after)
}

func TestLeadRepository_ListNewestFirstWithStatusFilter_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	base := time.Now().UTC().Add(200 * 24 * time.Hour).Truncate(time.Microsecond)
	older := newTestLead(t, db, "+573001230006", base)
	newer := newTestLead(t, db, "+573001230007", base.Add(time.Minute))
	contacted := newTestLead(t, db, "+573001230008", base.Add(2*time.Minute))
	contacted.Status = dto.LeadStatusContacted
	for _, l := range []*dto.Lead{older, newer, contacted} {
		require.NoError(t, repo.Create(ctx, l))
	}

	all, total, err := repo.List(ctx, dto.ListLeadsFilter{Page: 1, PageSize: 3})
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.GreaterOrEqual(t, total, int64(3))
	assert.Equal(t, []string{contacted.ID, newer.ID, older.ID}, []string{all[0].ID, all[1].ID, all[2].ID})

	onlyContacted, _, err := repo.List(ctx, dto.ListLeadsFilter{Status: "contacted", Page: 1, PageSize: 50})
	require.NoError(t, err)
	ids := make([]string, 0, len(onlyContacted))
	for _, l := range onlyContacted {
		assert.Equal(t, dto.LeadStatusContacted, l.Status)
		ids = append(ids, l.ID)
	}
	assert.Contains(t, ids, contacted.ID)
	assert.NotContains(t, ids, newer.ID)
}

func TestLeadRepository_UpdateTouchesOnlyStatusAndNotes_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	lead := newTestLead(t, db, "+573001230009", time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, repo.Create(ctx, lead))
	changed := *lead
	changed.Status, changed.Notes, changed.Name, changed.UTMSource = dto.LeadStatusWon, "Demo el jueves", "Otro", "tampered"
	changed.UpdatedAt = lead.UpdatedAt.Add(time.Hour)

	require.NoError(t, repo.Update(ctx, &changed))

	got, err := repo.Get(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, dto.LeadStatusWon, got.Status)
	assert.Equal(t, "Demo el jueves", got.Notes)
	assert.Equal(t, "Ana Ruiz", got.Name)
	assert.Equal(t, "facebook", got.UTMSource)
	assert.ErrorIs(t, repo.Update(ctx, &dto.Lead{ID: uuid.Must(uuid.NewV7()).String()}), domainError.ErrLeadNotFound)
}

func TestLeadRepository_Delete_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	lead := newTestLead(t, db, "+573001230010", time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, repo.Create(ctx, lead))

	require.NoError(t, repo.Delete(ctx, lead.ID))

	_, err := repo.Get(ctx, lead.ID)
	assert.ErrorIs(t, err, domainError.ErrLeadNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, lead.ID), domainError.ErrLeadNotFound)
}

func TestLeadRepository_ClaimDueAlerts_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	now := time.Now().UTC().Add(300 * 24 * time.Hour).Truncate(time.Microsecond)
	due := newTestLead(t, db, "+573001230011", now.Add(-time.Minute))
	notYet := newTestLead(t, db, "+573001230012", now.Add(-time.Minute))
	later := now.Add(time.Hour)
	notYet.AlertNextAttemptAt = &later
	skipped := newTestLead(t, db, "+573001230013", now.Add(-time.Minute))
	skipped.AlertStatus, skipped.AlertNextAttemptAt = dto.LeadAlertSkipped, nil
	for _, l := range []*dto.Lead{due, notYet, skipped} {
		require.NoError(t, repo.Create(ctx, l))
	}

	claimed, err := repo.ClaimDueAlerts(ctx, now, now.Add(2*time.Minute), 50)
	require.NoError(t, err)
	var ids []string
	for _, l := range claimed {
		ids = append(ids, l.ID)
	}
	assert.Contains(t, ids, due.ID)
	assert.NotContains(t, ids, notYet.ID)
	assert.NotContains(t, ids, skipped.ID)

	again, err := repo.ClaimDueAlerts(ctx, now, now.Add(2*time.Minute), 50)
	require.NoError(t, err)
	for _, l := range again {
		assert.NotEqual(t, due.ID, l.ID, "a leased alert is not claimed twice")
	}
	got, err := repo.Get(ctx, due.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.AlertAttempts)
}

func TestLeadRepository_MarkAlert_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewLeadRepository(db.DB)
	ctx := context.Background()
	lead := newTestLead(t, db, "+573001230014", time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, repo.Create(ctx, lead))
	retryAt := lead.CreatedAt.Add(time.Minute)

	require.NoError(t, repo.MarkAlert(ctx, lead.ID, dto.LeadAlertPending, retryAt, "HTTP 503"))
	got, err := repo.Get(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, dto.LeadAlertPending, got.AlertStatus)
	assert.Equal(t, "HTTP 503", got.AlertLastError)
	require.NotNil(t, got.AlertNextAttemptAt)
	assert.True(t, retryAt.Equal(*got.AlertNextAttemptAt))

	require.NoError(t, repo.MarkAlert(ctx, lead.ID, dto.LeadAlertSent, time.Time{}, ""))
	got, err = repo.Get(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, dto.LeadAlertSent, got.AlertStatus)
}
