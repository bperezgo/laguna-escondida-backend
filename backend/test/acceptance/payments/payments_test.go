package payments

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayments_NoPaymentLinksNoAgentOrder(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute)
	c.join(r)

	resp := c.createTakeaway(r.agentKey, r, 1)

	require.Equal(t, http.StatusUnprocessableEntity, resp.status, string(resp.body))
	assert.Contains(t, string(resp.body), `"code":"PAYMENT_LINKS_NOT_CONFIGURED"`)
	var list struct {
		OpenBills []order `json:"open_bills"`
	}
	c.as(r.managerKey, http.MethodGet, "/api/orders", nil).decode(t, &list)
	assert.Empty(t, list.OpenBills, "no order was created")

	staff := c.createTakeaway(r.managerKey, r, 1)
	require.Equal(t, http.StatusCreated, staff.status, "staff orders are untouched: %s", staff.body)
}

func TestPayments_CreateChangeAndPayThroughTheLink(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)

	resp := c.createTakeaway(r.agentKey, r, 1)
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created order
	resp.decode(t, &created)
	first := linkID(t, created)

	var read order
	c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).decode(t, &read)
	assert.Equal(t, created.PaymentURL, read.PaymentURL, "the detail shows the active link")

	staffOrder := c.createTakeaway(r.managerKey, r, 1)
	var staffCreated order
	staffOrder.decode(t, &staffCreated)
	assert.Nil(t, staffCreated.PaymentURL, "a till order gets no link")

	changed := c.as(r.agentKey, http.MethodPut, "/api/orders/"+created.ID, map[string]any{
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 2}},
	})
	require.Equal(t, http.StatusOK, changed.status, string(changed.body))
	var updated order
	changed.decode(t, &updated)
	second := linkID(t, updated)
	assert.NotEqual(t, first, second)
	assert.True(t, c.wompi.wasDeactivated(first), "the replaced link is deactivated at Wompi")

	assert.Equal(t, http.StatusUnauthorized, c.deliver(r, wompiEvent("tx-forged", "APPROVED", second, 9000000, "not-the-secret")))

	assert.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-old", "APPROVED", first, 4500000, eventsSecret)))
	require.Equal(t, http.StatusOK, c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).status,
		"a payment of the replaced link pays nothing")
	var matched bool
	require.NoError(t, cloudDB.Raw("SELECT matched FROM payment_link_transactions WHERE wompi_transaction_id = 'tx-old'").Scan(&matched).Error)
	assert.False(t, matched, "stored as unmatched")

	assert.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-1", "PENDING", second, 9000000, eventsSecret)))
	assert.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-1", "APPROVED", second, 9000000, eventsSecret)))
	assert.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-1", "APPROVED", second, 9000000, eventsSecret)), "a redelivery is fine")

	assert.Equal(t, http.StatusNotFound, c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).status, "paid: no longer open")
	var closed order
	c.as(r.managerKey, http.MethodGet, "/api/orders/closed/"+created.ID, nil).decode(t, &closed)
	assert.NotNil(t, closed.PaidAt)
	var bills int
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM bills WHERE organization_id = ?", r.orgID).Scan(&bills).Error)
	assert.Equal(t, 1, bills, "one bill, however often Wompi delivers")
	var transactions int
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM payment_link_transactions WHERE wompi_transaction_id = 'tx-1'").Scan(&transactions).Error)
	assert.Equal(t, 1, transactions)
}

func TestPayments_AnUnpaidOrderExpires(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, time.Second, r)
	c.join(r)

	var expiring, held order
	c.createTakeaway(r.agentKey, r, 1).decode(t, &expiring)
	c.createTakeaway(r.agentKey, r, 1).decode(t, &held)
	require.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-pending", "PENDING", linkID(t, held), 4500000, eventsSecret)))
	time.Sleep(1500 * time.Millisecond)

	c.expireDue()

	assert.Equal(t, http.StatusNotFound, c.as(r.agentKey, http.MethodGet, "/api/orders/"+expiring.ID, nil).status)
	var closed order
	c.as(r.managerKey, http.MethodGet, "/api/orders/closed/"+expiring.ID, nil).decode(t, &closed)
	require.NotNil(t, closed.VoidReason)
	assert.Equal(t, "payment_expired", *closed.VoidReason)
	assert.Nil(t, closed.PaidAt)
	assert.True(t, c.wompi.wasDeactivated(linkID(t, expiring)))
	var status string
	require.NoError(t, cloudDB.Raw("SELECT status FROM payment_links WHERE order_id = ?", expiring.ID).Scan(&status).Error)
	assert.Equal(t, "expired", status)

	assert.Equal(t, http.StatusOK, c.as(r.agentKey, http.MethodGet, "/api/orders/"+held.ID, nil).status, "a pending payment holds the order")
	require.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-pending", "DECLINED", linkID(t, held), 4500000, eventsSecret)))
	c.expireDue()
	assert.Equal(t, http.StatusNotFound, c.as(r.agentKey, http.MethodGet, "/api/orders/"+held.ID, nil).status, "declined: voided on the next run")
}

func TestPayments_CancellingDeactivatesTheLink(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)
	var created order
	c.createTakeaway(r.agentKey, r, 1).decode(t, &created)

	require.Equal(t, http.StatusOK, c.as(r.agentKey, http.MethodDelete, "/api/orders/"+created.ID, nil).status)

	assert.True(t, c.wompi.wasDeactivated(linkID(t, created)))
	var status string
	require.NoError(t, cloudDB.Raw("SELECT status FROM payment_links WHERE order_id = ?", created.ID).Scan(&status).Error)
	assert.Equal(t, "cancelled", status)
	var closed order
	c.as(r.managerKey, http.MethodGet, "/api/orders/closed/"+created.ID, nil).decode(t, &closed)
	assert.Nil(t, closed.VoidReason, "a cancel is not an expiry")
}

func TestPayments_WompiDownLeavesTheOrderWithoutALink(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, time.Second, r)
	c.join(r)
	c.wompi.fail = true

	resp := c.createTakeaway(r.agentKey, r, 1)

	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created order
	resp.decode(t, &created)
	assert.Nil(t, created.PaymentURL)
	time.Sleep(1500 * time.Millisecond)
	c.expireDue()
	assert.Equal(t, http.StatusOK, c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).status, "never expires without a link")
}
