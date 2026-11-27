package signup

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newChargingCloud is a cloud with all four billing values: cards are saved and charged.
func newChargingCloud(t *testing.T) (*cloud, *testsupport.FakeBillingWompi) {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
	wompi := testsupport.BillingWompi(t)
	return newCloudWith(t, wompi.PaymentLinks(), wompi.ChargingBilling()), wompi
}

// chargeJob is the charge job of one replica: its own service on the shared database and Wompi
// account. The cloud under test builds the same thing, behind a scheduler these tests don't wait for.
func chargeJob(wompi *testsupport.FakeBillingWompi) *service.ChargeService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider := httpclient.NewWompiBilling(httpclient.NewClient(logger), wompi.APIURL(),
		testsupport.FakeBillingPublicKey, testsupport.FakeBillingPrivateKey, testsupport.FakeBillingIntegritySecret)
	return service.NewChargeService(repository.NewBillingRepository(cloudDB), repository.NewOrganizationRepository(cloudDB),
		provider, postgres.NewUnitOfWork(cloudDB), nil, logger)
}

// trialWithCard signs a restaurant up and leaves a card, as signup-card-step does.
func (c *cloud) trialWithCard(email, restaurant, plan string) (orgID, token string) {
	c.t.Helper()
	orgID, setup := c.signUpForSetup(email, restaurant, plan)
	resp := c.saveCard(setup, "tok_test_4242", c.setupState(setup))
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	var signIn dto.SignInResponse
	resp.decode(c.t, &signIn)
	return orgID, signIn.Token
}

// trialEnded moves the trial's end to the past through the operator, which is how time passes here.
func (c *cloud) trialEnded(orgID string, ago time.Duration) time.Time {
	c.t.Helper()
	end := time.Now().UTC().Add(-ago).Truncate(time.Second)
	resp := c.operator(http.MethodPatch, "/api/platform/organizations/"+orgID, map[string]string{"trial_ends_at": end.Format(time.RFC3339)})
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	return end
}

func chargesOf(wompi *testsupport.FakeBillingWompi, orgID string) []testsupport.FakeTransaction {
	var own []testsupport.FakeTransaction
	for _, tx := range wompi.ChargeList() {
		if strings.HasPrefix(tx.Reference, "kb-"+orgID+"-") {
			own = append(own, tx)
		}
	}
	return own
}

func (c *cloud) billingEvent(body []byte) response {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodPost, c.server.URL+"/api/webhooks/wompi-billing", bytes.NewReader(body))
	require.NoError(c.t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, body: raw}
}

func (c *cloud) subscription(token string) dto.Subscription {
	c.t.Helper()
	resp := c.as(token, http.MethodGet, "/api/billing", nil)
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	var sub dto.Subscription
	resp.decode(c.t, &sub)
	return sub
}

func paidUntilOf(t *testing.T, orgID string) *time.Time {
	t.Helper()
	var row struct{ PaidUntil *time.Time }
	require.NoError(t, cloudDB.Raw("SELECT paid_until FROM organization_billing WHERE organization_id = ?", orgID).Scan(&row).Error)
	return row.PaidUntil
}

// Trial ends → charge → event → active and paid.
func TestSubscriptionCharges_TrialEndsChargedAndSettledByTheEvent(t *testing.T) {
	c, wompi := newChargingCloud(t)
	email := unique("cobro")
	orgID, token := c.trialWithCard(email, "Casa Cobro", "einvoicing")
	job := chargeJob(wompi)

	require.NoError(t, job.ChargeDue(context.Background()))
	assert.Empty(t, chargesOf(wompi, orgID), "nothing is charged during the trial")

	trialEnd := c.trialEnded(orgID, time.Minute)
	require.NoError(t, job.ChargeDue(context.Background()))

	charges := chargesOf(wompi, orgID)
	require.Len(t, charges, 1)
	assert.Equal(t, int64(4000000), charges[0].AmountInCents, "einvoicing, monthly: 40000 pesos, no tax")
	assert.Equal(t, "COP", charges[0].Currency)
	assert.Equal(t, email, charges[0].CustomerEmail)
	assert.Equal(t, "kb-"+orgID+"-"+trialEnd.Format("20060102")+"-1", charges[0].Reference)
	stored, _ := billingOf(t, orgID)
	assert.Equal(t, *stored.PaymentSourceID, charges[0].PaymentSourceID)

	pending := c.subscription(token)
	assert.Equal(t, "trial", pending.Status)
	require.NotNil(t, pending.LatestCharge)
	assert.Equal(t, "pending", pending.LatestCharge.Status)
	assert.Equal(t, "trial", statusOf(t, orgID), "only an approval changes the organization")

	wompi.SetStatus(charges[0].ID, "APPROVED")
	forged := c.billingEvent(wompi.Event(t, charges[0].ID, "test_events_someone_else"))
	assert.Equal(t, http.StatusUnauthorized, forged.status)
	assert.Equal(t, "trial", statusOf(t, orgID), "a forged event changes nothing")
	assert.Nil(t, paidUntilOf(t, orgID))

	signed := wompi.Event(t, charges[0].ID, testsupport.FakeBillingEventsSecret)
	require.Equal(t, http.StatusOK, c.billingEvent(signed).status)

	assert.Equal(t, "active", statusOf(t, orgID))
	paidUntil := paidUntilOf(t, orgID)
	require.NotNil(t, paidUntil)
	assert.True(t, paidUntil.Equal(service.AddBillingPeriod(trialEnd, "monthly")), "paid from the trial's end for one month, got %s", paidUntil)

	require.Equal(t, http.StatusOK, c.billingEvent(signed).status, "Wompi sends an event again")
	assert.True(t, paidUntilOf(t, orgID).Equal(*paidUntil), "and the period is extended once")

	paid := c.subscription(token)
	assert.Equal(t, "active", paid.Status)
	require.Len(t, paid.Charges, 1)
	assert.Equal(t, "approved", paid.Charges[0].Status)
	assert.Equal(t, 40000, paid.Charges[0].Amount)
	require.NotNil(t, paid.Charges[0].CoversFrom)
	assert.True(t, paid.Charges[0].CoversFrom.Equal(trialEnd))
	assert.True(t, paid.DueDate.Equal(*paidUntil), "the next due date is the end of the paid period")
	assert.False(t, paid.PaymentFailed)
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status)

	require.NoError(t, job.ChargeDue(context.Background()))
	assert.Len(t, chargesOf(wompi, orgID), 1, "a paid period is not charged again")

	var m struct {
		Billing *dto.BillingSummary `json:"billing"`
	}
	c.as(token, http.MethodGet, "/api/auth/me", nil).decode(t, &m)
	require.NotNil(t, m.Billing, "an admin's /auth/me carries what the notice needs")
	assert.True(t, m.Billing.HasCard)
}

// Declined three times → stopped with trial_expired after grace, then paid back through recovery.
func TestSubscriptionCharges_ThreeDeclinesStopTheTrialAndRecoveryPaysItBack(t *testing.T) {
	c, wompi := newChargingCloud(t)
	email := unique("rechazo")
	orgID, token := c.trialWithCard(email, "Casa Rechazo", "starter")
	job := chargeJob(wompi)
	wompi.AnswerWith("DECLINED")

	c.trialEnded(orgID, time.Minute)
	require.NoError(t, job.ChargeDue(context.Background()))
	require.NoError(t, job.ChargeDue(context.Background()))
	require.Len(t, chargesOf(wompi, orgID), 1, "the retry waits a day")
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status, "it keeps working while the retries run")
	failing := c.subscription(token)
	assert.True(t, failing.PaymentFailed)
	assert.Equal(t, "declined", failing.LatestCharge.Status)

	c.trialEnded(orgID, 25*time.Hour)
	require.NoError(t, job.ChargeDue(context.Background()))
	require.NoError(t, job.ChargeDue(context.Background()))
	require.Len(t, chargesOf(wompi, orgID), 2)
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status)

	c.trialEnded(orgID, 73*time.Hour)
	require.NoError(t, job.ChargeDue(context.Background()))
	require.NoError(t, job.ChargeDue(context.Background()))
	require.Len(t, chargesOf(wompi, orgID), 3, "no fourth charge")
	assert.Equal(t, "trial", statusOf(t, orgID))

	refused := c.as(token, http.MethodGet, "/api/products", nil)
	assert.Equal(t, http.StatusForbidden, refused.status)
	assert.Equal(t, "trial_expired", refused.code(t))

	signIn := c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")})
	require.Equal(t, http.StatusForbidden, signIn.status)
	assert.Equal(t, "trial_expired", signIn.code(t))
	var stopped map[string]string
	signIn.decode(t, &stopped)
	setup := stopped["setup_token"]
	require.NotEmpty(t, setup, "the admin gets a setup token for the recovery routes")
	assert.NotContains(t, stopped, "token")

	assert.Equal(t, http.StatusConflict, c.as(setup, http.MethodGet, "/api/billing/setup", nil).status, "it is not a pending signup")
	assert.Equal(t, http.StatusUnauthorized, c.as(setup, http.MethodGet, "/api/billing", nil).status, "nor a sign-in")

	var state dto.RecoveryState
	stateResp := c.as(setup, http.MethodGet, "/api/billing/recovery", nil)
	require.Equal(t, http.StatusOK, stateResp.status, string(stateResp.body))
	stateResp.decode(t, &state)
	assert.Equal(t, "trial_expired", state.Reason)
	assert.Equal(t, "Casa Rechazo", state.OrganizationName)
	assert.Equal(t, 25000, state.AmountDue)
	assert.Equal(t, &dto.SavedCard{Brand: "VISA", LastFour: "4242"}, state.Card)
	assert.Equal(t, "declined", state.LatestCharge.Status)
	assert.True(t, state.CardEntry)

	declinedAgain := c.as(setup, http.MethodPost, "/api/billing/recovery/pay", nil)
	require.Equal(t, http.StatusAccepted, declinedAgain.status, string(declinedAgain.body))
	assert.Equal(t, "trial", statusOf(t, orgID), "a declined recovery charge leaves it stopped")
	require.Len(t, chargesOf(wompi, orgID), 4)

	wompi.AnswerWith("PENDING")
	paying := c.as(setup, http.MethodPost, "/api/billing/recovery/pay", nil)
	require.Equal(t, http.StatusAccepted, paying.status, string(paying.body))
	twice := c.as(setup, http.MethodPost, "/api/billing/recovery/pay", nil)
	assert.Equal(t, http.StatusConflict, twice.status)
	assert.Equal(t, "charge_pending", twice.code(t))
	charges := chargesOf(wompi, orgID)
	require.Len(t, charges, 5)

	approvedAt := time.Now().UTC()
	wompi.SetStatus(charges[4].ID, "APPROVED")
	require.Equal(t, http.StatusOK, c.billingEvent(wompi.Event(t, charges[4].ID, testsupport.FakeBillingEventsSecret)).status)

	assert.Equal(t, "active", statusOf(t, orgID))
	paidUntil := paidUntilOf(t, orgID)
	require.NotNil(t, paidUntil)
	assert.WithinDuration(t, service.AddBillingPeriod(approvedAt, "monthly"), *paidUntil, time.Minute, "a recovery pays from the approval")

	recovered := c.as(setup, http.MethodGet, "/api/billing/recovery", nil)
	require.Equal(t, http.StatusOK, recovered.status, string(recovered.body))
	var signedIn dto.SignInResponse
	recovered.decode(t, &signedIn)
	require.NotEmpty(t, signedIn.Token, "the approved payment signs the admin in")
	assert.Equal(t, http.StatusOK, c.as(signedIn.Token, http.MethodGet, "/api/products", nil).status)

	again := c.as(setup, http.MethodGet, "/api/billing/recovery", nil)
	assert.Equal(t, http.StatusConflict, again.status, "once")
	assert.Equal(t, "not_stopped", again.code(t))
	assert.Equal(t, http.StatusConflict, c.as(setup, http.MethodPost, "/api/billing/recovery/pay", nil).status)
}

// Two replicas' ChargeDue at once → one transaction at the fake.
func TestSubscriptionCharges_ReplicasAtTheDueTimeMakeOneTransaction(t *testing.T) {
	c, wompi := newChargingCloud(t)
	orgID, _ := c.trialWithCard(unique("replicas"), "Casa Réplicas", "full")
	c.trialEnded(orgID, time.Minute)

	const replicas = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, replicas)
	for i := range replicas {
		job := chargeJob(wompi)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = job.ChargeDue(context.Background())
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	charges := chargesOf(wompi, orgID)
	require.Len(t, charges, 1, "one transaction, whichever replica made it")
	assert.Equal(t, int64(10000000), charges[0].AmountInCents)
	var rows, attempts int
	require.NoError(t, cloudDB.Raw("SELECT COUNT(*) FROM billing_charges WHERE organization_id = ?", orgID).Scan(&rows).Error)
	require.NoError(t, cloudDB.Raw("SELECT attempts FROM organization_billing WHERE organization_id = ?", orgID).Scan(&attempts).Error)
	assert.Equal(t, 1, rows)
	assert.Equal(t, 1, attempts)
}

// The event never arrives: the lookup settles the charge by what Wompi returns.
func TestSubscriptionCharges_AChargeWithNoEventIsLookedUp(t *testing.T) {
	c, wompi := newChargingCloud(t)
	orgID, _ := c.trialWithCard(unique("consulta"), "Casa Consulta", "starter")
	job := chargeJob(wompi)
	c.trialEnded(orgID, time.Minute)
	require.NoError(t, job.ChargeDue(context.Background()))
	charges := chargesOf(wompi, orgID)
	require.Len(t, charges, 1)
	wompi.SetStatus(charges[0].ID, "APPROVED")

	require.NoError(t, job.Reconcile(context.Background()))
	assert.Equal(t, "trial", statusOf(t, orgID), "a charge younger than ten minutes waits for its event")

	require.NoError(t, cloudDB.Exec("UPDATE billing_charges SET created_at = created_at - INTERVAL '11 minutes' WHERE organization_id = ?", orgID).Error)
	require.NoError(t, job.Reconcile(context.Background()))

	assert.Equal(t, "active", statusOf(t, orgID))
	assert.NotNil(t, paidUntilOf(t, orgID))
}

// Cancelled before the trial ends: no charge, and it stops at the trial's end with no grace.
func TestSubscriptionCharges_ACancelledTrialIsNeverCharged(t *testing.T) {
	c, wompi := newChargingCloud(t)
	email := unique("cancela")
	orgID, token := c.trialWithCard(email, "Casa Cancela", "einvoicing")

	manager := c.as(token, http.MethodPost, "/api/billing/cancel", nil)
	require.Equal(t, http.StatusOK, manager.status, string(manager.body))
	var cancelled dto.Subscription
	manager.decode(t, &cancelled)
	assert.True(t, cancelled.Cancelled)
	assert.Nil(t, cancelled.NextChargeAmount)
	assert.NotNil(t, cancelled.Card, "the card is kept")
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status, "it works until the due date")

	resumed := c.as(token, http.MethodPost, "/api/billing/resume", nil)
	require.Equal(t, http.StatusOK, resumed.status)
	require.Equal(t, http.StatusOK, c.as(token, http.MethodPost, "/api/billing/cancel", nil).status)

	c.trialEnded(orgID, time.Minute)
	require.NoError(t, chargeJob(wompi).ChargeDue(context.Background()))
	assert.Empty(t, chargesOf(wompi, orgID), "no charge is made for a cancelled subscription")

	refused := c.as(token, http.MethodGet, "/api/products", nil)
	assert.Equal(t, http.StatusForbidden, refused.status)
	assert.Equal(t, "trial_expired", refused.code(t))
	signIn := c.call(http.MethodPost, "/api/auth/google", nil, map[string]string{"id_token": googleToken(email, "Rosa")})
	assert.Equal(t, http.StatusForbidden, signIn.status)
	assert.Equal(t, "trial_expired", signIn.code(t))
	var stopped map[string]string
	signIn.decode(t, &stopped)
	require.NotEmpty(t, stopped["setup_token"])
	lapsed := c.as(stopped["setup_token"], http.MethodPost, "/api/billing/recovery/pay", nil)
	assert.Equal(t, http.StatusAccepted, lapsed.status, "a cancelled subscription can still pay its way back")
}

// An organization activated by hand is never charged and never stops, card or not.
func TestSubscriptionCharges_ActivatedByHandIsNeverCharged(t *testing.T) {
	c, wompi := newChargingCloud(t)
	orgID, token := c.trialWithCard(unique("manual"), "Casa Manual", "full")
	c.trialEnded(orgID, 30*24*time.Hour)
	require.Equal(t, http.StatusOK, c.operator(http.MethodPatch, "/api/platform/organizations/"+orgID, map[string]string{"status": "active"}).status)

	require.NoError(t, chargeJob(wompi).ChargeDue(context.Background()))

	assert.Empty(t, chargesOf(wompi, orgID))
	assert.Equal(t, http.StatusOK, c.as(token, http.MethodGet, "/api/products", nil).status)
	sub := c.subscription(token)
	assert.Nil(t, sub.NextChargeAmount)
	assert.Nil(t, sub.AccessEndsAt)
}

// A manager is refused every admin billing route.
func TestSubscriptionCharges_AManagerCannotManageBilling(t *testing.T) {
	c, _ := newChargingCloud(t)
	orgID, _ := c.trialWithCard(unique("gerente"), "Casa Gerente", "starter")
	var managerID string
	require.NoError(t, cloudDB.Raw(`SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.organization_id = ? AND ur.role_id = 3 LIMIT 1`, orgID).Scan(&managerID).Error)
	if managerID == "" {
		t.Skip("signup created no manager to test with")
	}
	token, err := service.NewJWTService("accept-secret", tenant.AnyOrganization()).GenerateToken(managerID, orgID, "caja", []int{3})
	require.NoError(t, err)

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/billing"}, {http.MethodPost, "/api/billing/cancel"}, {http.MethodPost, "/api/billing/resume"},
		{http.MethodGet, "/api/billing/card-setup"}, {http.MethodPut, "/api/billing/card"},
	} {
		assert.Equal(t, http.StatusForbidden, c.as(token, route.method, route.path, map[string]string{}).status, "%s %s", route.method, route.path)
	}
}
