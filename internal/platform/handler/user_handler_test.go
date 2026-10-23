package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testCurrentUserID = "019a0000-0000-7000-8000-0000000000a1"

// serveCurrentUser calls /auth/me as JWTAuthMiddleware leaves it: a user of orgID.
func serveCurrentUser(t *testing.T, orgRepo *mocks.MockOrganizationRepository, orgID string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	userRepo := mocks.NewMockUserRepository(t)
	userRoleRepo := mocks.NewMockUserRoleRepository(t)
	userRepo.EXPECT().FindByID(mock.Anything, testCurrentUserID).Return(&dto.User{ID: testCurrentUserID, Username: "cajero", Name: "Cajero"}, nil)
	userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, testCurrentUserID).Return([]*dto.Role{{ID: 1, Name: "server"}}, nil)

	userService := service.NewUserService(userRepo, mocks.NewMockRoleRepository(t), userRoleRepo, orgRepo, service.NewJWTService("test-secret", ""), immediateUnitOfWork{}, "")
	organizationService := service.NewOrganizationService(orgRepo, mocks.NewMockEInvoicingSettingsRepository(t), immediateUnitOfWork{})
	h := NewUserHandler(userService, organizationService, slog.Default())

	router := gin.New()
	router.GET("/api/auth/me", func(c *gin.Context) {
		c.Set("user_id", testCurrentUserID)
		c.Set("org_id", orgID)
		h.GetCurrentUserHandler(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	return rec
}

func TestUserHandler_GetCurrentUser_IncludesTheOrganization(t *testing.T) {
	orgRepo := mocks.NewMockOrganizationRepository(t)
	displayName := "Laguna Escondida"
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{
		ID: testOrganizationID, Slug: "laguna-escondida", Name: "Laguna Escondida, Restaurante y Pesca S.A.S",
		Status: dto.OrganizationStatusActive, Timezone: "America/Bogota", Country: "CO", Locale: "es-CO", Currency: "COP",
		DisplayName: &displayName,
		EInvoicing:  &dto.EInvoicingSettings{InvoicePrefix: "LAG", SupportDocumentPrefix: "DS"},
	}, nil)

	rec := serveCurrentUser(t, orgRepo, testOrganizationID)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	assert.JSONEq(t, `"cajero"`, string(body["username"]))
	assert.JSONEq(t, `{
		"id": "`+testOrganizationID+`",
		"slug": "laguna-escondida",
		"name": "Laguna Escondida, Restaurante y Pesca S.A.S",
		"timezone": "America/Bogota", "country": "CO", "locale": "es-CO", "currency": "COP",
		"legal_name": null, "nit": null, "address": null, "phone": null,
		"ticket_footer": null, "ticket_legal_notice": null,
		"display_name": "Laguna Escondida", "logo_url": null,
		"primary_color": null, "secondary_color": null
	}`, string(body["organization"]))
}

func TestUserHandler_GetCurrentUser_MissingOrganizationReturns500(t *testing.T) {
	orgRepo := mocks.NewMockOrganizationRepository(t)
	orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, domainError.ErrOrganizationNotFound)

	rec := serveCurrentUser(t, orgRepo, testOrganizationID)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "not found")
}

// The organization is the token's, not the deployment's.
func TestUserHandler_GetCurrentUser_SecondOrganization(t *testing.T) {
	const orgB = "019a0000-0000-7000-8000-00000000000b"
	orgRepo := mocks.NewMockOrganizationRepository(t)
	orgRepo.EXPECT().Get(mock.Anything, orgB).Return(&dto.Organization{
		ID: orgB, Slug: "org-b", Name: "Org B", Status: dto.OrganizationStatusActive,
		Timezone: "America/Bogota", Country: "CO", Locale: "es-CO", Currency: "COP",
	}, nil)

	rec := serveCurrentUser(t, orgRepo, orgB)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	var org map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["organization"], &org))
	assert.JSONEq(t, `"`+orgB+`"`, string(org["id"]))
	assert.JSONEq(t, `"org-b"`, string(org["slug"]))
}
