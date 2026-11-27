package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const billingPrivateKey = "prv_test_never_in_a_body"

type billingSetupDeps struct {
	router   *gin.Engine
	orgRepo  *mocks.MockOrganizationRepository
	userRepo *mocks.MockUserRepository
	roles    *mocks.MockUserRoleRepository
	billing  *mocks.MockBillingRepository
	provider *mocks.MockBillingProvider
	jwt      *service.JWTService
}

// newBillingSetupDeps mounts the routes as the cloud does, behind a stand-in for SetupAuthMiddleware.
func newBillingSetupDeps(t *testing.T, cardEntry bool) billingSetupDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d := billingSetupDeps{
		orgRepo: mocks.NewMockOrganizationRepository(t), userRepo: mocks.NewMockUserRepository(t),
		roles: mocks.NewMockUserRoleRepository(t), billing: mocks.NewMockBillingRepository(t),
		provider: mocks.NewMockBillingProvider(t), jwt: service.NewJWTService("test-secret", tenant.AnyOrganization()),
	}
	users := service.NewUserService(d.userRepo, mocks.NewMockRoleRepository(t), d.roles, d.orgRepo, d.jwt, immediateUnitOfWork{}, tenant.AnyOrganization())
	setup := service.NewBillingSetupService(d.billing, d.orgRepo, d.userRepo, d.roles, users, immediateUnitOfWork{})
	if cardEntry {
		setup.EnableCardEntry(d.provider, dto.BillingProviderPublic{PublicKey: "pub_test_abc", APIURL: "https://sandbox.wompi.co/v1"})
	}
	h := NewBillingSetupHandler(setup, slog.New(slog.DiscardHandler))
	d.router = gin.New()
	group := d.router.Group("/api/billing/setup", func(c *gin.Context) {
		c.Set("org_id", testOrganizationID)
		c.Set("user_id", "rosa-id")
	})
	group.GET("", h.StateHandler)
	group.PUT("/selection", h.SelectionHandler)
	if cardEntry {
		group.POST("/card", RateLimitByOrganization(5, time.Hour), h.SaveCardHandler)
	}
	return d
}

func (d billingSetupDeps) do(method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	d.router.ServeHTTP(rec, req)
	return rec
}

func pendingOrganization(plan string) *dto.Organization {
	return &dto.Organization{ID: testOrganizationID, Name: "Doña Rosa", Status: "pending", Plan: plan}
}

func (d billingSetupDeps) rosaLeavesACard() {
	email := "rosa@example.com"
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(pendingOrganization("einvoicing"), nil)
	d.userRepo.EXPECT().FindByID(mock.Anything, "rosa-id").Return(&dto.User{ID: "rosa-id", Username: email, Email: &email, Active: true}, nil)
}

const cardBody = `{"card_token":"tok_test_1","acceptance_token":"acc-1","personal_data_acceptance_token":"acc-2"}`

func TestBillingSetupHandler_State_CardEntryOff(t *testing.T) {
	d := newBillingSetupDeps(t, false)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(pendingOrganization("starter"), nil)
	d.billing.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, false, nil)

	rec := d.do(http.MethodGet, "/api/billing/setup", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"organization_name":"Doña Rosa","plan":"starter","period":"monthly","card_entry":false}`, rec.Body.String())
}

func TestBillingSetupHandler_State_CardEntryOn(t *testing.T) {
	d := newBillingSetupDeps(t, true)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(pendingOrganization("einvoicing"), nil)
	d.billing.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.OrganizationBilling{Period: "annual"}, true, nil)
	d.provider.EXPECT().Terms(mock.Anything).Return(dto.WompiMerchantTerms{
		TermsURL: "https://wompi.co/terms.pdf", PersonalDataURL: "https://wompi.co/data.pdf",
		AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2", Email: "merchant@example.com",
	}, nil)

	rec := d.do(http.MethodGet, "/api/billing/setup", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{
		"organization_name": "Doña Rosa", "plan": "einvoicing", "period": "annual", "card_entry": true,
		"wompi": {
			"public_key": "pub_test_abc", "api_url": "https://sandbox.wompi.co/v1",
			"terms_url": "https://wompi.co/terms.pdf", "personal_data_url": "https://wompi.co/data.pdf",
			"acceptance_token": "acc-1", "personal_data_acceptance_token": "acc-2"
		}
	}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "prv_")
	assert.NotContains(t, rec.Body.String(), billingPrivateKey)
}

func TestBillingSetupHandler_State_ProviderDown(t *testing.T) {
	d := newBillingSetupDeps(t, true)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(pendingOrganization("einvoicing"), nil)
	d.billing.EXPECT().Get(mock.Anything, testOrganizationID).Return(nil, false, nil)
	d.provider.EXPECT().Terms(mock.Anything).Return(dto.WompiMerchantTerms{}, errors.New("status 500: boom "+billingPrivateKey))

	rec := d.do(http.MethodGet, "/api/billing/setup", "")

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.JSONEq(t, `{"error":"Payment provider unavailable","code":"payment_provider_unavailable"}`, rec.Body.String())
}

func TestBillingSetupHandler_Selection_AnswersTheTablesAmount(t *testing.T) {
	d := newBillingSetupDeps(t, false)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(pendingOrganization("starter"), nil)
	d.orgRepo.EXPECT().Update(mock.Anything, testOrganizationID, mock.Anything).Return(pendingOrganization("full"), nil)
	d.billing.EXPECT().SetPeriod(mock.Anything, testOrganizationID, "annual").Return(nil)

	// The amount a client sends is ignored: 1 peso buys nothing.
	rec := d.do(http.MethodPut, "/api/billing/setup/selection", `{"plan":"full","period":"annual","amount":1}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"plan":"full","period":"annual","amount":1000000,"currency":"COP"}`, rec.Body.String())
	d.orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
}

func TestBillingSetupHandler_Selection_UnknownPeriod(t *testing.T) {
	d := newBillingSetupDeps(t, false)

	rec := d.do(http.MethodPut, "/api/billing/setup/selection", `{"plan":"einvoicing","period":"weekly"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"period"`, string(decodeBody(t, rec)["field"]))
	d.billing.AssertNotCalled(t, "SetPeriod", mock.Anything, mock.Anything, mock.Anything)
}

func TestBillingSetupHandler_SaveCard_SignsThePersonIn(t *testing.T) {
	d := newBillingSetupDeps(t, true)
	d.rosaLeavesACard()
	d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{ID: "3891", Brand: "VISA", LastFour: "4242"}, nil)
	d.billing.EXPECT().SaveCard(mock.Anything, testOrganizationID, mock.Anything).Return(nil)
	d.orgRepo.EXPECT().StartTrial(mock.Anything, testOrganizationID, mock.Anything).Return(&dto.Organization{ID: testOrganizationID, Status: "trial"}, nil)
	d.roles.EXPECT().FindRolesByUserID(mock.Anything, "rosa-id").Return([]*dto.Role{{ID: permissions.RoleAdmin, Name: "admin"}}, nil)

	rec := d.do(http.MethodPost, "/api/billing/setup/card", cardBody)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	for _, field := range []string{"token", "username", "roles", "permissions"} {
		assert.Contains(t, body, field, "the same body as a sign-in")
	}
	assert.NotContains(t, rec.Body.String(), "3891", "the payment source never leaves the cloud")
}

func TestBillingSetupHandler_SaveCard_Failures(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		want string
	}{
		{"refused", domainError.ErrCardRefused, http.StatusPaymentRequired, `{"error":"Card refused","code":"card_refused"}`},
		{"provider down", domainError.ErrPaymentProviderUnavailable, http.StatusBadGateway, `{"error":"Payment provider unavailable","code":"payment_provider_unavailable"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newBillingSetupDeps(t, true)
			d.rosaLeavesACard()
			d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, tc.err)

			rec := d.do(http.MethodPost, "/api/billing/setup/card", cardBody)

			assert.Equal(t, tc.code, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
			d.orgRepo.AssertNotCalled(t, "StartTrial", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestBillingSetupHandler_SaveCard_MissingToken(t *testing.T) {
	d := newBillingSetupDeps(t, true)

	rec := d.do(http.MethodPost, "/api/billing/setup/card", `{"acceptance_token":"acc-1","personal_data_acceptance_token":"acc-2"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `"card_token"`, string(decodeBody(t, rec)["field"]))
}

func TestBillingSetupHandler_SaveCard_TrialAlreadyStarted(t *testing.T) {
	d := newBillingSetupDeps(t, true)
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(&dto.Organization{ID: testOrganizationID, Status: "trial"}, nil)

	rec := d.do(http.MethodPost, "/api/billing/setup/card", cardBody)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `"not_pending"`, string(decodeBody(t, rec)["code"]))
}

func TestBillingSetupHandler_SaveCard_CardEntryOffIsNotFound(t *testing.T) {
	d := newBillingSetupDeps(t, false)

	rec := d.do(http.MethodPost, "/api/billing/setup/card", cardBody)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestBillingSetupHandler_SaveCard_SixthAttemptInAnHourIsRefused(t *testing.T) {
	d := newBillingSetupDeps(t, true)
	email := "rosa@example.com"
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(pendingOrganization("einvoicing"), nil).Times(5)
	d.userRepo.EXPECT().FindByID(mock.Anything, "rosa-id").Return(&dto.User{ID: "rosa-id", Username: email, Email: &email, Active: true}, nil).Times(5)
	d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, domainError.ErrCardRefused).Times(5)

	for attempt := 1; attempt <= 5; attempt++ {
		rec := d.do(http.MethodPost, "/api/billing/setup/card", cardBody)
		require.Equal(t, http.StatusPaymentRequired, rec.Code, "attempt %d", attempt)
	}
	rec := d.do(http.MethodPost, "/api/billing/setup/card", cardBody)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	d.provider.AssertNumberOfCalls(t, "SavePaymentSource", 5)
}
