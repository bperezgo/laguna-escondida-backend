package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	pkgports "laguna-escondida/backend/pkg/domain/ports"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Centro: a square around Bogotá's center, [lng, lat].
const (
	centroBoundary = `{"type":"Polygon","coordinates":[[[-74.10,4.58],[-74.06,4.58],[-74.06,4.62],[-74.10,4.62],[-74.10,4.58]]]}`
	insideLat      = "4.590000"
	insideLng      = "-74.095000"
	agentPhone     = "+573005551234"
)

type recordingBus struct {
	events []pkgports.Event
}

func (b *recordingBus) Publish(_ context.Context, e pkgports.Event) error {
	b.events = append(b.events, e)
	return nil
}

func (b *recordingBus) released() []dto.OrderReleasedToKitchenEvent {
	var out []dto.OrderReleasedToKitchenEvent
	for _, e := range b.events {
		if r, ok := e.(dto.OrderReleasedToKitchenEvent); ok {
			out = append(out, r)
		}
	}
	return out
}

type deliveryRig struct {
	*rig
	orders   *service.OrderService
	zones    *service.DeliveryZoneService
	openBill ports.OpenBillRepository
	bus      *recordingBus
	userID   string
	product  string
	zoneID   string
}

func newDeliveryRig(t *testing.T) *deliveryRig {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)
	r.seedCloudProductResponsibilities(newProductResponsibility(productID, kitchenArea, 1, time.Now().UTC()))

	db := r.cloudDB
	openBills := repository.NewOpenBillRepository(db)
	zoneRepo := repository.NewDeliveryZoneRepository(db)
	bus := &recordingBus{}
	orders := service.NewOrderServiceWithSSE(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		openBills,
		repository.NewProductRepository(db),
		repository.NewBillRepository(db),
		repository.NewPendingInvoiceRepository(db),
		dto.PendingInvoiceStatusPending,
		repository.NewBillOwnerRepository(db),
		postgres.NewUnitOfWork(db),
		bus,
		nil, nil,
		repository.NewSyncOutboxRepository(db),
		r.identity,
		repository.NewProductIngredientRepository(db),
		zoneRepo,
		"CO",
	)
	zones := service.NewDeliveryZoneService(zoneRepo)
	zone, err := zones.Create(r.ctx, &dto.CreateDeliveryZoneRequest{Name: "Centro", Boundary: json.RawMessage(centroBoundary)})
	require.NoError(t, err)

	return &deliveryRig{rig: r, orders: orders, zones: zones, openBill: openBills, bus: bus, userID: userID, product: productID, zoneID: zone.ID}
}

func (d *deliveryRig) createOrder(contact dto.OrderContactRequest) (orderID, lineID string) {
	d.t.Helper()
	orderID, lineID = uuid.NewString(), uuid.NewString()
	_, err := d.orders.CreateOrder(d.ctx, &dto.CreateOrderRequest{
		OpenBillID:          orderID,
		TemporalIdentifier:  uuid.NewString(),
		Products:            []dto.OrderProductItem{{OpenBillProductID: lineID, ProductID: d.product, Quantity: 1}},
		OrderContactRequest: contact,
	}, dto.UserDomain{ID: d.userID})
	require.NoError(d.t, err)
	return orderID, lineID
}

func deliveryContact() dto.OrderContactRequest {
	return dto.OrderContactRequest{
		OrderType:       dto.OrderTypeDelivery,
		CustomerName:    strPtr("Ana"),
		CustomerPhone:   strPtr("300 555 1234"),
		DeliveryAddress: strPtr("Cra 7 # 12-34, apto 501"),
		DeliveryLat:     strPtr(insideLat),
		DeliveryLng:     strPtr(insideLng),
	}
}

func strPtr(s string) *string { return &s }

func (d *deliveryRig) pay(orderID string) {
	d.t.Helper()
	require.NoError(d.t, d.orders.PayOrder(d.ctx, command.PayOrderCommand{
		OpenBillID:  orderID,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
	}))
}

func assertDeliveryFulfillment(t *testing.T, f dto.OrderFulfillment, zoneID string) {
	t.Helper()
	assert.Equal(t, dto.OrderTypeDelivery, f.OrderType)
	require.NotNil(t, f.CustomerPhone)
	assert.Equal(t, agentPhone, *f.CustomerPhone)
	require.NotNil(t, f.CustomerName)
	assert.Equal(t, "Ana", *f.CustomerName)
	require.NotNil(t, f.DeliveryAddress)
	require.NotNil(t, f.DeliveryLat)
	assert.True(t, f.DeliveryLat.Equal(decimal.RequireFromString(insideLat)))
	assert.True(t, f.DeliveryLng.Equal(decimal.RequireFromString(insideLng)))
	require.NotNil(t, f.DeliveryZoneID)
	assert.Equal(t, zoneID, *f.DeliveryZoneID)
	require.NotNil(t, f.DeliveryMapURL)
	assert.Contains(t, *f.DeliveryMapURL, "query=4.59,-74.095")
}

func listIDs(list *dto.OpenBillListResponse) []string {
	ids := make([]string, len(list.OpenBills))
	for i, b := range list.OpenBills {
		ids[i] = b.ID
	}
	return ids
}

// 6.1: a delivery order round-trips through every finder, and both lists filter by type and phone.
func TestDeliveryOrders_PersistedAndFiltered(t *testing.T) {
	d := newDeliveryRig(t)
	deliveryID, _ := d.createOrder(deliveryContact())
	dineInID, _ := d.createOrder(dto.OrderContactRequest{})
	otherPhoneID, _ := d.createOrder(dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr("+34 612 345 678")})

	withProducts, err := d.openBill.FindByIDWithProducts(d.ctx, deliveryID)
	require.NoError(t, err)
	assertDeliveryFulfillment(t, withProducts.OrderFulfillment, d.zoneID)
	header, err := d.openBill.FindByID(d.ctx, deliveryID)
	require.NoError(t, err)
	assertDeliveryFulfillment(t, header.OrderFulfillment, d.zoneID)
	aggregate, err := d.openBill.FindAggregateByID(d.ctx, deliveryID)
	require.NoError(t, err)
	assert.True(t, aggregate.IsHeld())
	dineIn, err := d.openBill.FindByIDWithProducts(d.ctx, dineInID)
	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeDineIn, dineIn.OrderType)
	assert.Nil(t, dineIn.DeliveryMapURL)

	all, err := d.orders.GetAllActiveOpenBills(d.ctx, dto.OrderListFilter{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{deliveryID, dineInID, otherPhoneID}, listIDs(all), "no filter: same list as before")

	deliveries, err := d.orders.GetAllActiveOpenBills(d.ctx, dto.OrderListFilter{OrderType: dto.OrderTypeDelivery})
	require.NoError(t, err)
	assert.Equal(t, []string{deliveryID}, listIDs(deliveries))
	assertDeliveryFulfillment(t, deliveries.OpenBills[0].OrderFulfillment, d.zoneID)

	byPhone, err := d.orders.GetAllActiveOpenBills(d.ctx, dto.OrderListFilter{CustomerPhone: "300-555-1234"})
	require.NoError(t, err)
	assert.Equal(t, []string{deliveryID}, listIDs(byPhone), "a differently typed phone still matches")

	d.pay(deliveryID)
	require.NoError(t, d.orders.DeleteOrder(d.ctx, otherPhoneID, dto.UserDomain{}))

	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	closedDeliveries, err := d.orders.GetClosedOpenBills(d.ctx, from, to, dto.OrderListFilter{OrderType: dto.OrderTypeDelivery, CustomerPhone: agentPhone})
	require.NoError(t, err)
	require.Equal(t, []string{deliveryID}, listIDs(closedDeliveries))
	assert.NotNil(t, closedDeliveries.OpenBills[0].PaidAt, "the closed list tells paid from voided")
	voided, err := d.orders.GetClosedOpenBills(d.ctx, from, to, dto.OrderListFilter{CustomerPhone: "+34612345678"})
	require.NoError(t, err)
	require.Equal(t, []string{otherPhoneID}, listIDs(voided))
	assert.Nil(t, voided.OpenBills[0].PaidAt)

	closedDetail, err := d.openBill.FindByIDIncludingDeletedWithProducts(d.ctx, deliveryID)
	require.NoError(t, err)
	assertDeliveryFulfillment(t, closedDetail.OrderFulfillment, d.zoneID)
	assert.NotNil(t, closedDetail.PaidAt)
}

// 6.2: an unpaid delivery is off the board and cannot be started; paying releases it with the
// waiting time measured from the payment, while a dine-in line is unaffected.
func TestDeliveryOrders_HeldUntilPaidThenReleased(t *testing.T) {
	d := newDeliveryRig(t)
	deliveryID, deliveryLine := d.createOrder(deliveryContact())
	_, dineInLine := d.createOrder(dto.OrderContactRequest{})

	board := pendingLineIDs(t, d.rig, d.openBill)
	assert.NotContains(t, board, deliveryLine, "an unpaid delivery is not on the board")
	assert.Contains(t, board, dineInLine, "dine-in shows immediately")

	err := d.orders.SetOpenBillProductInProgress(d.ctx, deliveryID, deliveryLine)
	require.ErrorIs(t, err, openBillError.ErrOrderAwaitingPayment)

	time.Sleep(10 * time.Millisecond)
	d.pay(deliveryID)

	var paidAt time.Time
	require.NoError(t, d.cloudDB.Raw("SELECT paid_at FROM open_bills WHERE id = ?", deliveryID).Scan(&paidAt).Error)
	board = pendingLineIDs(t, d.rig, d.openBill)
	require.Contains(t, board, deliveryLine, "paying releases the line")
	assert.WithinDuration(t, paidAt, board[deliveryLine].CreatedAt, time.Microsecond, "waiting time starts at the payment")
	assert.NotEqual(t, paidAt, board[dineInLine].CreatedAt, "a dine-in line keeps its own creation instant")

	released := d.bus.released()
	require.Len(t, released, 1)
	assert.Equal(t, deliveryID, released[0].OpenBillID)
	assert.WithinDuration(t, paidAt, released[0].PaidAt, time.Microsecond, "a reload agrees with the live event")

	require.NoError(t, d.orders.SetOpenBillProductInProgress(d.ctx, deliveryID, deliveryLine))
	require.NoError(t, d.orders.CompleteOpenBillProduct(d.ctx, deliveryID, deliveryLine))
}

// 6.3: zone CRUD, soft delete, and ListActive ordering.
func TestDeliveryZones_Repository(t *testing.T) {
	r := newRig(t)
	repo := repository.NewDeliveryZoneRepository(r.cloudDB)
	base := time.Now().UTC().Truncate(time.Microsecond)
	zone := func(name string, active bool, at time.Time) *dto.DeliveryZone {
		return &dto.DeliveryZone{ID: uuid.NewString(), Name: name, Boundary: json.RawMessage(centroBoundary), Active: active, CreatedAt: at, UpdatedAt: at}
	}
	newer, older, inactive := zone("Nueva", true, base), zone("Vieja", true, base.Add(-time.Hour)), zone("Apagada", true, base.Add(-2*time.Hour))
	for _, z := range []*dto.DeliveryZone{newer, older, inactive} {
		require.NoError(t, repo.Create(r.ctx, z))
	}
	inactive.Active = false
	inactive.Name = "Apagada (pausada)"
	require.NoError(t, repo.Update(r.ctx, inactive))

	got, err := repo.FindByID(r.ctx, inactive.ID)
	require.NoError(t, err)
	assert.Equal(t, "Apagada (pausada)", got.Name)
	assert.False(t, got.Active)
	assert.JSONEq(t, centroBoundary, string(got.Boundary))

	active, err := repo.ListActive(r.ctx)
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, []string{older.ID, newer.ID}, []string{active[0].ID, active[1].ID}, "oldest first")

	require.NoError(t, repo.Delete(r.ctx, older.ID))
	_, err = repo.FindByID(r.ctx, older.ID)
	require.Error(t, err)
	all, err := repo.FindAll(r.ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2, "soft-deleted zones are gone from the list")
	assert.Equal(t, int64(1), r.cloudCount("delivery_zones", "id = ? AND deleted_at IS NOT NULL", older.ID), "the row is kept")
}

// 6.4: the sync applier needs no change: a pushed order is dine_in, and re-pushing an existing
// delivery keeps its type, contact, location and zone.
func TestDeliveryOrders_SyncApplierLeavesFulfillmentAlone(t *testing.T) {
	d := newDeliveryRig(t)

	pushedID, pushedLine := uuid.NewString(), uuid.NewString()
	d.applyOpenBill(dto.SyncOperationCreate, openBillPayload(pushedID, pushedLine, d.userID, d.product, dto.CommandStatusCreated))
	pushed, err := d.openBill.FindByIDWithProducts(d.ctx, pushedID)
	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeDineIn, pushed.OrderType)
	assert.Nil(t, pushed.CustomerPhone)
	deliveries, err := d.orders.GetAllActiveOpenBills(d.ctx, dto.OrderListFilter{OrderType: dto.OrderTypeDelivery})
	require.NoError(t, err)
	assert.NotContains(t, listIDs(deliveries), pushedID)

	deliveryID, deliveryLine := d.createOrder(deliveryContact())
	d.applyOpenBill(dto.SyncOperationUpdate, openBillPayload(deliveryID, deliveryLine, d.userID, d.product, dto.CommandStatusCreated))
	after, err := d.openBill.FindByIDWithProducts(d.ctx, deliveryID)
	require.NoError(t, err)
	assertDeliveryFulfillment(t, after.OrderFulfillment, d.zoneID)
}

// ---------------------------------------------------------------------------
// HTTP: status codes (6.5) and the agent role (6.6), through the real route wiring.
// ---------------------------------------------------------------------------

const jwtSecret = "acceptance-secret"

type deliveryAPI struct {
	*deliveryRig
	server *httptest.Server
	jwt    *service.JWTService
}

func newDeliveryAPI(t *testing.T) *deliveryAPI {
	d := newDeliveryRig(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	jwt := service.NewJWTService(jwtSecret, "")
	orderHandler := handler.NewOrderHandler(d.orders,
		service.NewOrderIdempotencyService(d.orders, repository.NewIdempotencyKeyRepository(d.cloudDB)))
	handler.RegisterOrderRoutes(router, orderHandler, jwt)
	handler.RegisterDeliveryZoneRoutes(router, &config.Config{AppMode: config.ModeCloud}, handler.NewDeliveryZoneHandler(d.zones), jwt)
	// Mirrors cmd/main.go; the handler is never reached for a role without ProductsCreate.
	router.POST("/api/products", handler.JWTAuthMiddleware(jwt), handler.RequirePermission(permissions.ProductsCreate),
		func(c *gin.Context) { c.Status(http.StatusTeapot) })
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &deliveryAPI{deliveryRig: d, server: server, jwt: jwt}
}

func (a *deliveryAPI) call(roleID int, method, path string, body any) (int, []byte) {
	a.t.Helper()
	token, err := a.jwt.GenerateToken(a.userID, lagunaOrgID, "acceptance", []int{roleID})
	require.NoError(a.t, err)
	var reader io.Reader
	if body != nil {
		raw, marshalErr := json.Marshal(body)
		require.NoError(a.t, marshalErr)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, a.server.URL+path, reader)
	require.NoError(a.t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(a.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(a.t, err)
	return resp.StatusCode, raw
}

func (a *deliveryAPI) orderBody(contact map[string]any) map[string]any {
	body := map[string]any{
		"open_bill_id":        uuid.NewString(),
		"temporal_identifier": uuid.NewString(),
		"products": []map[string]any{
			{"open_bill_product_id": uuid.NewString(), "product_id": a.product, "quantity": 1},
		},
	}
	for k, v := range contact {
		body[k] = v
	}
	return body
}

func TestDeliveryOrders_HTTPStatusCodes(t *testing.T) {
	a := newDeliveryAPI(t)

	tests := []struct {
		name    string
		contact map[string]any
		status  int
	}{
		{"valid delivery", map[string]any{"order_type": "delivery", "customer_phone": "3005551234", "delivery_address": "Calle 1", "delivery_lat": insideLat, "delivery_lng": insideLng}, http.StatusCreated},
		{"unknown type", map[string]any{"order_type": "pickup"}, http.StatusBadRequest},
		{"takeaway without phone", map[string]any{"order_type": "takeaway"}, http.StatusBadRequest},
		{"invalid phone", map[string]any{"order_type": "takeaway", "customer_phone": "12345"}, http.StatusBadRequest},
		{"delivery without location", map[string]any{"order_type": "delivery", "customer_phone": "3005551234", "delivery_address": "Calle 1"}, http.StatusBadRequest},
		{"delivery outside zone", map[string]any{"order_type": "delivery", "customer_phone": "3005551234", "delivery_address": "Calle 1", "delivery_lat": "4.7", "delivery_lng": "-74.2"}, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := a.call(permissions.RoleServer, http.MethodPost, "/api/orders", a.orderBody(tt.contact))
			assert.Equal(t, tt.status, status, string(body))
		})
	}

	status, body := a.call(permissions.RoleServer, http.MethodGet, "/api/orders?order_type=pickup", nil)
	assert.Equal(t, http.StatusBadRequest, status, string(body))
	status, body = a.call(permissions.RoleServer, http.MethodGet, "/api/orders/closed?customer_phone=12345", nil)
	assert.Equal(t, http.StatusBadRequest, status, string(body))

	orderID, lineID := a.createOrder(deliveryContact())
	for _, action := range []string{"in-progress", "complete"} {
		status, body = a.call(permissions.RoleCooker, http.MethodPatch, "/api/orders/"+orderID+"/products/"+lineID+"/"+action, nil)
		assert.Equal(t, http.StatusConflict, status, "%s: %s", action, body)
		assert.Contains(t, string(body), "ORDER_AWAITING_PAYMENT")
	}

	status, body = a.call(permissions.RoleServer, http.MethodGet, "/api/delivery-zones/check?lat=4.59&lng=-74.095", nil)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), `"deliverable":true`)
	status, _ = a.call(permissions.RoleManager, http.MethodPost, "/api/delivery-zones", map[string]any{
		"name": "Rota", "boundary": json.RawMessage(`{"type":"Polygon","coordinates":[[[-74.1,4.58],[-74.06,4.58],[-74.06,4.62],[-74.1,4.62]]]}`),
	})
	assert.Equal(t, http.StatusBadRequest, status, "an open ring is rejected")
}

func TestDeliveryOrders_AgentRole(t *testing.T) {
	a := newDeliveryAPI(t)
	agent := permissions.RoleAgent

	orderID, lineID := uuid.NewString(), uuid.NewString()
	body := a.orderBody(map[string]any{"order_type": "takeaway", "customer_phone": "3005551234"})
	body["open_bill_id"] = orderID
	body["products"] = []map[string]any{{"open_bill_product_id": lineID, "product_id": a.product, "quantity": 1}}
	status, raw := a.call(agent, http.MethodPost, "/api/orders", body)
	require.Equal(t, http.StatusCreated, status, string(raw))

	status, raw = a.call(agent, http.MethodGet, "/api/orders?customer_phone=%2B573005551234", nil)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(raw), orderID)

	status, raw = a.call(agent, http.MethodPut, "/api/orders/"+orderID, map[string]any{
		"products": []map[string]any{{"open_bill_product_id": lineID, "product_id": a.product, "quantity": 2}},
	})
	require.Equal(t, http.StatusOK, status, string(raw))
	assert.Contains(t, string(raw), `"order_type":"takeaway"`, "the edit keeps the type")

	forbidden := []struct{ method, path string }{
		{http.MethodPost, "/api/orders/pay-order"},
		{http.MethodPatch, "/api/orders/" + orderID + "/products/" + lineID + "/complete"},
		{http.MethodPatch, "/api/orders/" + orderID + "/products/" + lineID + "/in-progress"},
		{http.MethodPost, "/api/products"},
		{http.MethodPost, "/api/delivery-zones"},
		{http.MethodGet, "/api/delivery-zones"},
	}
	for _, f := range forbidden {
		status, _ = a.call(agent, f.method, f.path, map[string]any{"order_id": orderID})
		assert.Equal(t, http.StatusForbidden, status, "%s %s", f.method, f.path)
	}
	assert.Equal(t, int64(0), a.cloudCount("open_bills", "id = ? AND paid_at IS NOT NULL", orderID), "nothing was paid")

	status, raw = a.call(agent, http.MethodGet, "/api/delivery-zones/check?lat=4.59&lng=-74.095", nil)
	assert.Equal(t, http.StatusOK, status, "checking a point only needs OrdersCreate: %s", raw)

	status, raw = a.call(agent, http.MethodDelete, "/api/orders/"+orderID, nil)
	require.Equal(t, http.StatusOK, status, string(raw))
	assert.True(t, strings.Contains(string(raw), "deleted"))
}
