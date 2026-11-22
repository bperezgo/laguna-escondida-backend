package deliveries

import (
	"encoding/json"
	"net/http"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloud_AZoneKeepsItsOwnICORate(t *testing.T) {
	f := newFeeFixture(t)

	status, raw := f.c.post(f.manager, "/api/delivery-zones", map[string]any{
		"name": "Norte", "boundary": json.RawMessage(norteBoundary), "delivery_fee": "5000", "delivery_fee_ico_rate": "0.19",
	})
	require.Equal(t, http.StatusCreated, status, string(raw))
	var created dto.DeliveryZone
	require.NoError(t, json.Unmarshal(raw, &created))
	defaulted := f.createZone("Centro", centroBoundary, strPtr("5000"))

	status, raw = f.c.get(f.manager, "/api/delivery-zones")
	require.Equal(t, http.StatusOK, status, string(raw))
	var list struct {
		Zones []dto.DeliveryZone `json:"delivery_zones"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	var zone dto.DeliveryZone
	for _, z := range list.Zones {
		if z.ID == created.ID {
			zone = z
		}
	}
	require.Equal(t, created.ID, zone.ID, "the list returns the zone")
	requireDecimal(t, "0.19", &zone.DeliveryFeeICORate, "rate")
	requireDecimal(t, "4201.68", &zone.DeliveryFeeBase, "base")
	requireDecimal(t, "798.32", &zone.DeliveryFeeICO, "ico")
	requireDecimal(t, "0.08", &defaulted.DeliveryFeeICORate, "no rate means 0.08")

	status, raw = f.c.post(f.manager, "/api/delivery-zones", map[string]any{
		"name": "Sur", "boundary": json.RawMessage(norteBoundary), "delivery_fee_ico_rate": "8",
	})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, string(raw), "DELIVERY_ZONE_INVALID_ICO_RATE")
}

func TestCloud_AnAgentCannotChangeAZoneICORate(t *testing.T) {
	f := newFeeFixture(t)
	zone := f.createZone("Centro", centroBoundary, strPtr("5000"))
	agent := f.c.token(f.r, f.r.seedUser("Agente", permissions.RoleAgent), permissions.RoleAgent)

	status, _ := f.c.do(http.MethodPut, agent, "/api/delivery-zones/"+zone.ID, jsonBody(t, map[string]any{"delivery_fee_ico_rate": "0.19"}))

	assert.Equal(t, http.StatusForbidden, status)
	var rate decimal.Decimal
	require.NoError(t, cloudDB.Raw(`SELECT delivery_fee_ico_rate FROM delivery_zones WHERE id = ?`, zone.ID).Scan(&rate).Error)
	assert.True(t, decimal.RequireFromString("0.08").Equal(rate), "the zone is unchanged, got %s", rate)
}

// The order and its bill keep the rate the zone had when the order was created.
func TestCloud_ADeliveryOrderRecordsTheZoneICORate(t *testing.T) {
	f := newFeeFixture(t)
	zone := f.createZone("Centro", centroBoundary, strPtr("5000"))

	created := f.createDelivery("4.60", "-74.08")
	requireDecimal(t, "0.08", created.DeliveryFeeICORate, "created order")

	status, raw := f.c.do(http.MethodPut, f.manager, "/api/delivery-zones/"+zone.ID, jsonBody(t, map[string]any{"delivery_fee_ico_rate": "0.19"}))
	require.Equal(t, http.StatusOK, status, string(raw))

	status, raw = f.c.get(f.manager, "/api/orders/"+created.ID)
	require.Equal(t, http.StatusOK, status, string(raw))
	var single dto.OpenBillWithProducts
	require.NoError(t, json.Unmarshal(raw, &single))
	requireDecimal(t, "0.08", single.DeliveryFeeICORate, "after the zone's rate changed")
	requireDecimal(t, "370.37", single.DeliveryFeeICO, "and its ICO")

	f.pay(created.ID)
	var billRate decimal.Decimal
	require.NoError(t, cloudDB.Raw(`SELECT delivery_fee_ico_rate FROM bills WHERE organization_id = ? AND delivery_fee = 5000`,
		f.r.orgID).Scan(&billRate).Error)
	assert.True(t, decimal.RequireFromString("0.08").Equal(billRate), "the bill carries the order's rate, got %s", billRate)

	next := f.createDelivery("4.60", "-74.08")
	requireDecimal(t, "0.19", next.DeliveryFeeICORate, "the next order gets the new rate")
	requireDecimal(t, "798.32", next.DeliveryFeeICO, "and its ICO")
}
