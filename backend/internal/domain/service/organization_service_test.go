package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type inUnitOfWorkKey struct{}

// Marks the context so a test can assert a write happened inside the unit of work.
type markingUnitOfWork struct{}

func (markingUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(context.WithValue(ctx, inUnitOfWorkKey{}, true))
}

func inUnitOfWork(ctx context.Context) bool {
	marked, _ := ctx.Value(inUnitOfWorkKey{}).(bool)
	return marked
}

func createTestOrganizationService(t *testing.T) (*OrganizationService, *mocks.MockOrganizationRepository, *mocks.MockEInvoicingSettingsRepository) {
	orgRepo := mocks.NewMockOrganizationRepository(t)
	einvoicingRepo := mocks.NewMockEInvoicingSettingsRepository(t)
	return NewOrganizationService(orgRepo, einvoicingRepo, markingUnitOfWork{}), orgRepo, einvoicingRepo
}

func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	var validationErr *domainError.OrganizationValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, field, validationErr.Field)
}

func TestOrganizationService_Create_MinimalGetsDefaults(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	var stored *dto.Organization
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, org *dto.Organization) error {
		assert.True(t, inUnitOfWork(ctx))
		stored = org
		return nil
	})

	org, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{Name: "  Restaurante X S.A.S ", Slug: "restaurante-x"})

	require.NoError(t, err)
	assert.Same(t, stored, org)
	parsed, err := uuid.Parse(org.ID)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())
	assert.Equal(t, "Restaurante X S.A.S", org.Name)
	assert.Equal(t, "restaurante-x", org.Slug)
	assert.Equal(t, dto.OrganizationStatusActive, org.Status)
	assert.Equal(t, "America/Bogota", org.Timezone)
	assert.Equal(t, "CO", org.Country)
	assert.Equal(t, "es-CO", org.Locale)
	assert.Equal(t, "COP", org.Currency)
	assert.Nil(t, org.LegalName)
	assert.Nil(t, org.DisplayName)
	assert.Nil(t, org.EInvoicing)
	assert.False(t, org.CreatedAt.IsZero())
}

func TestOrganizationService_Create_WithAllSettings(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	org, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{
		Name: "Restaurante X", Slug: "rx1", Status: "suspended", Timezone: "America/Mexico_City",
		Country: "MX", Locale: "es-MX", Currency: "MXN", LegalName: "Restaurante X S.A. de C.V.",
		NIT: "900123456-7", Address: "Calle 1", Phone: "+52 55 1234 5678", TicketFooter: "Gracias",
		TicketLegalNotice: "Régimen común", DisplayName: "RX", LogoURL: "https://cdn.example.com/rx.png",
		PrimaryColor: "#1A2B3C", SecondaryColor: "#ffffff",
	})

	require.NoError(t, err)
	assert.Equal(t, "suspended", org.Status)
	assert.Equal(t, "America/Mexico_City", org.Timezone)
	assert.Equal(t, "MXN", org.Currency)
	require.NotNil(t, org.LogoURL)
	assert.Equal(t, "https://cdn.example.com/rx.png", *org.LogoURL)
	require.NotNil(t, org.SecondaryColor)
	assert.Equal(t, "#ffffff", *org.SecondaryColor)
}

func TestOrganizationService_Create_WithPrefixesUpsertsInsideUnitOfWork(t *testing.T) {
	svc, orgRepo, einvoicingRepo := createTestOrganizationService(t)
	var orgID string
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, org *dto.Organization) error {
		orgID = org.ID
		return nil
	})
	einvoicingRepo.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, settings *dto.EInvoicingSettings) error {
		assert.True(t, inUnitOfWork(ctx), "the prefixes are stored with the organization")
		assert.Equal(t, orgID, settings.OrganizationID)
		assert.Equal(t, "RX", settings.InvoicePrefix)
		assert.Equal(t, "DSX", settings.SupportDocumentPrefix)
		return nil
	})

	org, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{
		Name: "Restaurante X", Slug: "restaurante-x",
		EInvoicing: &dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"},
	})

	require.NoError(t, err)
	require.NotNil(t, org.EInvoicing)
	assert.Equal(t, "RX", org.EInvoicing.InvoicePrefix)
}

func TestOrganizationService_Create_UpsertFailureFailsTheCreate(t *testing.T) {
	svc, orgRepo, einvoicingRepo := createTestOrganizationService(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	einvoicingRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(errors.New("db down"))

	_, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{
		Name: "Restaurante X", Slug: "restaurante-x",
		EInvoicing: &dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"},
	})

	assert.ErrorContains(t, err, "db down")
}

func TestOrganizationService_Create_SlugTaken(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(domainError.ErrOrganizationSlugTaken)

	_, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{Name: "Laguna", Slug: "laguna-escondida"})

	assert.ErrorIs(t, err, domainError.ErrOrganizationSlugTaken)
}

func TestOrganizationService_Create_InvalidFieldsStoreNothing(t *testing.T) {
	valid := func() dto.CreateOrganizationRequest {
		return dto.CreateOrganizationRequest{Name: "Restaurante X", Slug: "restaurante-x"}
	}
	tests := []struct {
		name   string
		modify func(*dto.CreateOrganizationRequest)
		field  string
	}{
		{"empty name", func(r *dto.CreateOrganizationRequest) { r.Name = "" }, "name"},
		{"blank name", func(r *dto.CreateOrganizationRequest) { r.Name = "   " }, "name"},
		{"name too long", func(r *dto.CreateOrganizationRequest) { r.Name = strings.Repeat("a", 256) }, "name"},
		{"empty slug", func(r *dto.CreateOrganizationRequest) { r.Slug = "" }, "slug"},
		{"slug too short", func(r *dto.CreateOrganizationRequest) { r.Slug = "rx" }, "slug"},
		{"slug too long", func(r *dto.CreateOrganizationRequest) { r.Slug = strings.Repeat("a", 41) }, "slug"},
		{"slug uppercase", func(r *dto.CreateOrganizationRequest) { r.Slug = "Restaurante-x" }, "slug"},
		{"slug leading hyphen", func(r *dto.CreateOrganizationRequest) { r.Slug = "-restaurante" }, "slug"},
		{"slug trailing hyphen", func(r *dto.CreateOrganizationRequest) { r.Slug = "restaurante-" }, "slug"},
		{"slug with space", func(r *dto.CreateOrganizationRequest) { r.Slug = "rest x" }, "slug"},
		{"unknown status", func(r *dto.CreateOrganizationRequest) { r.Status = "deleted" }, "status"},
		{"misspelled timezone", func(r *dto.CreateOrganizationRequest) { r.Timezone = "America/Bogata" }, "timezone"},
		{"local timezone", func(r *dto.CreateOrganizationRequest) { r.Timezone = "Local" }, "timezone"},
		{"lowercase country", func(r *dto.CreateOrganizationRequest) { r.Country = "co" }, "country"},
		{"three-letter country", func(r *dto.CreateOrganizationRequest) { r.Country = "COL" }, "country"},
		{"locale without region", func(r *dto.CreateOrganizationRequest) { r.Locale = "es" }, "locale"},
		{"locale with underscore", func(r *dto.CreateOrganizationRequest) { r.Locale = "es_CO" }, "locale"},
		{"lowercase currency", func(r *dto.CreateOrganizationRequest) { r.Currency = "cop" }, "currency"},
		{"two-letter currency", func(r *dto.CreateOrganizationRequest) { r.Currency = "CO" }, "currency"},
		{"logo over http", func(r *dto.CreateOrganizationRequest) { r.LogoURL = "http://cdn.example.com/logo.png" }, "logo_url"},
		{"relative logo", func(r *dto.CreateOrganizationRequest) { r.LogoURL = "/logo.png" }, "logo_url"},
		{"named color", func(r *dto.CreateOrganizationRequest) { r.PrimaryColor = "blue" }, "primary_color"},
		{"short hex color", func(r *dto.CreateOrganizationRequest) { r.SecondaryColor = "#fff" }, "secondary_color"},
		{"legal name too long", func(r *dto.CreateOrganizationRequest) { r.LegalName = strings.Repeat("a", 256) }, "legal_name"},
		{"nit too long", func(r *dto.CreateOrganizationRequest) { r.NIT = strings.Repeat("1", 256) }, "nit"},
		{"address too long", func(r *dto.CreateOrganizationRequest) { r.Address = strings.Repeat("a", 256) }, "address"},
		{"phone too long", func(r *dto.CreateOrganizationRequest) { r.Phone = strings.Repeat("1", 256) }, "phone"},
		{"display name too long", func(r *dto.CreateOrganizationRequest) { r.DisplayName = strings.Repeat("a", 256) }, "display_name"},
		{"ticket footer too long", func(r *dto.CreateOrganizationRequest) { r.TicketFooter = strings.Repeat("a", 1001) }, "ticket_footer"},
		{"ticket legal notice too long", func(r *dto.CreateOrganizationRequest) { r.TicketLegalNotice = strings.Repeat("a", 1001) }, "ticket_legal_notice"},
		{"equal prefixes", func(r *dto.CreateOrganizationRequest) {
			r.EInvoicing = &dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "RX"}
		}, "support_document_prefix"},
		{"lowercase prefix", func(r *dto.CreateOrganizationRequest) {
			r.EInvoicing = &dto.SetEInvoicingRequest{InvoicePrefix: "rx", SupportDocumentPrefix: "DSX"}
		}, "invoice_prefix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := createTestOrganizationService(t)
			req := valid()
			tt.modify(&req)

			_, err := svc.Create(context.Background(), &req)

			assertValidationError(t, err, tt.field)
		})
	}
}

func TestOrganizationService_Create_LengthLimitsAreInclusive(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	_, err := svc.Create(context.Background(), &dto.CreateOrganizationRequest{
		Name: strings.Repeat("ñ", 255), Slug: strings.Repeat("a", 40),
		TicketFooter: strings.Repeat("ñ", 1000), Address: strings.Repeat("a", 255),
	})

	assert.NoError(t, err)
}

func TestOrganizationService_Update_PartialPassesOnlyGivenFields(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	updated := &dto.Organization{ID: "org-1", DisplayName: strPtr("Laguna")}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", &dto.UpdateOrganizationRequest{DisplayName: strPtr("Laguna")}).Return(updated, nil)

	org, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{DisplayName: strPtr("Laguna")})

	require.NoError(t, err)
	assert.Same(t, updated, org)
}

func TestOrganizationService_Update_TrimsName(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Update(mock.Anything, "org-1", &dto.UpdateOrganizationRequest{Name: strPtr("Laguna")}).Return(&dto.Organization{}, nil)

	_, err := svc.Update(context.Background(), "org-1", &dto.UpdateOrganizationRequest{Name: strPtr(" Laguna ")})

	assert.NoError(t, err)
}

func TestOrganizationService_Update_EmptyOptionalFieldClearsIt(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	req := &dto.UpdateOrganizationRequest{TicketFooter: strPtr(""), LogoURL: strPtr(""), PrimaryColor: strPtr("")}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{ID: "org-1"}, nil)

	org, err := svc.Update(context.Background(), "org-1", req)

	require.NoError(t, err)
	assert.Nil(t, org.TicketFooter)
}

func TestOrganizationService_Update_Suspend(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	req := &dto.UpdateOrganizationRequest{Status: strPtr("suspended")}
	orgRepo.EXPECT().Update(mock.Anything, "org-1", req).Return(&dto.Organization{Status: "suspended"}, nil)

	org, err := svc.Update(context.Background(), "org-1", req)

	require.NoError(t, err)
	assert.Equal(t, "suspended", org.Status)
}

func TestOrganizationService_Update_InvalidFieldsChangeNothing(t *testing.T) {
	tests := []struct {
		name  string
		req   dto.UpdateOrganizationRequest
		field string
	}{
		{"empty name", dto.UpdateOrganizationRequest{Name: strPtr("")}, "name"},
		{"empty status", dto.UpdateOrganizationRequest{Status: strPtr("")}, "status"},
		{"empty timezone", dto.UpdateOrganizationRequest{Timezone: strPtr("")}, "timezone"},
		{"empty country", dto.UpdateOrganizationRequest{Country: strPtr("")}, "country"},
		{"empty locale", dto.UpdateOrganizationRequest{Locale: strPtr("")}, "locale"},
		{"empty currency", dto.UpdateOrganizationRequest{Currency: strPtr("")}, "currency"},
		{"unknown status", dto.UpdateOrganizationRequest{Status: strPtr("closed")}, "status"},
		{"misspelled timezone", dto.UpdateOrganizationRequest{Timezone: strPtr("America/Bogata")}, "timezone"},
		{"named color", dto.UpdateOrganizationRequest{PrimaryColor: strPtr("blue")}, "primary_color"},
		{"logo over http", dto.UpdateOrganizationRequest{LogoURL: strPtr("http://x.com/a.png")}, "logo_url"},
		{"ticket footer too long", dto.UpdateOrganizationRequest{TicketFooter: strPtr(strings.Repeat("a", 1001))}, "ticket_footer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := createTestOrganizationService(t)

			_, err := svc.Update(context.Background(), "org-1", &tt.req)

			assertValidationError(t, err, tt.field)
		})
	}
}

func TestOrganizationService_Update_NotFound(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Update(mock.Anything, "missing", mock.Anything).Return(nil, domainError.ErrOrganizationNotFound)

	_, err := svc.Update(context.Background(), "missing", &dto.UpdateOrganizationRequest{DisplayName: strPtr("X")})

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationService_SetEInvoicing_StoresPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		existing *dto.EInvoicingSettings
	}{
		{"first configuration", nil},
		{"replace", &dto.EInvoicingSettings{InvoicePrefix: "OLD", SupportDocumentPrefix: "DSO"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, orgRepo, einvoicingRepo := createTestOrganizationService(t)
			orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{ID: "org-1", EInvoicing: tt.existing}, nil)
			want := &dto.EInvoicingSettings{OrganizationID: "org-1", InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"}
			einvoicingRepo.EXPECT().Upsert(mock.Anything, want).Return(nil)

			settings, err := svc.SetEInvoicing(context.Background(), "org-1", &dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"})

			require.NoError(t, err)
			assert.Equal(t, want, settings)
		})
	}
}

func TestOrganizationService_SetEInvoicing_NotFound(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "missing").Return(nil, domainError.ErrOrganizationNotFound)

	_, err := svc.SetEInvoicing(context.Background(), "missing", &dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"})

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationService_SetEInvoicing_UpsertFails(t *testing.T) {
	svc, orgRepo, einvoicingRepo := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{ID: "org-1"}, nil)
	einvoicingRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(errors.New("db down"))

	_, err := svc.SetEInvoicing(context.Background(), "org-1", &dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "DSX"})

	assert.ErrorContains(t, err, "db down")
}

func TestOrganizationService_SetEInvoicing_InvalidPrefixes(t *testing.T) {
	tests := []struct {
		name  string
		req   dto.SetEInvoicingRequest
		field string
	}{
		{"equal prefixes", dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "RX"}, "support_document_prefix"},
		{"lowercase invoice prefix", dto.SetEInvoicingRequest{InvoicePrefix: "rx", SupportDocumentPrefix: "DSX"}, "invoice_prefix"},
		{"empty invoice prefix", dto.SetEInvoicingRequest{InvoicePrefix: "", SupportDocumentPrefix: "DSX"}, "invoice_prefix"},
		{"support prefix too long", dto.SetEInvoicingRequest{InvoicePrefix: "RX", SupportDocumentPrefix: "ABCDEFGHIJK"}, "support_document_prefix"},
		{"prefix with hyphen", dto.SetEInvoicingRequest{InvoicePrefix: "R-X", SupportDocumentPrefix: "DSX"}, "invoice_prefix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := createTestOrganizationService(t)

			_, err := svc.SetEInvoicing(context.Background(), "org-1", &tt.req)

			assertValidationError(t, err, tt.field)
		})
	}
}

func TestOrganizationService_List_PassesThrough(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgs := []*dto.Organization{{ID: "a", Name: "A"}, {ID: "b", Name: "B", Status: "suspended"}}
	orgRepo.EXPECT().List(mock.Anything).Return(orgs, nil)

	got, err := svc.List(context.Background())

	require.NoError(t, err)
	assert.Equal(t, orgs, got)
}

func TestOrganizationService_List_Error(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().List(mock.Anything).Return(nil, errors.New("db down"))

	_, err := svc.List(context.Background())

	assert.ErrorContains(t, err, "db down")
}

func TestOrganizationService_Get_PassesThrough(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	org := &dto.Organization{ID: "org-1"}
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(org, nil)

	got, err := svc.Get(context.Background(), "org-1")

	require.NoError(t, err)
	assert.Same(t, org, got)
}

func TestOrganizationService_Get_NotFound(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "missing").Return(nil, domainError.ErrOrganizationNotFound)

	_, err := svc.Get(context.Background(), "missing")

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}

func TestOrganizationService_GetCurrent_MapsTenantFacingFields(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	footer, color := "Gracias", "#0E7C86"
	orgRepo.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{
		ID: "org-1", Slug: "laguna-escondida", Name: "Laguna", Status: dto.OrganizationStatusSuspended,
		Timezone: "America/Bogota", Country: "CO", Locale: "es-CO", Currency: "COP",
		TicketFooter: &footer, PrimaryColor: &color,
		EInvoicing: &dto.EInvoicingSettings{InvoicePrefix: "LAG", SupportDocumentPrefix: "DS"},
	}, nil)

	got, err := svc.GetCurrent(context.Background(), "org-1")

	require.NoError(t, err)
	assert.Equal(t, &dto.CurrentOrganization{
		ID: "org-1", Slug: "laguna-escondida", Name: "Laguna",
		Timezone: "America/Bogota", Country: "CO", Locale: "es-CO", Currency: "COP",
		TicketFooter: &footer, PrimaryColor: &color,
	}, got)
}

func TestOrganizationService_GetCurrent_NotFound(t *testing.T) {
	svc, orgRepo, _ := createTestOrganizationService(t)
	orgRepo.EXPECT().Get(mock.Anything, "missing").Return(nil, domainError.ErrOrganizationNotFound)

	_, err := svc.GetCurrent(context.Background(), "missing")

	assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
}
