package payments

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayments_KeysRotatedWithoutARestart(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)
	const rotatedKey, rotatedSecret = "prv_test_rotated", "test_events_rotated" //nolint:gosec // G101: test fixtures

	c.wompi.acceptOnly(rotatedKey)
	c.putWompi(r, rotatedKey, rotatedSecret)

	resp := c.createTakeaway(r.agentKey, r, 1)
	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created order
	resp.decode(t, &created)
	link := linkID(t, created)
	assert.NotEqual(t, http.StatusOK, c.deliver(r, wompiEvent("tx-old-secret", "APPROVED", link, 4500000, eventsSecret)),
		"an event signed with the old secret is refused")
	assert.Equal(t, http.StatusOK, c.deliver(r, wompiEvent("tx-new-secret", "APPROVED", link, 4500000, rotatedSecret)))
}

func TestPayments_DeletingWompiRefusesTheNextAgentOrder(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)
	path := "/api/platform/organizations/" + r.orgID + "/integrations/wompi"
	admin := map[string]string{"X-API-Key": adminAPIKey}

	read := c.call(http.MethodGet, path, admin, nil)
	require.Equal(t, http.StatusOK, read.status, string(read.body))
	assert.NotContains(t, string(read.body), privateKey)
	assert.NotContains(t, string(read.body), eventsSecret)
	assert.Contains(t, string(read.body), `"webhook_path":"/api/webhooks/wompi/`+r.orgID+`"`)

	require.Equal(t, http.StatusNoContent, c.call(http.MethodDelete, path, admin, nil).status)

	resp := c.createTakeaway(r.agentKey, r, 1)
	require.Equal(t, http.StatusUnprocessableEntity, resp.status, string(resp.body))
	assert.Contains(t, string(resp.body), `"code":"PAYMENT_LINKS_NOT_CONFIGURED"`)
}

func TestPayments_IntegrationRoutesNeedTheAdminKey(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)

	resp := c.as(r.agentKey, http.MethodGet, "/api/platform/organizations/"+r.orgID+"/integrations", nil)

	assert.Equal(t, http.StatusUnauthorized, resp.status)
	assert.NotContains(t, string(resp.body), "wompi")
}
