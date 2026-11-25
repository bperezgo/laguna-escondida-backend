package payments

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/png"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const qrTxID = "12022516-1795575418-68997"

type qrOrder struct {
	order
	PaymentQRURL *string `json:"payment_qr_url"`
}

// putQRKeys sets the Wompi keys with the QR pair, the way the operator does.
func (c *cloud) putQRKeys(r *restaurant) {
	c.t.Helper()
	resp := c.call(http.MethodPut, "/api/platform/organizations/"+r.orgID+"/integrations/wompi", map[string]string{"X-API-Key": adminAPIKey},
		map[string]string{"private_key": privateKey, "events_secret": eventsSecret, "public_key": "pub_test_accept", "integrity_secret": "test_integrity_accept"})
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	assert.Contains(c.t, string(resp.body), `"qr_enabled":true`)
}

func (c *cloud) createTakeawayWithTerms(key string, r *restaurant, accepted bool) response {
	c.t.Helper()
	return c.as(key, http.MethodPost, "/api/orders", map[string]any{
		"open_bill_id": uuid.NewString(), "temporal_identifier": uuid.NewString(), "order_type": "takeaway", "customer_phone": customer,
		"payment_terms_accepted": accepted,
		"products":               []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 1}},
	})
}

// qrPath is the part of the public image URL the cloud serves.
func qrPath(t *testing.T, qrURL *string) string {
	t.Helper()
	require.NotNil(t, qrURL, "the order has a QR")
	parsed, err := url.Parse(*qrURL)
	require.NoError(t, err)
	return parsed.Path
}

// qrEvent is a signed transaction.updated of a QR payment: no payment link, the QR's reference.
func qrEvent(status, reference string, amountInCents int64) []byte {
	const timestamp = 1763485200
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d%d%s", qrTxID, status, amountInCents, timestamp, eventsSecret)))
	return []byte(fmt.Sprintf(`{"event":"transaction.updated","data":{"transaction":{"id":%q,"amount_in_cents":%d,"reference":%q,`+
		`"currency":"COP","payment_method_type":"BANCOLOMBIA_QR","status":%q,"payment_link_id":null}},"environment":"test",`+
		`"signature":{"properties":["transaction.id","transaction.status","transaction.amount_in_cents"],"checksum":%q},"timestamp":%d}`,
		qrTxID, amountInCents, reference, status, hex.EncodeToString(sum[:]), timestamp))
}

func newQRCloud(t *testing.T) (*cloud, *restaurant) {
	t.Helper()
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute)
	c.wompi.qrSVG = loadSandboxQR()
	c.join(r)
	c.putQRKeys(r)
	return c, r
}

func TestPaymentQR_AnOrderIsPaidWithItsQR(t *testing.T) {
	c, r := newQRCloud(t)

	resp := c.createTakeawayWithTerms(r.agentKey, r, true)

	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created qrOrder
	resp.decode(t, &created)
	assert.NotNil(t, created.PaymentURL, "the link always comes first")
	path := qrPath(t, created.PaymentQRURL)

	image := c.call(http.MethodGet, path, nil, nil)
	require.Equal(t, http.StatusOK, image.status)
	_, err := png.Decode(bytes.NewReader(image.body))
	require.NoError(t, err, "a PNG, no credentials needed")

	var read qrOrder
	c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).decode(t, &read)
	assert.Equal(t, created.PaymentQRURL, read.PaymentQRURL, "the detail shows the active QR")

	refs := c.wompi.qrReferences()
	require.Len(t, refs, 1)
	assert.Equal(t, http.StatusOK, c.deliver(r, qrEvent("APPROVED", refs[0], 4500000)))

	assert.Equal(t, http.StatusNotFound, c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).status, "paid")
	var closed order
	c.as(r.managerKey, http.MethodGet, "/api/orders/closed/"+created.ID, nil).decode(t, &closed)
	assert.NotNil(t, closed.PaidAt)
	assert.Equal(t, http.StatusNotFound, c.call(http.MethodGet, path, nil, nil).status, "a paid order's QR is retired")
	var bills int
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM bills WHERE organization_id = ?", r.orgID).Scan(&bills).Error)
	assert.Equal(t, 1, bills)
	assert.Equal(t, http.StatusOK, c.deliver(r, qrEvent("APPROVED", refs[0], 4500000)), "a redelivery is fine")
	require.NoError(t, cloudDB.Raw("SELECT count(*) FROM bills WHERE organization_id = ?", r.orgID).Scan(&bills).Error)
	assert.Equal(t, 1, bills, "still one bill")
}

func TestPaymentQR_WithoutTheTermsThereIsOnlyTheLink(t *testing.T) {
	c, r := newQRCloud(t)

	resp := c.createTakeawayWithTerms(r.agentKey, r, false)

	require.Equal(t, http.StatusCreated, resp.status, string(resp.body))
	var created qrOrder
	resp.decode(t, &created)
	assert.NotNil(t, created.PaymentURL)
	assert.Nil(t, created.PaymentQRURL)
	assert.Empty(t, c.wompi.qrReferences(), "nothing was asked of Wompi")
}

func TestPaymentQR_AChangeRetiresTheOldQR(t *testing.T) {
	c, r := newQRCloud(t)
	var created qrOrder
	c.createTakeawayWithTerms(r.agentKey, r, true).decode(t, &created)
	oldPath := qrPath(t, created.PaymentQRURL)
	oldRefs := c.wompi.qrReferences()

	changed := c.as(r.agentKey, http.MethodPut, "/api/orders/"+created.ID, map[string]any{
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 2}},
	})

	require.Equal(t, http.StatusOK, changed.status, string(changed.body))
	var updated qrOrder
	changed.decode(t, &updated)
	newPath := qrPath(t, updated.PaymentQRURL)
	assert.NotEqual(t, oldPath, newPath)
	assert.Equal(t, http.StatusNotFound, c.call(http.MethodGet, oldPath, nil, nil).status, "the old image is gone")
	assert.Equal(t, http.StatusOK, c.call(http.MethodGet, newPath, nil, nil).status)

	var stale string
	require.Len(t, oldRefs, 1)
	stale = oldRefs[0]
	assert.Equal(t, http.StatusOK, c.deliver(r, qrEvent("APPROVED", stale, 4500000)), "answered, but pays nothing")
	assert.Equal(t, http.StatusOK, c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).status, "the order stays unpaid")
	var matched bool
	require.NoError(t, cloudDB.Raw("SELECT matched FROM payment_link_transactions WHERE wompi_transaction_id = ?", qrTxID).Scan(&matched).Error)
	assert.False(t, matched, "stored as unmatched")
}

func TestPaymentQR_AFailedQRPaymentRetiresItButKeepsTheLink(t *testing.T) {
	c, r := newQRCloud(t)
	var created qrOrder
	c.createTakeawayWithTerms(r.agentKey, r, true).decode(t, &created)
	path := qrPath(t, created.PaymentQRURL)
	refs := c.wompi.qrReferences()
	require.Len(t, refs, 1)

	require.Equal(t, http.StatusOK, c.deliver(r, qrEvent("ERROR", refs[0], 4500000)))

	assert.Equal(t, http.StatusNotFound, c.call(http.MethodGet, path, nil, nil).status)
	var read qrOrder
	c.as(r.agentKey, http.MethodGet, "/api/orders/"+created.ID, nil).decode(t, &read)
	assert.NotNil(t, read.PaymentURL, "the link still pays the order")
	assert.Nil(t, read.PaymentQRURL)
}

func TestPaymentQR_AGuessedTokenIs404(t *testing.T) {
	c, _ := newQRCloud(t)

	assert.Equal(t, http.StatusNotFound, c.call(http.MethodGet, "/api/payments/qr/never-issued", nil, nil).status)
}

func TestPaymentQR_AnIntegrationWithoutTheQRKeysKeepsTheLinkOnly(t *testing.T) {
	r := seedRestaurant(t)
	c := newCloud(t, 30*time.Minute, r)
	c.join(r)

	resp := c.createTakeawayWithTerms(r.agentKey, r, true)

	require.Equal(t, http.StatusCreated, resp.status)
	var created qrOrder
	resp.decode(t, &created)
	assert.NotNil(t, created.PaymentURL)
	assert.Nil(t, created.PaymentQRURL)
}

func TestPaymentQR_ThePaymentTermsAreOnTheSettingsRead(t *testing.T) {
	c, r := newQRCloud(t)

	resp := c.as(r.agentKey, http.MethodGet, "/api/organization/settings", nil)

	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	assert.Contains(t, string(resp.body), `"payment_strategy":"wompi"`)
	assert.Contains(t, string(resp.body), `"terms_url":"https://wompi.com/terms.pdf"`)
	assert.Contains(t, string(resp.body), `"personal_data_url":"https://wompi.com/data.pdf"`)
}

func TestPaymentQR_AManualRestaurantHasNoTerms(t *testing.T) {
	c, r := newQRCloud(t)
	c.setPayment(r, c.adminToken(r), manualSettings(60))

	resp := c.as(r.agentKey, http.MethodGet, "/api/organization/settings", nil)

	require.Equal(t, http.StatusOK, resp.status)
	assert.Contains(t, string(resp.body), `"payment_strategy":"manual"`)
	assert.NotContains(t, string(resp.body), "payment_terms")
}
