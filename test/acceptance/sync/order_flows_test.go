package sync

import (
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

// Order lifecycle acceptance tests: one test per flow in
// docs/playbooks/ORDER_FLOWS_ACCEPTANCE_SPEC.md, one subtest per numbered step, so
// `go test -v -run TestOrderFlow` reads as the spec. Every step goes over HTTP as the role
// that does it in the restaurant; payment is recorded by staff (simulated, as it is manual).

type flowRig struct {
	*deliveryAPI
	agentID string
	second  string // a second product, so an edit can add a line
}

func newFlowRig(t *testing.T) *flowRig {
	a := newDeliveryAPI(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	agent := newUser("whatsapp-agent", "$2a$10$abcdefghijklmnopqrstuvexamplehashvalue00000000000000", now)
	second := newProduct("SKU-FLOW-2", "Limonada", now)
	a.seedCloudUsers(agent)
	a.seedCloudProducts(second)
	a.seedCloudProductResponsibilities(newProductResponsibility(second.ID, kitchenArea, 1, now))
	return &flowRig{deliveryAPI: a, agentID: agent.ID, second: second.ID}
}

// actor is someone calling the API: a user id plus the role their token carries.
type actor struct {
	userID string
	role   int
}

func (f *flowRig) staff(role int) actor { return actor{f.userID, role} }
func (f *flowRig) agent() actor         { return actor{f.agentID, permissions.RoleAgent} }

type apiResponse struct {
	status int
	body   []byte
}

func (r apiResponse) json(t *testing.T) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(r.body, &out), string(r.body))
	return out
}

func (f *flowRig) do(t *testing.T, who actor, method, path string, body any) apiResponse {
	t.Helper()
	original := f.userID
	f.userID = who.userID
	defer func() { f.userID = original }()
	status, raw := f.call(who.role, method, path, body)
	return apiResponse{status, raw}
}

type line struct {
	ID, ProductID string
	Quantity      int
}

func lines(ls ...line) []map[string]any {
	out := make([]map[string]any, len(ls))
	for i, l := range ls {
		out[i] = map[string]any{"open_bill_product_id": l.ID, "product_id": l.ProductID, "quantity": l.Quantity}
	}
	return out
}

func newLine(productID string, qty int) line { return line{uuid.NewString(), productID, qty} }

func takeawayContact() map[string]any {
	return map[string]any{"order_type": "takeaway", "customer_name": "Ana", "customer_phone": "3005551234"}
}

func deliveryContactJSON() map[string]any {
	return map[string]any{
		"order_type": "delivery", "customer_name": "Ana", "customer_phone": "3005551234",
		"delivery_address": "Cra 7 # 12-34, apto 501", "delivery_lat": insideLat, "delivery_lng": insideLng,
	}
}

func (f *flowRig) createOrder(t *testing.T, who actor, contact map[string]any, ls ...line) (string, apiResponse) {
	t.Helper()
	id := uuid.NewString()
	body := map[string]any{"open_bill_id": id, "temporal_identifier": uuid.NewString(), "products": lines(ls...)}
	for k, v := range contact {
		body[k] = v
	}
	return id, f.do(t, who, http.MethodPost, "/api/orders", body)
}

func (f *flowRig) lineStatuses(t *testing.T, orderID string) map[string]string {
	t.Helper()
	res := f.do(t, f.staff(permissions.RoleManager), http.MethodGet, "/api/orders/"+orderID, nil)
	require.Equal(t, http.StatusOK, res.status, string(res.body))
	var order struct {
		Products []struct {
			ID     string `json:"open_bill_product_id"`
			Status string `json:"status"`
		} `json:"products"`
	}
	require.NoError(t, json.Unmarshal(res.body, &order))
	out := map[string]string{}
	for _, p := range order.Products {
		out[p.ID] = p.Status
	}
	return out
}

func (f *flowRig) activeIDs(t *testing.T) []string {
	t.Helper()
	res := f.do(t, f.staff(permissions.RoleManager), http.MethodGet, "/api/orders", nil)
	require.Equal(t, http.StatusOK, res.status, string(res.body))
	var list struct {
		OpenBills []struct {
			ID string `json:"id"`
		} `json:"open_bills"`
	}
	require.NoError(t, json.Unmarshal(res.body, &list))
	ids := make([]string, len(list.OpenBills))
	for i, b := range list.OpenBills {
		ids[i] = b.ID
	}
	return ids
}

func (f *flowRig) kitchen(t *testing.T, who actor, action, orderID, lineID string) apiResponse {
	t.Helper()
	return f.do(t, who, http.MethodPatch, "/api/orders/"+orderID+"/products/"+lineID+"/"+action, nil)
}

func amount(t *testing.T, v any) decimal.Decimal {
	t.Helper()
	text, ok := v.(string)
	require.True(t, ok, "amount %v is not a string", v)
	d, err := decimal.NewFromString(text)
	require.NoError(t, err)
	return d
}

// step runs one numbered step of a flow and stops the flow at the first failing step, so
// later steps never report on a state the earlier ones didn't reach.
func step(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}

// FLOW-DEL and FLOW-TKW: the same lifecycle for both order types that wait for payment.
func TestOrderFlow_TakeawayAndDeliveryLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name    string
		contact func() map[string]any
	}{
		{"delivery", deliveryContactJSON},
		{"takeaway", takeawayContact},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runLifecycle(t, newFlowRig(t), tt.name, tt.contact(), func(f *flowRig) actor { return f.staff(permissions.RoleServer) })
		})
	}
}

// FLOW-AGT-01: the agent drives the same lifecycle on its own orders; staff pay and cook.
func TestOrderFlow_AgentLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name    string
		contact func() map[string]any
	}{
		{"delivery", deliveryContactJSON},
		{"takeaway", takeawayContact},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runLifecycle(t, newFlowRig(t), tt.name, tt.contact(), (*flowRig).agent)
		})
	}
}

func runLifecycle(t *testing.T, f *flowRig, orderType string, contact map[string]any, taker func(*flowRig) actor) {
	who := taker(f)
	cook, manager := f.staff(permissions.RoleCooker), f.staff(permissions.RoleManager)
	first, toRemove := newLine(f.product, 1), newLine(f.product, 1)
	var orderID string
	var added, toCancel line
	var createdTotal decimal.Decimal

	step(t, "01 the order is created unpaid and off the kitchen board", func(t *testing.T) {
		var res apiResponse
		orderID, res = f.createOrder(t, who, contact, first, toRemove)
		require.Equal(t, http.StatusCreated, res.status, string(res.body))
		order := res.json(t)
		assert.Equal(t, orderType, order["order_type"])
		assert.Equal(t, "+573005551234", order["customer_phone"])
		assert.Nil(t, order["paid_at"], "not paid yet")
		if orderType == "delivery" {
			assert.Equal(t, f.zoneID, order["delivery_zone_id"], "the pin is inside Centro")
		}
		createdTotal = amount(t, order["total_amount"])
		assert.NotContains(t, pendingLineIDs(t, f.rig, f.openBill), first.ID, "held until paid")
	})

	step(t, "02 the order is edited: quantity changed, a line removed, a product added", func(t *testing.T) {
		first.Quantity = 3
		added = newLine(f.second, 1)
		res := f.do(t, who, http.MethodPut, "/api/orders/"+orderID, map[string]any{"products": lines(first, added)})
		require.Equal(t, http.StatusOK, res.status, string(res.body))
		order := res.json(t)
		assert.True(t, amount(t, order["total_amount"]).GreaterThan(createdTotal), "the total follows the edit")
		assert.Equal(t, orderType, order["order_type"], "an edit keeps the type and contact")
		assert.Equal(t, "+573005551234", order["customer_phone"])

		statuses := f.lineStatuses(t, orderID)
		assert.Len(t, statuses, 2)
		assert.NotContains(t, statuses, toRemove.ID, "the removed line is gone")
		assert.Contains(t, statuses, added.ID, "the new product is on the order")
	})

	step(t, "03 one line is cancelled", func(t *testing.T) {
		toCancel = newLine(f.product, 1)
		res := f.do(t, who, http.MethodPut, "/api/orders/"+orderID, map[string]any{"products": lines(first, added, toCancel)})
		require.Equal(t, http.StatusOK, res.status, string(res.body))

		res = f.do(t, who, http.MethodPatch, "/api/orders/"+orderID+"/products/"+toCancel.ID+"/cancel", nil)
		require.Equal(t, http.StatusOK, res.status, string(res.body))
		assert.Equal(t, "cancelled", f.lineStatuses(t, orderID)[toCancel.ID])
	})

	step(t, "04 the kitchen cannot start an unpaid order, whoever asks", func(t *testing.T) {
		for _, kitchenActor := range []actor{cook, manager} {
			for _, action := range []string{"in-progress", "complete"} {
				res := f.kitchen(t, kitchenActor, action, orderID, first.ID)
				assert.Equal(t, http.StatusConflict, res.status, "role %d %s: %s", kitchenActor.role, action, res.body)
				assert.Contains(t, string(res.body), "ORDER_AWAITING_PAYMENT")
			}
		}
		assert.Equal(t, "created", f.lineStatuses(t, orderID)[first.ID], "the line did not move")
	})

	step(t, "05 staff record the payment; the order closes", func(t *testing.T) {
		f.pay(orderID)
		assert.NotContains(t, f.activeIDs(t), orderID, "it left the active list")
		res := f.do(t, manager, http.MethodGet, "/api/orders/closed/"+orderID, nil)
		require.Equal(t, http.StatusOK, res.status, string(res.body))
		assert.NotNil(t, res.json(t)["paid_at"])
	})

	step(t, "06 the kitchen prepares every live line", func(t *testing.T) {
		board := pendingLineIDs(t, f.rig, f.openBill)
		assert.Contains(t, board, first.ID, "payment released it to the board")
		assert.Contains(t, board, added.ID)
		assert.NotContains(t, board, toCancel.ID, "a cancelled line never reaches the kitchen")

		for _, l := range []line{first, added} {
			res := f.kitchen(t, cook, "in-progress", orderID, l.ID)
			require.Equal(t, http.StatusOK, res.status, string(res.body))
			res = f.kitchen(t, cook, "complete", orderID, l.ID)
			require.Equal(t, http.StatusOK, res.status, string(res.body))
		}
		board = pendingLineIDs(t, f.rig, f.openBill)
		assert.NotContains(t, board, first.ID, "done lines leave the board")
		assert.NotContains(t, board, added.ID)
	})

	step(t, "07 a paid order is locked against changes", func(t *testing.T) {
		edit := f.do(t, who, http.MethodPut, "/api/orders/"+orderID, map[string]any{"products": lines(first)})
		assert.Equal(t, http.StatusNotFound, edit.status, "edit: %s", edit.body)
		cancel := f.do(t, who, http.MethodPatch, "/api/orders/"+orderID+"/products/"+first.ID+"/cancel", nil)
		assert.Equal(t, http.StatusConflict, cancel.status, "cancel a line: %s", cancel.body)
		eventsBefore := len(f.bus.events)
		del := f.do(t, manager, http.MethodDelete, "/api/orders/"+orderID, nil)
		assert.Equal(t, http.StatusNotFound, del.status, "delete: %s", del.body)
		// A void of a sold order would hand its stock back and pull its lines off the board.
		for _, e := range f.bus.events[eventsBefore:] {
			_, isDelete := e.(dto.OrderDeletedEvent)
			assert.False(t, isDelete, "no order-deleted event (stock restore, board removal) for a paid order")
		}
		assert.Equal(t, int64(0), f.cloudCount("sync_outbox", "entity_id = ? AND operation = 'delete'", orderID),
			"no delete is replicated for a paid order")
	})
}

// FLOW-VOID: an unpaid order is cancelled as a whole.
func TestOrderFlow_UnpaidOrderIsVoided(t *testing.T) {
	for _, tt := range []struct {
		name  string
		taker func(*flowRig) actor
	}{
		// Voiding a whole order is a manager decision: servers have no orders:delete.
		{"by a manager", func(f *flowRig) actor { return f.staff(permissions.RoleManager) }},
		{"by the agent", (*flowRig).agent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFlowRig(t)
			who := tt.taker(f)
			l := newLine(f.product, 2)
			orderID, res := f.createOrder(t, who, deliveryContactJSON(), l)
			require.Equal(t, http.StatusCreated, res.status, string(res.body))

			res = f.do(t, who, http.MethodDelete, "/api/orders/"+orderID, nil)
			require.Equal(t, http.StatusOK, res.status, string(res.body))

			assert.NotContains(t, f.activeIDs(t), orderID, "it left the active list")
			for _, action := range []string{"in-progress", "complete", "cancel"} {
				res = f.kitchen(t, f.staff(permissions.RoleManager), action, orderID, l.ID)
				assert.Equal(t, http.StatusNotFound, res.status, "%s on a voided order: %s", action, res.body)
			}

			eventsBefore := len(f.bus.events)
			res = f.do(t, f.staff(permissions.RoleManager), http.MethodDelete, "/api/orders/"+orderID, nil)
			assert.Equal(t, http.StatusNotFound, res.status, "voiding twice: %s", res.body)
			assert.Len(t, f.bus.events, eventsBefore, "a second void must not restore the stock again")
		})
	}
}

// FLOW-VOID: an id that is not an open order is not found, not an empty order.
func TestOrderFlow_UnknownOrderIsNotFound(t *testing.T) {
	f := newFlowRig(t)
	unknown := uuid.NewString()
	manager := f.staff(permissions.RoleManager)
	eventsBefore := len(f.bus.events)

	for name, res := range map[string]apiResponse{
		"read":          f.do(t, manager, http.MethodGet, "/api/orders/"+unknown, nil),
		"read (closed)": f.do(t, manager, http.MethodGet, "/api/orders/closed/"+unknown, nil),
		"delete":        f.do(t, manager, http.MethodDelete, "/api/orders/"+unknown, nil),
	} {
		assert.Equal(t, http.StatusNotFound, res.status, "%s: %s", name, res.body)
	}
	assert.Len(t, f.bus.events, eventsBefore, "nothing was published for an order that does not exist")
	assert.Equal(t, int64(0), f.cloudCount("sync_outbox", "entity_id = ?", unknown), "nothing was replicated")
}

// FLOW-AGT-02: the agent never takes money nor moves the kitchen.
func TestOrderFlow_AgentRefusedStaffActions(t *testing.T) {
	f := newFlowRig(t)
	l := newLine(f.product, 1)
	orderID, res := f.createOrder(t, f.agent(), takeawayContact(), l)
	require.Equal(t, http.StatusCreated, res.status, string(res.body))

	res = f.do(t, f.agent(), http.MethodPost, "/api/orders/pay-order", map[string]any{"order_id": orderID, "payment_type": "10"})
	assert.Equal(t, http.StatusForbidden, res.status, "pay: %s", res.body)
	for _, action := range []string{"in-progress", "complete", "uncomplete"} {
		res = f.kitchen(t, f.agent(), action, orderID, l.ID)
		assert.Equal(t, http.StatusForbidden, res.status, "%s: %s", action, res.body)
	}
	assert.Equal(t, int64(0), f.cloudCount("open_bills", "id = ? AND paid_at IS NOT NULL", orderID), "nothing was paid")
	assert.Equal(t, "created", f.lineStatuses(t, orderID)[l.ID], "the kitchen line did not move")
}

// FLOW-AGT-03: the agent cannot open a dine-in order.
func TestOrderFlow_AgentCannotCreateDineIn(t *testing.T) {
	f := newFlowRig(t)
	for name, contact := range map[string]map[string]any{
		"explicit dine_in":   {"order_type": "dine_in"},
		"order_type omitted": {},
	} {
		t.Run(name, func(t *testing.T) {
			orderID, res := f.createOrder(t, f.agent(), contact, newLine(f.product, 1))
			assert.Equal(t, http.StatusForbidden, res.status, string(res.body))
			assert.Contains(t, string(res.body), "AGENT_ORDER_TYPE_NOT_ALLOWED")
			assert.Equal(t, int64(0), f.cloudCount("open_bills", "id = ?", orderID), "nothing was created")
		})
	}
}

// FLOW-AGT-04: the agent cannot touch an order it did not create, nor any dine-in order.
func TestOrderFlow_AgentCannotChangeOthersOrders(t *testing.T) {
	f := newFlowRig(t)
	server := f.staff(permissions.RoleServer)

	for name, contact := range map[string]map[string]any{
		"a dine-in table":        {"order_type": "dine_in"},
		"a staff-taken takeaway": takeawayContact(),
		"a staff-taken delivery": deliveryContactJSON(),
	} {
		t.Run(name, func(t *testing.T) {
			l := newLine(f.product, 1)
			orderID, res := f.createOrder(t, server, contact, l)
			require.Equal(t, http.StatusCreated, res.status, string(res.body))

			attempts := map[string]apiResponse{
				"edit":          f.do(t, f.agent(), http.MethodPut, "/api/orders/"+orderID, map[string]any{"products": lines(newLine(f.product, 5))}),
				"cancel a line": f.do(t, f.agent(), http.MethodPatch, "/api/orders/"+orderID+"/products/"+l.ID+"/cancel", nil),
				"delete":        f.do(t, f.agent(), http.MethodDelete, "/api/orders/"+orderID, nil),
			}
			for attempt, res := range attempts {
				assert.Equal(t, http.StatusForbidden, res.status, "%s: %s", attempt, res.body)
				assert.Contains(t, string(res.body), "ORDER_NOT_OWNED", attempt)
			}

			assert.Contains(t, f.activeIDs(t), orderID, "the order is still open")
			assert.Equal(t, map[string]string{l.ID: "created"}, f.lineStatuses(t, orderID), "its lines are untouched")
		})
	}
}
