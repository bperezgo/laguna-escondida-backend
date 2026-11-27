package handler

import (
	"context"
	"encoding/json"
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

type billingDeps struct {
	router   *gin.Engine
	orgRepo  *mocks.MockOrganizationRepository
	userRepo *mocks.MockUserRepository
	roles    *mocks.MockUserRoleRepository
	billing  *mocks.MockBillingRepository
	provider *mocks.MockBillingProvider
	jwt      *service.JWTService
	service  *service.BillingService
}

// newBillingDeps mounts the admin routes behind the real sign-in and permission middleware, and
// the recovery routes behind a stand-in for SetupAuthMiddleware, sharing one card limit as the cloud does.
func newBillingDeps(t *testing.T, charging bool) billingDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.DiscardHandler)
	d := billingDeps{
		orgRepo: mocks.NewMockOrganizationRepository(t), userRepo: mocks.NewMockUserRepository(t),
		roles: mocks.NewMockUserRoleRepository(t), billing: mocks.NewMockBillingRepository(t),
		provider: mocks.NewMockBillingProvider(t), jwt: service.NewJWTService("test-secret", tenant.AnyOrganization()),
	}
	users := service.NewUserService(d.userRepo, mocks.NewMockRoleRepository(t), d.roles, d.orgRepo, d.jwt, immediateUnitOfWork{}, tenant.AnyOrganization())
	setup := service.NewBillingSetupService(d.billing, d.orgRepo, d.userRepo, d.roles, users, immediateUnitOfWork{})
	setup.EnableCardEntry(d.provider, dto.BillingProviderPublic{PublicKey: "pub_test_abc", APIURL: "https://sandbox.wompi.co/v1"})
	d.service = service.NewBillingService(d.billing, d.orgRepo, d.roles, users, setup, immediateUnitOfWork{}, nil)
	if charging {
		d.service.EnableCharging(service.NewChargeService(d.billing, d.orgRepo, d.provider, immediateUnitOfWork{}, nil, logger))
	}
	h := NewBillingHandler(d.service, logger)
	cardSaves := RateLimitByOrganization(5, time.Hour)
	d.router = gin.New()
	admin := d.router.Group("/api/billing", JWTAuthMiddleware(d.jwt), RequirePermission(permissions.BillingManage))
	admin.GET("", h.ReadHandler)
	admin.GET("/card-setup", h.CardSetupHandler)
	admin.PUT("/card", cardSaves, h.ChangeCardHandler)
	admin.POST("/cancel", h.CancelHandler)
	admin.POST("/resume", h.ResumeHandler)
	recovery := d.router.Group("/api/billing/recovery", func(c *gin.Context) {
		c.Set("org_id", testOrganizationID)
		c.Set("user_id", "rosa-id")
	})
	recovery.GET("", h.RecoveryStateHandler)
	recovery.PUT("/selection", h.RecoverySelectionHandler)
	recovery.POST("/pay", cardSaves, h.RecoveryPayHandler)
	setupHandler := NewBillingSetupHandler(setup, logger)
	d.router.POST("/api/billing/setup/card", func(c *gin.Context) {
		c.Set("org_id", testOrganizationID)
		c.Set("user_id", "rosa-id")
	}, cardSaves, setupHandler.SaveCardHandler)
	return d
}

func (d billingDeps) as(t *testing.T, roleID int, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := d.jwt.GenerateToken("rosa-id", testOrganizationID, "rosa", []int{roleID})
	require.NoError(t, err)
	return d.send(method, path, body, token)
}

func (d billingDeps) send(method, path, body, token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	d.router.ServeHTTP(rec, req)
	return rec
}

func cardOnFile(mutate func(*dto.OrganizationBilling)) *dto.OrganizationBilling {
	provider, source, brand, last := "wompi", "3891", "VISA", "4242"
	billing := &dto.OrganizationBilling{OrganizationID: testOrganizationID, Period: "monthly", Provider: &provider,
		PaymentSourceID: &source, CardBrand: &brand, CardLastFour: &last}
	if mutate != nil {
		mutate(billing)
	}
	return billing
}

func (d billingDeps) organizationIs(org *dto.Organization, billing *dto.OrganizationBilling, charges ...*dto.BillingCharge) {
	d.orgRepo.EXPECT().Get(mock.Anything, testOrganizationID).Return(org, nil).Maybe()
	d.billing.EXPECT().Get(mock.Anything, testOrganizationID).Return(billing, billing != nil, nil).Maybe()
	d.billing.EXPECT().LockSubscription(mock.Anything, testOrganizationID).Return(billing, billing != nil, nil).Maybe()
	d.billing.EXPECT().ListCharges(mock.Anything, testOrganizationID).Return(charges, nil).Maybe()
}

func (d billingDeps) rosaIsTheAdmin() {
	email := "rosa@example.com"
	d.userRepo.EXPECT().FindByID(mock.Anything, "rosa-id").Return(&dto.User{ID: "rosa-id", Username: email, Email: &email, Active: true}, nil).Maybe()
	d.roles.EXPECT().FindRolesByUserID(mock.Anything, "rosa-id").Return([]*dto.Role{{ID: permissions.RoleAdmin, Name: "admin"}}, nil).Maybe()
}

func trialOrganization(endsIn time.Duration) *dto.Organization {
	end := time.Now().Add(endsIn)
	return &dto.Organization{ID: testOrganizationID, Name: "Doña Rosa", Status: "trial", Plan: "einvoicing", TrialEndsAt: &end}
}

func paidOrganization() *dto.Organization {
	return &dto.Organization{ID: testOrganizationID, Name: "Doña Rosa", Status: "active", Plan: "einvoicing"}
}

func paidUntil(in time.Duration) func(*dto.OrganizationBilling) {
	return func(b *dto.OrganizationBilling) { until := time.Now().Add(in); b.PaidUntil = &until }
}

const day = 24 * time.Hour

// "During the trial": the trial's end as the due date, 40000 as the next charge, and the card.
func TestBillingHandler_Read_DuringTheTrial(t *testing.T) {
	d := newBillingDeps(t, true)
	org := trialOrganization(4 * day)
	d.organizationIs(org, cardOnFile(nil))

	rec := d.as(t, permissions.RoleAdmin, http.MethodGet, "/api/billing", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got dto.Subscription
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "einvoicing", got.Plan)
	assert.Equal(t, "monthly", got.Period)
	assert.Equal(t, "trial", got.Status)
	require.NotNil(t, got.DueDate)
	assert.True(t, got.DueDate.Equal(*org.TrialEndsAt))
	require.NotNil(t, got.NextChargeAmount)
	assert.Equal(t, 40000, *got.NextChargeAmount)
	assert.Equal(t, &dto.SavedCard{Brand: "VISA", LastFour: "4242"}, got.Card)
	assert.False(t, got.Cancelled)
	assert.Empty(t, got.Charges)
	assert.NotContains(t, rec.Body.String(), "3891", "the payment source's identifier is never in the body")
	assert.NotContains(t, rec.Body.String(), "payment_source")
	for _, field := range []string{"plan", "period", "status", "due_date", "next_charge_amount", "currency", "cancelled", "card", "latest_charge", "charges"} {
		assert.Contains(t, decodeBody(t, rec), field)
	}
}

func TestBillingHandler_Read_WithoutACard(t *testing.T) {
	d := newBillingDeps(t, true)
	d.organizationIs(paidOrganization(), nil)

	rec := d.as(t, permissions.RoleAdmin, http.MethodGet, "/api/billing", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeBody(t, rec)
	assert.JSONEq(t, `null`, string(got["card"]))
	assert.JSONEq(t, `null`, string(got["next_charge_amount"]))
	assert.JSONEq(t, `[]`, string(got["charges"]))
	assert.JSONEq(t, `"einvoicing"`, string(got["plan"]))
}

// "A manager": 403. Nobody but an admin holds billing:manage.
func TestBillingHandler_AdminRoutes_OnlyAnAdmin(t *testing.T) {
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/billing"}, {http.MethodGet, "/api/billing/card-setup"}, {http.MethodPut, "/api/billing/card"},
		{http.MethodPost, "/api/billing/cancel"}, {http.MethodPost, "/api/billing/resume"},
	}
	for _, role := range []int{permissions.RoleManager, permissions.RoleAccountant, permissions.RoleServer, permissions.RoleAgent} {
		for _, route := range routes {
			d := newBillingDeps(t, true)

			rec := d.as(t, role, route.method, route.path, cardBody)

			assert.Equal(t, http.StatusForbidden, rec.Code, "role %d on %s %s", role, route.method, route.path)
		}
	}
	d := newBillingDeps(t, true)
	assert.Equal(t, http.StatusUnauthorized, d.send(http.MethodGet, "/api/billing", "", "").Code, "no token")
	setup, err := d.jwt.GenerateSetupToken("rosa-id", testOrganizationID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, d.send(http.MethodGet, "/api/billing", "", setup).Code, "a setup token is no sign-in")
}

func TestBillingHandler_CardSetup(t *testing.T) {
	d := newBillingDeps(t, true)
	d.provider.EXPECT().Terms(mock.Anything).Return(dto.WompiMerchantTerms{TermsURL: "https://wompi.co/terms.pdf",
		PersonalDataURL: "https://wompi.co/data.pdf", AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2"}, nil)

	rec := d.as(t, permissions.RoleAdmin, http.MethodGet, "/api/billing/card-setup", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"card_entry":true,"wompi":{"public_key":"pub_test_abc","api_url":"https://sandbox.wompi.co/v1",
		"terms_url":"https://wompi.co/terms.pdf","personal_data_url":"https://wompi.co/data.pdf",
		"acceptance_token":"acc-1","personal_data_acceptance_token":"acc-2"}}`, rec.Body.String())
}

// "New card after a declined charge": the new card is saved and one charge is created at once.
func TestBillingHandler_ChangeCard_AfterADeclinedCharge(t *testing.T) {
	d := newBillingDeps(t, true)
	d.rosaIsTheAdmin()
	billing := cardOnFile(paidUntil(-day))
	due := *billing.PaidUntil
	declined := &dto.BillingCharge{ID: "c1", OrganizationID: testOrganizationID, DueOn: due, Attempt: 1, Reference: "kb-1",
		Plan: "einvoicing", Period: "monthly", Amount: 40000, Currency: "COP", Status: dto.BillingChargeDeclined, CreatedAt: due}
	d.organizationIs(paidOrganization(), billing, declined)
	d.provider.EXPECT().SavePaymentSource(mock.Anything, dto.PaymentSourceRequest{CardToken: "tok_test_1", CustomerEmail: "rosa@example.com",
		AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2"}).Return(dto.PaymentSource{ID: "4000", Brand: "MASTERCARD", LastFour: "5100"}, nil)
	d.billing.EXPECT().SaveCard(mock.Anything, testOrganizationID, mock.MatchedBy(func(card dto.BillingCard) bool {
		return card.PaymentSourceID == "4000" && card.Brand == "MASTERCARD" && card.LastFour == "5100" && card.Provider == "wompi"
	})).Return(nil)
	var inserted *dto.BillingCharge
	d.billing.EXPECT().InsertCharge(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c *dto.BillingCharge) error {
		inserted = c
		return nil
	})
	d.provider.EXPECT().Charge(mock.Anything, mock.MatchedBy(func(req dto.BillingChargeRequest) bool {
		return req.AmountInCents == 4000000 && req.Currency == "COP" && req.CustomerEmail == "rosa@example.com"
	})).Return(dto.BillingTransaction{ID: "tx_1", Status: dto.BillingChargePending, AmountInCents: 4000000, Currency: "COP"}, nil)
	d.billing.EXPECT().LockChargeByReference(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string) (*dto.BillingCharge, bool, error) { return inserted, true, nil })
	d.billing.EXPECT().UpdateCharge(mock.Anything, mock.Anything).Return(nil)

	rec := d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, inserted, "one charge for the unpaid period")
	assert.Equal(t, 0, inserted.Attempt)
	assert.Equal(t, 40000, inserted.Amount)
	assert.True(t, inserted.DueOn.Equal(due))
	assert.Contains(t, decodeBody(t, rec), "charges", "the same body as GET /api/billing")
}

// "New card refused": 402 card_refused and the previous card is still the saved one.
func TestBillingHandler_ChangeCard_Refused(t *testing.T) {
	d := newBillingDeps(t, true)
	d.rosaIsTheAdmin()
	d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, domainError.ErrCardRefused)

	rec := d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody)

	require.Equal(t, http.StatusPaymentRequired, rec.Code)
	assert.JSONEq(t, `{"error":"Card refused","code":"card_refused"}`, rec.Body.String())
	d.billing.AssertNotCalled(t, "SaveCard", mock.Anything, mock.Anything, mock.Anything)
}

func TestBillingHandler_ChangeCard_Failures(t *testing.T) {
	t.Run("a missing token names its field", func(t *testing.T) {
		rec := newBillingDeps(t, true).as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", `{"card_token":"tok_test_1"}`)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `"acceptance_token"`, string(decodeBody(t, rec)["field"]))
	})
	t.Run("the provider is down", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.rosaIsTheAdmin()
		d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).
			Return(dto.PaymentSource{}, errors.Join(domainError.ErrPaymentProviderUnavailable, errors.New(billingPrivateKey)))

		rec := d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody)

		require.Equal(t, http.StatusBadGateway, rec.Code)
		assert.JSONEq(t, `"payment_provider_unavailable"`, string(decodeBody(t, rec)["code"]))
		assert.NotContains(t, rec.Body.String(), billingPrivateKey)
	})
	t.Run("not JSON", func(t *testing.T) {
		rec := newBillingDeps(t, true).as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", `card`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// The limit of 5 card attempts an hour is one count for the organization, whichever route sends the card.
func TestBillingHandler_CardRoutes_ShareOneLimitOfFiveAttempts(t *testing.T) {
	d := newBillingDeps(t, true)
	d.rosaIsTheAdmin()
	d.organizationIs(&dto.Organization{ID: testOrganizationID, Status: "pending", Plan: "starter"}, nil)
	d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, domainError.ErrCardRefused)

	codes := []int{
		d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody).Code,
		d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody).Code,
		d.send(http.MethodPost, "/api/billing/setup/card", cardBody, "").Code,
		d.send(http.MethodPost, "/api/billing/setup/card", cardBody, "").Code,
		d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody).Code,
	}
	assert.Equal(t, []int{402, 402, 402, 402, 402}, codes, "refused cards count too")

	sixth := d.as(t, permissions.RoleAdmin, http.MethodPut, "/api/billing/card", cardBody)
	assert.Equal(t, http.StatusTooManyRequests, sixth.Code)
	assert.NotEmpty(t, sixth.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusTooManyRequests, d.send(http.MethodPost, "/api/billing/setup/card", cardBody, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, d.send(http.MethodPost, "/api/billing/recovery/pay", cardBody, "").Code)
}

// "Cancelling on day 10 of the trial" and "Changing their mind".
func TestBillingHandler_CancelAndResume(t *testing.T) {
	t.Run("cancel marks the subscription and answers the subscription", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.organizationIs(trialOrganization(4*day), cardOnFile(nil))
		d.billing.EXPECT().SetCancelled(mock.Anything, testOrganizationID, mock.MatchedBy(func(at *time.Time) bool { return at != nil })).Return(nil)

		rec := d.as(t, permissions.RoleAdmin, http.MethodPost, "/api/billing/cancel", "")

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, decodeBody(t, rec), "charges")
	})
	t.Run("resume before the due date removes the cancellation", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.organizationIs(trialOrganization(2*day), cardOnFile(func(b *dto.OrganizationBilling) { at := time.Now().Add(-2 * day); b.CancelledAt = &at }))
		d.billing.EXPECT().SetCancelled(mock.Anything, testOrganizationID, (*time.Time)(nil)).Return(nil)

		rec := d.as(t, permissions.RoleAdmin, http.MethodPost, "/api/billing/resume", "")

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
	t.Run("resume after the due date is 409 subscription_lapsed", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.organizationIs(trialOrganization(-time.Hour), cardOnFile(func(b *dto.OrganizationBilling) { at := time.Now().Add(-2 * day); b.CancelledAt = &at }))

		rec := d.as(t, permissions.RoleAdmin, http.MethodPost, "/api/billing/resume", "")

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.JSONEq(t, `"subscription_lapsed"`, string(decodeBody(t, rec)["code"]))
	})
	for _, action := range []string{"cancel", "resume"} {
		t.Run(action+" without a saved card is 409 no_subscription", func(t *testing.T) {
			d := newBillingDeps(t, true)
			d.organizationIs(paidOrganization(), nil)

			rec := d.as(t, permissions.RoleAdmin, http.MethodPost, "/api/billing/"+action, "")

			require.Equal(t, http.StatusConflict, rec.Code)
			assert.JSONEq(t, `"no_subscription"`, string(decodeBody(t, rec)["code"]))
		})
	}
}

func stoppedBilling() *dto.OrganizationBilling {
	return cardOnFile(func(b *dto.OrganizationBilling) { paidUntil(-10 * day)(b); b.Attempts = 3 })
}

func declinedCharge(billing *dto.OrganizationBilling) *dto.BillingCharge {
	return &dto.BillingCharge{ID: "c3", OrganizationID: testOrganizationID, DueOn: *billing.PaidUntil, Attempt: 3, Reference: "kb-3",
		Plan: "einvoicing", Period: "monthly", Amount: 40000, Currency: "COP", Status: dto.BillingChargeDeclined, CreatedAt: time.Now().Add(-7 * day)}
}

func TestBillingHandler_RecoveryState(t *testing.T) {
	d := newBillingDeps(t, true)
	billing := stoppedBilling()
	d.organizationIs(paidOrganization(), billing, declinedCharge(billing))
	d.provider.EXPECT().Terms(mock.Anything).Return(dto.WompiMerchantTerms{TermsURL: "https://wompi.co/terms.pdf",
		PersonalDataURL: "https://wompi.co/data.pdf", AcceptanceToken: "acc-1", PersonalAuthToken: "acc-2"}, nil)

	rec := d.send(http.MethodGet, "/api/billing/recovery", "", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got dto.RecoveryState
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "Doña Rosa", got.OrganizationName)
	assert.Equal(t, "payment_required", got.Reason)
	assert.Equal(t, "einvoicing", got.Plan)
	assert.Equal(t, "monthly", got.Period)
	assert.Equal(t, 40000, got.AmountDue)
	assert.Equal(t, &dto.SavedCard{Brand: "VISA", LastFour: "4242"}, got.Card)
	assert.Equal(t, "declined", got.LatestCharge.Status)
	assert.True(t, got.CardEntry)
	assert.Equal(t, "pub_test_abc", got.Wompi.PublicKey)
	assert.NotContains(t, rec.Body.String(), "3891")
	assert.NotContains(t, decodeBody(t, rec), "token", "no sign-in while it is stopped")
}

func TestBillingHandler_RecoveryPay(t *testing.T) {
	t.Run("with the saved card answers 202 and the charge's state", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.rosaIsTheAdmin()
		billing := stoppedBilling()
		d.organizationIs(paidOrganization(), billing, declinedCharge(billing))
		var inserted *dto.BillingCharge
		d.billing.EXPECT().InsertCharge(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c *dto.BillingCharge) error {
			inserted = c
			return nil
		})
		d.provider.EXPECT().Charge(mock.Anything, mock.MatchedBy(func(req dto.BillingChargeRequest) bool {
			return req.PaymentSourceID == "3891" && req.AmountInCents == 4000000
		})).Return(dto.BillingTransaction{ID: "tx_9", Status: dto.BillingChargePending}, nil)
		d.billing.EXPECT().LockChargeByReference(mock.Anything, mock.Anything).RunAndReturn(
			func(context.Context, string) (*dto.BillingCharge, bool, error) { return inserted, true, nil })
		d.billing.EXPECT().UpdateCharge(mock.Anything, mock.Anything).Return(nil)

		rec := d.send(http.MethodPost, "/api/billing/recovery/pay", "", "")

		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		var got dto.RecoveryPayResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, "pending", got.Charge.Status)
		assert.Equal(t, 40000, got.Charge.Amount)
	})
	t.Run("while a charge is pending is 409 charge_pending", func(t *testing.T) {
		d := newBillingDeps(t, true)
		billing := stoppedBilling()
		pending := declinedCharge(billing)
		pending.Status = dto.BillingChargePending
		d.organizationIs(paidOrganization(), billing, pending)

		rec := d.send(http.MethodPost, "/api/billing/recovery/pay", "", "")

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.JSONEq(t, `"charge_pending"`, string(decodeBody(t, rec)["code"]))
	})
	t.Run("without a saved card and without a new one is 400 naming the card token", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.organizationIs(trialOrganization(-7*day), nil)

		rec := d.send(http.MethodPost, "/api/billing/recovery/pay", `{}`, "")

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `"card_token"`, string(decodeBody(t, rec)["field"]))
	})
	t.Run("a working organization is 409 not_stopped", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.organizationIs(trialOrganization(4*day), cardOnFile(nil))

		rec := d.send(http.MethodPost, "/api/billing/recovery/pay", "", "")

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.JSONEq(t, `"not_stopped"`, string(decodeBody(t, rec)["code"]))
	})
	t.Run("a new card that is refused is 402 card_refused", func(t *testing.T) {
		d := newBillingDeps(t, true)
		d.rosaIsTheAdmin()
		billing := stoppedBilling()
		d.organizationIs(paidOrganization(), billing, declinedCharge(billing))
		d.provider.EXPECT().SavePaymentSource(mock.Anything, mock.Anything).Return(dto.PaymentSource{}, domainError.ErrCardRefused)

		rec := d.send(http.MethodPost, "/api/billing/recovery/pay", cardBody, "")

		require.Equal(t, http.StatusPaymentRequired, rec.Code)
		assert.JSONEq(t, `"card_refused"`, string(decodeBody(t, rec)["code"]))
	})
}

// After the approval the state answers the same body as a successful sign-in, once.
func TestBillingHandler_RecoveryState_SignsInOnceAfterApproval(t *testing.T) {
	d := newBillingDeps(t, true)
	d.rosaIsTheAdmin()
	d.organizationIs(paidOrganization(), cardOnFile(paidUntil(30*day)))
	d.billing.EXPECT().ClaimRecoverySignIn(mock.Anything, testOrganizationID, mock.Anything, mock.Anything).Return(true, nil).Once()
	d.billing.EXPECT().ClaimRecoverySignIn(mock.Anything, testOrganizationID, mock.Anything, mock.Anything).Return(false, nil).Once()

	first := d.send(http.MethodGet, "/api/billing/recovery", "", "")

	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var signIn dto.SignInResponse
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &signIn))
	claims, err := d.jwt.ValidateToken(signIn.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrganizationID, claims.OrgID)
	assert.Equal(t, "rosa-id", claims.UserID)
	assert.Contains(t, signIn.Permissions, "billing:manage")

	second := d.send(http.MethodGet, "/api/billing/recovery", "", "")
	require.Equal(t, http.StatusConflict, second.Code)
	assert.JSONEq(t, `"not_stopped"`, string(decodeBody(t, second)["code"]))
	assert.NotContains(t, decodeBody(t, second), "token")
}

func TestBillingHandler_RecoverySelection(t *testing.T) {
	d := newBillingDeps(t, true)
	d.organizationIs(paidOrganization(), stoppedBilling())
	starter := "starter"
	d.orgRepo.EXPECT().Update(mock.Anything, testOrganizationID, &dto.UpdateOrganizationRequest{Plan: &starter}).Return(paidOrganization(), nil)
	d.billing.EXPECT().SetPeriod(mock.Anything, testOrganizationID, "annual").Return(nil)

	rec := d.send(http.MethodPut, "/api/billing/recovery/selection", `{"plan":"starter","period":"annual"}`, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"plan":"starter","period":"annual","amount":250000,"currency":"COP"}`, rec.Body.String())

	working := newBillingDeps(t, true)
	working.organizationIs(trialOrganization(4*day), cardOnFile(nil))
	refused := working.send(http.MethodPut, "/api/billing/recovery/selection", `{"plan":"starter","period":"annual"}`, "")
	require.Equal(t, http.StatusConflict, refused.Code)
	assert.JSONEq(t, `"not_stopped"`, string(decodeBody(t, refused)["code"]))
}
