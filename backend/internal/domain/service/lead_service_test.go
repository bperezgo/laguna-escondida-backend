package service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var leadTestNow = time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)

func newLeadService(t *testing.T, alerts bool) (*LeadService, *mocks.MockLeadRepository) {
	repo := mocks.NewMockLeadRepository(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewLeadService(repo, alerts, logger, func() time.Time { return leadTestNow }), repo
}

func validSubmission() *dto.SubmitLeadRequest {
	return &dto.SubmitLeadRequest{
		Name: "  Ana Ruiz ", RestaurantName: "El Fogón", Phone: "300 123 4567", Consent: true,
		LeadAttribution: dto.LeadAttribution{UTMSource: "facebook", UTMCampaign: "lanzamiento", FBCLID: "abc"},
	}
}

func expectNoDuplicateNoCap(repo *mocks.MockLeadRepository) {
	repo.EXPECT().FindRecentByPhone(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.EXPECT().CountSince(mock.Anything, mock.Anything).Return(0, nil)
}

func assertLeadFieldError(t *testing.T, err error, field string) {
	t.Helper()
	var validationErr *domainError.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, field, validationErr.Field)
}

func TestLeadService_Submit_StoresLeadFromAnAd(t *testing.T) {
	svc, repo := newLeadService(t, true)
	expectNoDuplicateNoCap(repo)
	var stored *dto.Lead
	repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, l *dto.Lead) error {
		stored = l
		return nil
	})

	require.NoError(t, svc.Submit(context.Background(), validSubmission()))

	assert.Equal(t, "Ana Ruiz", stored.Name)
	assert.Equal(t, "+573001234567", stored.Phone)
	assert.Equal(t, dto.LeadStatusNew, stored.Status)
	assert.Equal(t, "facebook", stored.UTMSource)
	assert.Equal(t, "lanzamiento", stored.UTMCampaign)
	assert.Equal(t, "abc", stored.FBCLID)
	assert.Equal(t, leadTestNow, stored.ConsentAt)
	assert.Equal(t, dto.LeadAlertPending, stored.AlertStatus)
	require.NotNil(t, stored.AlertNextAttemptAt)
}

func TestLeadService_Submit_ForeignNumberKeepsItsCountry(t *testing.T) {
	svc, repo := newLeadService(t, false)
	expectNoDuplicateNoCap(repo)
	var stored *dto.Lead
	repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, l *dto.Lead) error {
		stored = l
		return nil
	})
	req := validSubmission()
	req.Phone = "+52 55 1234 5678"

	require.NoError(t, svc.Submit(context.Background(), req))

	assert.Equal(t, "+525512345678", stored.Phone)
}

func TestLeadService_Submit_RejectsInvalidRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*dto.SubmitLeadRequest)
		field  string
	}{
		{"empty name", func(r *dto.SubmitLeadRequest) { r.Name = "   " }, "name"},
		{"long name", func(r *dto.SubmitLeadRequest) { r.Name = strings.Repeat("a", 101) }, "name"},
		{"empty restaurant", func(r *dto.SubmitLeadRequest) { r.RestaurantName = "" }, "restaurant_name"},
		{"long restaurant", func(r *dto.SubmitLeadRequest) { r.RestaurantName = strings.Repeat("a", 121) }, "restaurant_name"},
		{"phone not a number", func(r *dto.SubmitLeadRequest) { r.Phone = "12345" }, "phone"},
		{"empty phone", func(r *dto.SubmitLeadRequest) { r.Phone = "" }, "phone"},
		{"no consent", func(r *dto.SubmitLeadRequest) { r.Consent = false }, "consent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newLeadService(t, true)
			req := validSubmission()
			tt.mutate(req)

			err := svc.Submit(context.Background(), req)

			assertLeadFieldError(t, err, tt.field)
		})
	}
}

func TestLeadService_Submit_DropsInvalidAttributionAndStoresTheLead(t *testing.T) {
	svc, repo := newLeadService(t, true)
	expectNoDuplicateNoCap(repo)
	var stored *dto.Lead
	repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, l *dto.Lead) error {
		stored = l
		return nil
	})
	req := validSubmission()
	req.UTMSource = "google"
	req.LandingURL = "javascript:alert(1)"
	req.Referrer = "https://www.google.com/"
	req.EventID = "bad id!"
	req.UTMTerm = strings.Repeat("x", 256)
	req.GCLID = strings.Repeat("g", 513)

	require.NoError(t, svc.Submit(context.Background(), req))

	assert.Equal(t, "google", stored.UTMSource)
	assert.Empty(t, stored.LandingURL)
	assert.Equal(t, "https://www.google.com/", stored.Referrer)
	assert.Empty(t, stored.EventID)
	assert.Empty(t, stored.UTMTerm)
	assert.Empty(t, stored.GCLID)
}

func TestLeadService_Submit_KeepsValidEventID(t *testing.T) {
	svc, repo := newLeadService(t, true)
	expectNoDuplicateNoCap(repo)
	var stored *dto.Lead
	repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, l *dto.Lead) error {
		stored = l
		return nil
	})
	req := validSubmission()
	req.EventID = "0b9c1c3e-6a1f-4c1e-9d55-3f2a8b7c1d10"
	req.LandingURL = "https://kuenta.com/?utm_source=facebook"

	require.NoError(t, svc.Submit(context.Background(), req))

	assert.Equal(t, req.EventID, stored.EventID)
	assert.Equal(t, req.LandingURL, stored.LandingURL)
}

func TestLeadService_Submit_HoneypotAnswersSuccessAndStoresNothing(t *testing.T) {
	svc, _ := newLeadService(t, true)
	req := validSubmission()
	req.Website = "http://spam.example"

	require.NoError(t, svc.Submit(context.Background(), req))
}

func TestLeadService_Submit_RecentPhoneStoresNothing(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().FindRecentByPhone(mock.Anything, "+573001234567", leadTestNow.Add(-24*time.Hour)).
		Return(&dto.Lead{ID: "existing"}, nil)

	require.NoError(t, svc.Submit(context.Background(), validSubmission()))
}

func TestLeadService_Submit_GlobalCapRejects(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().FindRecentByPhone(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.EXPECT().CountSince(mock.Anything, leadTestNow.Add(-time.Hour)).Return(300, nil)

	err := svc.Submit(context.Background(), validSubmission())

	assert.ErrorIs(t, err, domainError.ErrLeadRateLimited)
}

func TestLeadService_Submit_JustUnderTheCapStores(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().FindRecentByPhone(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.EXPECT().CountSince(mock.Anything, mock.Anything).Return(299, nil)
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	require.NoError(t, svc.Submit(context.Background(), validSubmission()))
}

func TestLeadService_Submit_WithoutAlertDestinationSkipsTheAlert(t *testing.T) {
	svc, repo := newLeadService(t, false)
	expectNoDuplicateNoCap(repo)
	var stored *dto.Lead
	repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, l *dto.Lead) error {
		stored = l
		return nil
	})

	require.NoError(t, svc.Submit(context.Background(), validSubmission()))

	assert.Equal(t, dto.LeadAlertSkipped, stored.AlertStatus)
	assert.Nil(t, stored.AlertNextAttemptAt)
}

func TestLeadService_Submit_RepositoryFailureIsReturned(t *testing.T) {
	svc, repo := newLeadService(t, true)
	expectNoDuplicateNoCap(repo)
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(assert.AnError)

	err := svc.Submit(context.Background(), validSubmission())

	assert.ErrorIs(t, err, assert.AnError)
}

func TestLeadService_List_DefaultsAndClampsPaging(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().List(mock.Anything, dto.ListLeadsFilter{Status: "new", Page: 1, PageSize: 100}).
		Return([]*dto.Lead{{ID: "a"}}, int64(1), nil)

	page, err := svc.List(context.Background(), dto.ListLeadsFilter{Status: "new", Page: 0, PageSize: 5000})

	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, 100, page.PageSize)
}

func TestLeadService_List_RejectsUnknownStatus(t *testing.T) {
	svc, _ := newLeadService(t, true)

	_, err := svc.List(context.Background(), dto.ListLeadsFilter{Status: "maybe"})

	assertLeadFieldError(t, err, "status")
}

func TestLeadService_Update_ChangesStatusAndNotesOnly(t *testing.T) {
	svc, repo := newLeadService(t, true)
	existing := &dto.Lead{
		ID: "l1", Name: "Ana Ruiz", Phone: "+573001234567", Status: dto.LeadStatusNew,
		LeadAttribution: dto.LeadAttribution{UTMSource: "facebook"},
	}
	repo.EXPECT().Get(mock.Anything, "l1").Return(existing, nil)
	repo.EXPECT().Update(mock.Anything, existing).Return(nil)
	status, notes := "contacted", "Demo el jueves"

	got, err := svc.Update(context.Background(), "l1", &dto.UpdateLeadRequest{Status: &status, Notes: &notes})

	require.NoError(t, err)
	assert.Equal(t, dto.LeadStatusContacted, got.Status)
	assert.Equal(t, "Demo el jueves", got.Notes)
	assert.Equal(t, "Ana Ruiz", got.Name)
	assert.Equal(t, "+573001234567", got.Phone)
	assert.Equal(t, "facebook", got.UTMSource)
	assert.Equal(t, leadTestNow, got.UpdatedAt)
}

func TestLeadService_Update_RejectsUnknownStatus(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().Get(mock.Anything, "l1").Return(&dto.Lead{ID: "l1", Status: dto.LeadStatusNew}, nil)
	status := "archived"

	_, err := svc.Update(context.Background(), "l1", &dto.UpdateLeadRequest{Status: &status})

	assertLeadFieldError(t, err, "status")
}

func TestLeadService_Update_RejectsLongNotes(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().Get(mock.Anything, "l1").Return(&dto.Lead{ID: "l1"}, nil)
	notes := strings.Repeat("n", 2001)

	_, err := svc.Update(context.Background(), "l1", &dto.UpdateLeadRequest{Notes: &notes})

	assertLeadFieldError(t, err, "notes")
}

func TestLeadService_Update_UnknownLead(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().Get(mock.Anything, "nope").Return(nil, domainError.ErrLeadNotFound)

	_, err := svc.Update(context.Background(), "nope", &dto.UpdateLeadRequest{})

	assert.ErrorIs(t, err, domainError.ErrLeadNotFound)
}

func TestLeadService_Delete(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().Delete(mock.Anything, "l1").Return(nil)
	repo.EXPECT().Delete(mock.Anything, "nope").Return(domainError.ErrLeadNotFound)

	require.NoError(t, svc.Delete(context.Background(), "l1"))
	assert.ErrorIs(t, svc.Delete(context.Background(), "nope"), domainError.ErrLeadNotFound)
}

func TestLeadService_Get_UnknownLead(t *testing.T) {
	svc, repo := newLeadService(t, true)
	repo.EXPECT().Get(mock.Anything, "nope").Return(nil, domainError.ErrLeadNotFound)

	_, err := svc.Get(context.Background(), "nope")

	assert.ErrorIs(t, err, domainError.ErrLeadNotFound)
}
