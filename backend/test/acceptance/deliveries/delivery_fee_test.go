package deliveries

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Centro around (4.60, -74.08) and Norte around (4.70, -74.04), [lng, lat].
const (
	centroBoundary = `{"type":"Polygon","coordinates":[[[-74.10,4.58],[-74.06,4.58],[-74.06,4.62],[-74.10,4.62],[-74.10,4.58]]]}`
	norteBoundary  = `{"type":"Polygon","coordinates":[[[-74.06,4.68],[-74.02,4.68],[-74.02,4.72],[-74.06,4.72],[-74.06,4.68]]]}`
)

type feeFixture struct {
	c       *cloud
	r       *restaurant
	admin   string
	manager string
}

func newFeeFixture(t *testing.T) *feeFixture {
	c := newCloud(t)
	r := seedRestaurant(t)
	return &feeFixture{
		c: c, r: r,
		admin:   c.token(r, r.seedUser("Dueña", permissions.RoleAdmin), permissions.RoleAdmin),
		manager: c.token(r, r.seedUser("Gerente", permissions.RoleManager), permissions.RoleManager),
	}
}

func (f *feeFixture) createZone(name, boundary string, fee *string) dto.DeliveryZone {
	f.c.t.Helper()
	body := map[string]any{"name": name, "boundary": json.RawMessage(boundary)}
	if fee != nil {
		body["delivery_fee"] = *fee
	}
	status, raw := f.c.post(f.manager, "/api/delivery-zones", body)
	require.Equal(f.c.t, http.StatusCreated, status, string(raw))
	var zone dto.DeliveryZone
	require.NoError(f.c.t, json.Unmarshal(raw, &zone))
	return zone
}

func (f *feeFixture) check(lat, lng string) dto.DeliveryZoneCheckResult {
	f.c.t.Helper()
	status, raw := f.c.get(f.manager, "/api/delivery-zones/check?lat="+lat+"&lng="+lng)
	require.Equal(f.c.t, http.StatusOK, status, string(raw))
	var result dto.DeliveryZoneCheckResult
	require.NoError(f.c.t, json.Unmarshal(raw, &result))
	return result
}

// createDelivery orders one Mojarra frita (45000) delivered at the point, sending a fee the server must ignore.
func (f *feeFixture) createDelivery(lat, lng string) dto.OpenBill {
	f.c.t.Helper()
	status, raw := f.c.post(f.manager, "/api/orders", map[string]any{
		"open_bill_id": uuid.NewString(), "temporal_identifier": uuid.NewString(),
		"products":   []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": f.r.productID, "quantity": 1}},
		"order_type": "delivery", "customer_name": "Ana", "customer_phone": "3005551234",
		"delivery_address": "Cra 7 # 12-34", "delivery_lat": lat, "delivery_lng": lng,
		"delivery_fee": "0",
	})
	require.Equal(f.c.t, http.StatusCreated, status, string(raw))
	var order dto.OpenBill
	require.NoError(f.c.t, json.Unmarshal(raw, &order))
	return order
}

func (f *feeFixture) pay(orderID string) {
	f.c.t.Helper()
	status, raw := f.c.post(f.manager, "/api/orders/pay-order", map[string]any{
		"order_id": orderID, "payment_type": dto.ElectronicInvoicePaymentCodeCash,
		"customer": dto.Customer{DocumentNumber: "222222222222", DocumentType: dto.DocumentTypeNationalIdentificationNumber, Name: "Consumidor final", Email: "cf@example.com"},
	})
	require.Equal(f.c.t, http.StatusCreated, status, string(raw))
}

func requireDecimal(t *testing.T, want string, got *decimal.Decimal, msg string) {
	t.Helper()
	require.NotNil(t, got, msg)
	assert.True(t, decimal.RequireFromString(want).Equal(*got), "%s: want %s, got %s", msg, want, got)
}

func TestCloud_AZoneFeeIsStoredWithItsBaseAndICO(t *testing.T) {
	f := newFeeFixture(t)

	zone := f.createZone("Centro", centroBoundary, strPtr("5000"))
	free := f.createZone("Norte", norteBoundary, nil)

	requireDecimal(t, "5000", &zone.DeliveryFee, "fee")
	requireDecimal(t, "4629.63", &zone.DeliveryFeeBase, "base")
	requireDecimal(t, "370.37", &zone.DeliveryFeeICO, "ico")
	assert.True(t, free.DeliveryFee.IsZero(), "no fee means free delivery")

	status, raw := f.c.get(f.manager, "/api/delivery-zones")
	require.Equal(t, http.StatusOK, status, string(raw))
	var list struct {
		Zones []dto.DeliveryZone `json:"delivery_zones"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Len(t, list.Zones, 2)
	requireDecimal(t, "5000", &list.Zones[0].DeliveryFee, "the list returns the fee")
	requireDecimal(t, "370.37", &list.Zones[0].DeliveryFeeICO, "and its ICO")

	status, raw = f.c.post(f.manager, "/api/delivery-zones", map[string]any{
		"name": "Sur", "boundary": json.RawMessage(norteBoundary), "delivery_fee": "-1000",
	})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(raw), "DELIVERY_ZONE_INVALID_FEE")
}

func TestCloud_ThePointCheckQuotesTheZoneFee(t *testing.T) {
	f := newFeeFixture(t)
	f.createZone("Centro", centroBoundary, strPtr("5000"))
	f.createZone("Norte", norteBoundary, strPtr("0"))

	paid := f.check("4.60", "-74.08")
	assert.True(t, paid.Deliverable)
	assert.Equal(t, "Centro", *paid.ZoneName)
	requireDecimal(t, "5000", paid.DeliveryFee, "paid zone")

	free := f.check("4.70", "-74.04")
	assert.True(t, free.Deliverable)
	requireDecimal(t, "0", free.DeliveryFee, "free zone")

	outside := f.check("4.90", "-74.30")
	assert.False(t, outside.Deliverable)
	assert.Nil(t, outside.ZoneName)
	assert.Nil(t, outside.DeliveryFee)
}

func TestCloud_AnAgentCannotChangeAZoneFee(t *testing.T) {
	f := newFeeFixture(t)
	zone := f.createZone("Centro", centroBoundary, strPtr("5000"))
	agent := f.c.token(f.r, f.r.seedUser("Agente", permissions.RoleAgent), permissions.RoleAgent)

	status, _ := f.c.do(http.MethodPut, agent, "/api/delivery-zones/"+zone.ID, jsonBody(t, map[string]any{"delivery_fee": "0"}))

	assert.Equal(t, http.StatusForbidden, status)
	var fee decimal.Decimal
	require.NoError(t, cloudDB.Raw(`SELECT delivery_fee FROM delivery_zones WHERE id = ?`, zone.ID).Scan(&fee).Error)
	assert.True(t, decimal.NewFromInt(5000).Equal(fee), "the zone is unchanged")
}

func TestCloud_ADeliveryOrderCarriesItsFeeFromCreationToTheDailyClose(t *testing.T) {
	f := newFeeFixture(t)
	zone := f.createZone("Centro", centroBoundary, strPtr("5000"))
	f.createZone("Norte", norteBoundary, strPtr("8000"))

	created := f.createDelivery("4.60", "-74.08")
	requireDecimal(t, "5000", created.DeliveryFee, "the zone's fee, not the one sent")
	assert.True(t, decimal.NewFromInt(50000).Equal(created.TotalAmount), "45000 + 5000, got %s", created.TotalAmount)

	status, raw := f.c.do(http.MethodPut, f.manager, "/api/delivery-zones/"+zone.ID, jsonBody(t, map[string]any{"delivery_fee": "6000"}))
	require.Equal(t, http.StatusOK, status, string(raw))

	status, raw = f.c.get(f.manager, "/api/orders?order_type=delivery")
	require.Equal(t, http.StatusOK, status, string(raw))
	var active dto.OpenBillListResponse
	require.NoError(t, json.Unmarshal(raw, &active))
	require.Len(t, active.OpenBills, 1)
	requireDecimal(t, "5000", active.OpenBills[0].DeliveryFee, "active list, after the zone was repriced")
	assert.True(t, decimal.NewFromInt(50000).Equal(active.OpenBills[0].TotalAmount))

	status, raw = f.c.get(f.manager, "/api/orders/"+created.ID)
	require.Equal(t, http.StatusOK, status, string(raw))
	var single dto.OpenBillWithProducts
	require.NoError(t, json.Unmarshal(raw, &single))
	requireDecimal(t, "5000", single.DeliveryFee, "single order")
	require.Len(t, single.Products, 1, "the fee is not a line")

	f.pay(created.ID)
	f.pay(f.createDelivery("4.70", "-74.04").ID)

	status, raw = f.c.get(f.manager, "/api/orders/closed?order_type=delivery")
	require.Equal(t, http.StatusOK, status, string(raw))
	var closed dto.OpenBillListResponse
	require.NoError(t, json.Unmarshal(raw, &closed))
	require.Len(t, closed.OpenBills, 2)
	for _, o := range closed.OpenBills {
		require.NotNil(t, o.DeliveryFee, "closed list")
	}

	status, raw = f.c.get(f.manager, "/api/orders/closed/"+created.ID)
	require.Equal(t, http.StatusOK, status, string(raw))
	var closedSingle dto.OpenBillWithProducts
	require.NoError(t, json.Unmarshal(raw, &closedSingle))
	requireDecimal(t, "5000", closedSingle.DeliveryFee, "single closed order")

	var bill struct {
		PayAmount   decimal.Decimal
		ICO         decimal.Decimal `gorm:"column:ico"`
		DeliveryFee decimal.Decimal
	}
	require.NoError(t, cloudDB.Raw(`SELECT pay_amount, ico, delivery_fee FROM bills
		WHERE organization_id = ? AND delivery_fee = 5000`, f.r.orgID).Scan(&bill).Error)
	assert.True(t, decimal.NewFromInt(50000).Equal(bill.PayAmount), "the bill was paid with the fee, got %s", bill.PayAmount)
	assert.True(t, decimal.RequireFromString("370.37").Equal(bill.ICO), "the plate has no ICO; only the fee's, got %s", bill.ICO)

	today := time.Now().In(time.FixedZone("COT", -5*3600)).Format("2006-01-02")
	status, raw = f.c.get(f.admin, "/api/reports/daily-close?date="+today)
	require.Equal(t, http.StatusOK, status, string(raw))
	var report dto.DailyCloseReport
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.True(t, decimal.NewFromInt(13000).Equal(report.TotalDeliveryFee), "5000 + 8000, got %s", report.TotalDeliveryFee)
	assert.True(t, decimal.RequireFromString("12037.04").Equal(report.TotalDeliveryFeeBase), "got %s", report.TotalDeliveryFeeBase)
	assert.True(t, decimal.RequireFromString("962.96").Equal(report.TotalDeliveryFeeICO), "got %s", report.TotalDeliveryFeeICO)
	assert.True(t, decimal.NewFromInt(103000).Equal(report.TotalCollected), "the collected total includes the fees, got %s", report.TotalCollected)
}

func TestCloud_ADayWithoutDeliveriesReportsNoFees(t *testing.T) {
	f := newFeeFixture(t)

	status, raw := f.c.get(f.admin, "/api/reports/daily-close")
	require.Equal(t, http.StatusOK, status, string(raw))
	var report dto.DailyCloseReport
	require.NoError(t, json.Unmarshal(raw, &report))
	assert.True(t, report.TotalDeliveryFee.IsZero())
	assert.True(t, report.TotalDeliveryFeeBase.IsZero())
	assert.True(t, report.TotalDeliveryFeeICO.IsZero())
}

func jsonBody(t *testing.T, body any) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return bytes.NewReader(raw)
}
