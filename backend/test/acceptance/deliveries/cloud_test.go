package deliveries

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/pkg/eventbus"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A marketing_app signing secret needs 32+ characters; a repeat keeps it plainly a fixture.
var signingSecret = strings.Repeat("s", 43)

const (
	jwtSecret = "accept-secret"
	channel   = "ch_1"
)

type received struct {
	path, signature string
	body            []byte
}

// marketingApp records every POST it receives and answers 2xx.
type marketingApp struct {
	mu     sync.Mutex
	posts  []received
	server *httptest.Server
}

func newMarketingApp(t *testing.T) *marketingApp {
	m := &marketingApp{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.posts = append(m.posts, received{path: r.URL.Path, signature: r.Header.Get("X-Signature-256"), body: body})
		m.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(m.server.Close)
	return m
}

func (m *marketingApp) received() []received {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]received(nil), m.posts...)
}

type cloud struct {
	t      *testing.T
	server *httptest.Server
	app    *marketingApp
	logger *slog.Logger
}

func newCloud(t *testing.T) *cloud {
	t.Helper()
	skipUnlessEnabled(t)
	gin.SetMode(gin.TestMode)
	app := newMarketingApp(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := eventbus.NewGoChannelEventBus(watermill.NopLogger{})
	t.Cleanup(func() { _ = bus.Close() })
	cfg := &config.Cloud{
		Shared: config.Shared{
			AdminAPIKey: "accept-admin-key", JWTSecret: jwtSecret, NodeSyncKey: "accept-node-key", PhoneHomeRegion: "CO",
			Observability: config.Observability{ServiceName: "laguna-backend", MetricsPort: "0"},
		},
		PaymentLinks:    testsupport.PaymentLinks(t),
		Integrations:    testsupport.Integrations(),
		MarketingEvents: config.MarketingEvents{URL: app.server.URL},
	}
	built, err := bootstrap.NewCloud(cfg, bootstrap.Infra{
		DB: cloudDB, Logger: logger, HTTPClient: httpclient.NewClient(logger),
		Storage: mocks.NewMockStorageClient(t), ElectronicInvoice: mocks.NewMockElectronicInvoiceClient(t), EventBus: bus,
	})
	require.NoError(t, err)
	server := httptest.NewServer(built.Router)
	t.Cleanup(server.Close)
	return &cloud{t: t, server: server, app: app, logger: logger}
}

func (c *cloud) token(r *restaurant, userID string, roles ...int) string {
	c.t.Helper()
	token, err := service.NewJWTService(jwtSecret, tenant.AnyOrganization()).GenerateToken(userID, r.orgID, "user-"+userID[:8], roles)
	require.NoError(c.t, err)
	return token
}

func (c *cloud) post(token, path string, body any) (int, []byte) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(c.t, err)
		reader = bytes.NewReader(raw)
	}
	return c.do(http.MethodPost, token, path, reader)
}

func (c *cloud) get(token, path string) (int, []byte) {
	c.t.Helper()
	return c.do(http.MethodGet, token, path, nil)
}

func (c *cloud) do(method, token, path string, body io.Reader) (int, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.server.URL+path, body)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return resp.StatusCode, raw
}

// sendDue runs one pass of the marketing-event delivery job for the restaurant.
func (c *cloud) sendDue(r *restaurant) {
	c.t.Helper()
	integrations := testsupport.IntegrationService(c.t, cloudDB, "http://127.0.0.1/v1")
	delivery := service.NewMarketingEventDeliveryService(repository.NewMarketingEventOutboxRepository(cloudDB),
		httpclient.NewMarketingEventsClient(httpclient.NewClient(c.logger), c.app.server.URL),
		service.MarketingAppSecrets{Integrations: integrations}, c.logger, nil)
	_, err := delivery.DeliverDue(r.ctx)
	require.NoError(c.t, err)
}

func configureMarketingApp(t *testing.T, r *restaurant) {
	t.Helper()
	_, err := testsupport.IntegrationService(t, cloudDB, "http://127.0.0.1/v1").
		SetMarketingApp(context.Background(), r.orgID, dto.MarketingAppCredentials{SigningSecret: signingSecret})
	require.NoError(t, err)
}

func TestCloud_AClaimedAgentDeliveryIsNotifiedOnce(t *testing.T) {
	c := newCloud(t)
	r := seedRestaurant(t)
	configureMarketingApp(t, r)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	completed := []dto.CommandStatus{dto.CommandStatusCompleted}
	agentOrder := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(time.Now()), lineStatus: completed, channelID: strPtr(channel)})
	tillOrder := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(time.Now()), lineStatus: completed})
	courier := c.token(r, juan, permissions.RoleCourier)

	status, body := c.get(courier, "/api/deliveries/mine")
	require.Equal(t, http.StatusOK, status, string(body))
	var queue dto.CourierQueue
	require.NoError(t, json.Unmarshal(body, &queue))
	assert.Len(t, queue.Claimable, 2)

	status, body = c.post(courier, "/api/deliveries/orders/"+agentOrder+"/dispatch", nil)
	require.Equal(t, http.StatusOK, status, string(body))
	var order dto.DeliveryOrder
	require.NoError(t, json.Unmarshal(body, &order))
	assert.Equal(t, dto.DeliveryStatusOutForDelivery, order.DeliveryStatus)
	require.NotNil(t, order.Courier)
	assert.Equal(t, "Juan", order.Courier.Name)

	status, body = c.post(courier, "/api/deliveries/orders/"+agentOrder+"/dispatch", nil)
	assert.Equal(t, http.StatusConflict, status, "a second claim loses")
	assert.Contains(t, string(body), `"code":"INVALID_DELIVERY_TRANSITION"`)
	assert.Contains(t, string(body), `"current_status":"out_for_delivery"`)

	status, body = c.post(courier, "/api/deliveries/orders/"+tillOrder+"/dispatch", nil)
	require.Equal(t, http.StatusOK, status, string(body))

	c.sendDue(r)

	posts := c.app.received()
	require.Len(t, posts, 1, "one notification, none for the till order")
	assert.Equal(t, "/backend/notifications", posts[0].path)
	assert.Equal(t, service.SignMarketingEvent(signingSecret, posts[0].body), posts[0].signature, "signed with the organization's secret")
	var sent struct {
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(posts[0].body, &sent))
	assert.Equal(t, "order.out_for_delivery", sent.Type)
	assert.Equal(t, map[string]string{"order_id": agentOrder, "order_type": "delivery", "courier_name": "Juan"}, sent.Data)
}

func TestCloud_ACourierCannotReadTheDashboard(t *testing.T) {
	c := newCloud(t)
	r := seedRestaurant(t)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	manager := r.seedUser("Gerente", permissions.RoleManager)

	status, _ := c.get(c.token(r, juan, permissions.RoleCourier), "/api/deliveries/orders")
	assert.Equal(t, http.StatusForbidden, status)
	status, body := c.get(c.token(r, manager, permissions.RoleManager), "/api/deliveries/orders?status=out_for_delivery")
	assert.Equal(t, http.StatusOK, status, string(body))
	status, _ = c.get(c.token(r, manager, permissions.RoleManager), "/api/deliveries/orders?status=lost")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestCloud_AFailedDeliveryIsReportedWithoutTheNote(t *testing.T) {
	c := newCloud(t)
	r := seedRestaurant(t)
	configureMarketingApp(t, r)
	juan := r.seedUser("Juan", permissions.RoleCourier)
	pedro := r.seedUser("Pedro", permissions.RoleCourier)
	manager := r.seedUser("Gerente", permissions.RoleManager)
	order := r.seedOrder(seededOrder{orderType: dto.OrderTypeDelivery, paidAt: at(time.Now()),
		lineStatus: []dto.CommandStatus{dto.CommandStatusCompleted}, channelID: strPtr(channel)})
	managerToken := c.token(r, manager, permissions.RoleManager)
	juanToken := c.token(r, juan, permissions.RoleCourier)

	status, body := c.post(managerToken, "/api/deliveries/orders/"+order+"/dispatch", map[string]string{"courier_user_id": juan})
	require.Equal(t, http.StatusOK, status, string(body))
	status, _ = c.post(c.token(r, pedro, permissions.RoleCourier), "/api/deliveries/orders/"+order+"/deliver", nil)
	assert.Equal(t, http.StatusForbidden, status, "another courier's order")
	status, _ = c.post(juanToken, "/api/deliveries/orders/"+order+"/fail", map[string]string{})
	assert.Equal(t, http.StatusBadRequest, status, "a reason is required")
	status, body = c.post(juanToken, "/api/deliveries/orders/"+order+"/fail", map[string]string{"reason": "wrong_address", "note": "la dirección no existe"})
	require.Equal(t, http.StatusOK, status, string(body))

	c.sendDue(r)
	posts := c.app.received()
	require.Len(t, posts, 2)
	assert.Contains(t, string(posts[1].body), `"type":"order.delivery_failed"`)
	assert.Contains(t, string(posts[1].body), `"reason":"wrong_address"`)
	assert.NotContains(t, string(posts[1].body), "no existe", "the note stays with staff")

	status, body = c.get(managerToken, "/api/deliveries/orders/"+order+"/history")
	require.Equal(t, http.StatusOK, status, string(body))
	var history dto.DeliveryHistory
	require.NoError(t, json.Unmarshal(body, &history))
	require.Len(t, history.Entries, 2)
	assert.Equal(t, "la dirección no existe", *history.Entries[1].Note)
}
