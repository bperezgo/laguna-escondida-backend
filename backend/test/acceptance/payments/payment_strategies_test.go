package payments

import (
	"context"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/pkg/eventbus"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const transferText = "Llave Bre-B 3001234567, a nombre de Laguna Escondida SAS"

type manualOrder struct {
	order
	Strategy     *string    `json:"payment_strategy"`
	ExpiresAt    *time.Time `json:"payment_expires_at"`
	Instructions *string    `json:"payment_instructions"`
	UnderReview  *bool      `json:"payment_under_review"`
}

func (c *cloud) adminToken(r *restaurant) string {
	c.t.Helper()
	id := seedUser(c.t, r.orgID, permissions.RoleAdmin)
	token, err := service.NewJWTService(jwtSecret, tenant.AnyOrganization()).
		GenerateToken(id, r.orgID, "admin-"+id[:8], []int{permissions.RoleAdmin})
	require.NoError(c.t, err)
	return token
}

func (c *cloud) setPayment(r *restaurant, admin string, settings map[string]any) {
	c.t.Helper()
	resp := c.as(admin, http.MethodPut, "/api/organization/payment-settings", settings)
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
}

func manualSettings(window int) map[string]any {
	return map[string]any{"strategy": "manual", "manual_instructions": transferText, "manual_window_minutes": window}
}

// expireManualDue runs the expiry job with the manual query, as the cloud wires it.
func (c *cloud) expireManualDue(r *restaurant) {
	c.t.Helper()
	db := cloudDB
	orgs := repository.NewOrganizationRepository(db)
	linkRepo := repository.NewPaymentLinkRepository(db)
	txRepo := repository.NewPaymentTransactionRepository(db)
	openBills := repository.NewOpenBillRepository(db)
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	defer func() { _ = bus.Close() }()
	orders := service.NewOrderServiceWithSSE(c.logger, openBills, repository.NewProductRepository(db),
		repository.NewBillRepository(db), repository.NewPendingInvoiceRepository(db), dto.PendingInvoiceStatusPending,
		repository.NewBillOwnerRepository(db), postgres.NewUnitOfWork(db), bus, repository.NewUserRepository(db), nil,
		repository.NewSyncOutboxRepository(db), dto.SyncIdentity{NodeID: uuid.NewString()}, repository.NewProductIngredientRepository(db),
		repository.NewDeliveryZoneRepository(db), "CO")
	links := service.NewPaymentLinkService(linkRepo, httpclient.NewWompiClient(httpclient.NewClient(c.logger), c.links.APIURL),
		testsupport.IntegrationService(c.t, db, c.links.APIURL), orgs, c.links.TTL, c.logger, nil)
	runner := service.NewOrganizationRunner(orgs, postgres.NewSessionOpener(db))
	expiry := service.NewPaymentExpiryService(runner, linkRepo, txRepo, links, orders, c.logger, nil, service.WithManualExpiry(openBills, orders))
	require.NoError(c.t, expiry.ExpireDueForOrganization(tenant.WithOrg(context.Background(), r.orgID)))
}

func (c *cloud) payAtTheTill(key, orderID string) response {
	return c.as(key, http.MethodPost, "/api/orders/pay-order", map[string]any{"order_id": orderID, "payment_type": dto.ElectronicInvoicePaymentCodeCash, "customer": dto.Customer{DocumentNumber: "222222222222", DocumentType: dto.DocumentTypeNationalIdentificationNumber, Name: "Consumidor final", Email: "cf@example.com"}})
}

func TestPaymentStrategies_AManualOrderNeedsNoWompiAndIsPaidByStaff(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute)
	c.join(r)
	admin := c.adminToken(r)
	c.setPayment(r, admin, manualSettings(60))

	resp := c.createTakeaway(r.agentKey, r, 1)

	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created manualOrder
	resp.decode(t, &created)
	require.NotNil(t, created.Strategy)
	assert.Equal(t, "manual", *created.Strategy)
	assert.Nil(t, created.PaymentURL, "no link")
	require.NotNil(t, created.Instructions)
	assert.Equal(t, transferText, *created.Instructions)
	require.NotNil(t, created.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(time.Hour), *created.ExpiresAt, time.Minute)
	c.wompi.mu.Lock()
	assert.Empty(t, c.wompi.created, "Wompi was never called")
	c.wompi.mu.Unlock()

	changed := c.as(r.agentKey, http.MethodPut, "/api/orders/"+created.ID, map[string]any{
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 2}},
	})
	require.Equal(t, http.StatusOK, changed.status, string(changed.body))
	var updated manualOrder
	changed.decode(t, &updated)
	assert.True(t, created.ExpiresAt.Equal(*updated.ExpiresAt), "a change does not buy time")
	assert.Nil(t, updated.PaymentURL)

	assert.Equal(t, http.StatusForbidden, c.as(r.agentKey, http.MethodPut, "/api/orders/"+created.ID+"/payment-review", nil).status, "the agent cannot")
	review := c.as(r.managerKey, http.MethodPut, "/api/orders/"+created.ID+"/payment-review", nil)
	require.Equal(t, http.StatusOK, review.status, string(review.body))
	var reviewed manualOrder
	review.decode(t, &reviewed)
	require.NotNil(t, reviewed.UnderReview)
	assert.True(t, *reviewed.UnderReview)
	assert.Equal(t, http.StatusOK, c.as(r.managerKey, http.MethodPut, "/api/orders/"+created.ID+"/payment-review", nil).status, "idempotent")

	paid := c.payAtTheTill(r.managerKey, created.ID)
	require.Equal(t, http.StatusCreated, paid.status, string(paid.body))
	var closed order
	c.as(r.managerKey, http.MethodGet, "/api/orders/closed/"+created.ID, nil).decode(t, &closed)
	assert.NotNil(t, closed.PaidAt)
	var bills int
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM bills WHERE organization_id = ?", r.orgID).Scan(&bills).Error)
	assert.Equal(t, 1, bills)
	assert.Equal(t, http.StatusConflict, c.as(r.managerKey, http.MethodDelete, "/api/orders/"+created.ID+"/payment-review", nil).status, "a paid order cannot be reviewed")
}

func TestPaymentStrategies_AnUnpaidManualOrderExpiresUnlessUnderReview(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute)
	c.join(r)
	c.setPayment(r, c.adminToken(r), manualSettings(60))
	var expiring, checked manualOrder
	c.createTakeaway(r.agentKey, r, 1).decode(t, &expiring)
	c.createTakeaway(r.agentKey, r, 1).decode(t, &checked)
	require.Equal(t, http.StatusOK, c.as(r.managerKey, http.MethodPut, "/api/orders/"+checked.ID+"/payment-review", nil).status)
	require.NoError(t, cloudDB.Exec("UPDATE open_bills SET payment_expires_at = now() - interval '1 minute' WHERE id IN (?, ?)", expiring.ID, checked.ID).Error)

	c.expireManualDue(r)

	assert.Equal(t, http.StatusNotFound, c.as(r.agentKey, http.MethodGet, "/api/orders/"+expiring.ID, nil).status)
	var closed order
	c.as(r.managerKey, http.MethodGet, "/api/orders/closed/"+expiring.ID, nil).decode(t, &closed)
	require.NotNil(t, closed.VoidReason)
	assert.Equal(t, "payment_expired", *closed.VoidReason)
	assert.Equal(t, http.StatusOK, c.as(r.agentKey, http.MethodGet, "/api/orders/"+checked.ID, nil).status, "under review: held")

	require.Equal(t, http.StatusOK, c.as(r.managerKey, http.MethodDelete, "/api/orders/"+checked.ID+"/payment-review", nil).status)
	c.expireManualDue(r)
	assert.Equal(t, http.StatusNotFound, c.as(r.agentKey, http.MethodGet, "/api/orders/"+checked.ID, nil).status, "review cleared past its expiry: voided on the next run")
}

func TestPaymentStrategies_AnOrderKeepsItsStrategyWhenTheSettingChanges(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)
	admin := c.adminToken(r)
	c.setPayment(r, admin, manualSettings(60))
	var manual manualOrder
	c.createTakeaway(r.agentKey, r, 1).decode(t, &manual)

	c.setPayment(r, admin, map[string]any{"strategy": "wompi"})

	var read manualOrder
	c.as(r.agentKey, http.MethodGet, "/api/orders/"+manual.ID, nil).decode(t, &read)
	assert.Equal(t, "manual", *read.Strategy, "the order keeps its strategy")
	assert.Nil(t, read.PaymentURL)
	var next manualOrder
	c.createTakeaway(r.agentKey, r, 1).decode(t, &next)
	assert.Equal(t, "wompi", *next.Strategy)
	assert.NotNil(t, next.PaymentURL, "the next order is paid through its link")
	var staff manualOrder
	c.createTakeaway(r.managerKey, r, 1).decode(t, &staff)
	assert.Nil(t, staff.Strategy, "a till order has none")
}

func TestPaymentStrategies_WompiWithoutKeysStillRefusesAgentOrders(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute)
	c.join(r)
	c.setPayment(r, c.adminToken(r), map[string]any{"strategy": "wompi"})

	resp := c.createTakeaway(r.agentKey, r, 1)

	require.Equal(t, http.StatusUnprocessableEntity, resp.status)
	assert.Contains(t, string(resp.body), `"code":"PAYMENT_LINKS_NOT_CONFIGURED"`)
}
