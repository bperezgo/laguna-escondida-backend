package signup

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type setupState struct {
	OrganizationName string `json:"organization_name"`
	Plan             string `json:"plan"`
	Period           string `json:"period"`
	CardEntry        bool   `json:"card_entry"`
	Wompi            *struct {
		PublicKey                   string `json:"public_key"`
		APIURL                      string `json:"api_url"`
		TermsURL                    string `json:"terms_url"`
		PersonalDataURL             string `json:"personal_data_url"`
		AcceptanceToken             string `json:"acceptance_token"`
		PersonalDataAcceptanceToken string `json:"personal_data_acceptance_token"`
	} `json:"wompi"`
}

func (c *cloud) setupState(setupToken string) setupState {
	c.t.Helper()
	resp := c.as(setupToken, http.MethodGet, "/api/billing/setup", nil)
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	var state setupState
	resp.decode(c.t, &state)
	return state
}

func (c *cloud) saveCard(setupToken, cardToken string, state setupState) response {
	c.t.Helper()
	return c.as(setupToken, http.MethodPost, "/api/billing/setup/card", map[string]string{
		"card_token": cardToken, "acceptance_token": state.Wompi.AcceptanceToken,
		"personal_data_acceptance_token": state.Wompi.PersonalDataAcceptanceToken})
}

type storedBilling struct {
	Period          string
	Provider        *string
	PaymentSourceID *string
	CardBrand       *string
	CardLastFour    *string
	TermsAcceptedAt *time.Time
}

func billingOf(t *testing.T, orgID string) (storedBilling, bool) {
	t.Helper()
	var rows []storedBilling
	require.NoError(t, cloudDB.Raw(`SELECT period, provider, payment_source_id, card_brand, card_last_four, terms_accepted_at
		FROM organization_billing WHERE organization_id = ?`, orgID).Scan(&rows).Error)
	if len(rows) == 0 {
		return storedBilling{}, false
	}
	return rows[0], true
}

func statusOf(t *testing.T, orgID string) string {
	t.Helper()
	var status string
	require.NoError(t, cloudDB.Raw("SELECT status FROM organizations WHERE id = ?", orgID).Scan(&status).Error)
	return status
}

func TestBillingSetup_ASavedCardStartsTheTrial(t *testing.T) {
	c, wompi := newCloudWithBilling(t)
	email := unique("tarjeta")
	orgID, setup := c.signUpForSetup(email, "Casa Tarjeta", "einvoicing")

	assert.Equal(t, http.StatusUnauthorized, c.as(setup, http.MethodGet, "/api/orders", nil).status, "a setup token opens nothing else")
	assert.Equal(t, http.StatusUnauthorized, c.as(setup, http.MethodGet, "/api/auth/me", nil).status)

	state := c.setupState(setup)
	assert.Equal(t, "Casa Tarjeta", state.OrganizationName)
	assert.Equal(t, []string{"einvoicing", "monthly"}, []string{state.Plan, state.Period})
	require.True(t, state.CardEntry)
	require.NotNil(t, state.Wompi)
	assert.Equal(t, testsupport.FakeBillingPublicKey, state.Wompi.PublicKey)
	assert.NotEmpty(t, state.Wompi.TermsURL)
	assert.NotEmpty(t, state.Wompi.PersonalDataURL)
	assert.NotEqual(t, state.Wompi.AcceptanceToken, c.setupState(setup).Wompi.AcceptanceToken, "acceptance tokens are fresh on every read")

	resp := c.as(setup, http.MethodPut, "/api/billing/setup/selection", map[string]any{"plan": "full", "period": "annual", "amount": 1})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	assert.JSONEq(t, `{"plan":"full","period":"annual","amount":1000000,"currency":"COP"}`, string(resp.body))
	assert.Equal(t, "pending", statusOf(t, orgID), "choosing starts nothing")
	resp = c.as(setup, http.MethodPut, "/api/billing/setup/selection", map[string]string{"plan": "full", "period": "weekly"})
	assert.Equal(t, http.StatusBadRequest, resp.status)

	resp = c.saveCard(setup, "tok_test_4242", c.setupState(setup))
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var signIn dto.SignInResponse
	resp.decode(t, &signIn)
	require.NotEmpty(t, signIn.Token)
	assert.NotContains(t, string(resp.body), "payment_source")

	var m me
	meResp := c.as(signIn.Token, http.MethodGet, "/api/auth/me", nil)
	require.Equal(t, http.StatusOK, meResp.status, string(meResp.body))
	meResp.decode(t, &m)
	assert.Equal(t, "trial", m.Organization.Status)
	assert.Equal(t, "full", m.Organization.Plan)
	require.NotNil(t, m.Organization.TrialEndsAt)
	assert.WithinDuration(t, time.Now().Add(14*24*time.Hour), *m.Organization.TrialEndsAt, time.Minute)

	stored, found := billingOf(t, orgID)
	require.True(t, found)
	assert.Equal(t, "annual", stored.Period)
	require.NotNil(t, stored.PaymentSourceID)
	assert.Equal(t, []string{"wompi", "VISA", "4242"}, []string{*stored.Provider, *stored.CardBrand, *stored.CardLastFour})
	require.NotNil(t, stored.TermsAcceptedAt)
	assert.WithinDuration(t, time.Now(), *stored.TermsAcceptedAt, time.Minute)

	sources, transactions := wompi.Counts()
	assert.Equal(t, 1, sources)
	assert.Zero(t, transactions, "saving a card charges nothing")
	assert.Equal(t, email, wompi.Sources[0]["customer_email"])
	assert.Equal(t, "CARD", wompi.Sources[0]["type"])

	resp = c.as(setup, http.MethodGet, "/api/billing/setup", nil)
	assert.Equal(t, http.StatusConflict, resp.status)
	assert.Equal(t, "not_pending", resp.code(t), "the setup session ends with the pending state")
	assert.Equal(t, http.StatusUnauthorized, c.as(signIn.Token, http.MethodGet, "/api/billing/setup", nil).status, "a sign-in token opens no setup route")

	resp = c.operator(http.MethodGet, "/api/platform/organizations/"+orgID, nil)
	require.Equal(t, http.StatusOK, resp.status)
	var seen dto.Organization
	resp.decode(t, &seen)
	assert.Equal(t, "annual", seen.BillingPeriod)
	assert.True(t, seen.HasSavedCard)
	assert.NotContains(t, string(resp.body), *stored.PaymentSourceID+`"`)
	assert.NotContains(t, string(resp.body), "payment_source")
}

func TestBillingSetup_ARefusedCardLeavesTheAccountPending(t *testing.T) {
	c, wompi := newCloudWithBilling(t)
	orgID, setup := c.signUpForSetup(unique("rechazada"), "Casa Rechazada", "starter")

	resp := c.saveCard(setup, testsupport.RefusedCardToken, c.setupState(setup))

	assert.Equal(t, http.StatusPaymentRequired, resp.status)
	assert.Equal(t, "card_refused", resp.code(t))
	assert.Equal(t, "pending", statusOf(t, orgID))
	_, found := billingOf(t, orgID)
	assert.False(t, found, "a refused card stores nothing")
	_, transactions := wompi.Counts()
	assert.Zero(t, transactions)

	resp = c.as(setup, http.MethodPost, "/api/billing/setup/card", map[string]string{"card_token": "tok_test_4242"})
	assert.Equal(t, http.StatusBadRequest, resp.status, "the acceptances are required")

	resp = c.saveCard(setup, "tok_test_4242", c.setupState(setup))
	assert.Equal(t, http.StatusOK, resp.status, "the person retries with another card")
	assert.Equal(t, "trial", statusOf(t, orgID))
}

func TestBillingSetup_TwoSavesAtOnceStartOneTrial(t *testing.T) {
	c, _ := newCloudWithBilling(t)
	orgID, setup := c.signUpForSetup(unique("doble"), "Casa Doble", "starter")
	states := []setupState{c.setupState(setup), c.setupState(setup)}

	codes := make([]int, 2)
	bodies := make([]response, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bodies[i] = c.saveCard(setup, "tok_test_4242", states[i])
			codes[i] = bodies[i].status
		}()
	}
	wg.Wait()

	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes, "%s | %s", bodies[0].body, bodies[1].body)
	for _, b := range bodies {
		if b.status == http.StatusConflict {
			assert.Equal(t, "not_pending", b.code(t))
		}
	}
	assert.Equal(t, "trial", statusOf(t, orgID))
	stored, found := billingOf(t, orgID)
	require.True(t, found)
	require.NotNil(t, stored.PaymentSourceID, "the winner's card is the one kept")
}

func TestBillingSetup_WithoutKeysTheOperatorStillApproves(t *testing.T) {
	c := newCloud(t)
	email := unique("sinllaves")
	orgID, setup := c.signUpForSetup(email, "Casa Sin Llaves", "starter")

	state := c.setupState(setup)
	assert.False(t, state.CardEntry)
	assert.Nil(t, state.Wompi)
	assert.Equal(t, http.StatusNotFound, c.as(setup, http.MethodPost, "/api/billing/setup/card", map[string]string{"card_token": "tok"}).status)

	resp := c.as(setup, http.MethodPut, "/api/billing/setup/selection", map[string]string{"plan": "starter", "period": "annual"})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	assert.Equal(t, "annual", c.setupState(setup).Period)

	resp = c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")})
	require.Equal(t, http.StatusForbidden, resp.status)
	var pending map[string]string
	resp.decode(t, &pending)
	require.NotEmpty(t, pending["setup_token"], "a pending admin who signs in continues signup")
	assert.Equal(t, "annual", c.setupState(pending["setup_token"]).Period)

	resp = c.operator(http.MethodPost, "/api/platform/organizations/"+orgID+"/start-trial", nil)
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var started dto.Organization
	resp.decode(t, &started)
	assert.Equal(t, "trial", started.Status)
	assert.Equal(t, "annual", started.BillingPeriod)
	assert.False(t, started.HasSavedCard)
}
