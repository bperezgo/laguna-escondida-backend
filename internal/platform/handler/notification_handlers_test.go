package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	notifTestOrg     = "org-a"
	notifChannelID   = "019a0000-0000-7000-8000-0000000000c1"
	notifRuleID      = "019a0000-0000-7000-8000-0000000000d1"
	notifValidHook   = "https://discord.com/api/webhooks/123/abcd1234"
	notifChannelsURL = "/api/notification-channels"
	notifRulesURL    = "/api/notification-rules"
)

type notificationRouter struct {
	router   *gin.Engine
	channels *mocks.MockNotificationChannelRepository
	rules    *mocks.MockNotificationRuleRepository
	cipher   *mocks.MockSecretCipher
	senders  *mocks.MockStaffNotificationSenders
	sender   *mocks.MockStaffNotificationSender
}

func newNotificationRouter(t *testing.T) *notificationRouter {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &notificationRouter{
		channels: mocks.NewMockNotificationChannelRepository(t),
		rules:    mocks.NewMockNotificationRuleRepository(t),
		cipher:   mocks.NewMockSecretCipher(t),
		senders:  mocks.NewMockStaffNotificationSenders(t),
		sender:   mocks.NewMockStaffNotificationSender(t),
	}
	r.router = gin.New()
	r.router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), notifTestOrg))
	})
	registerNotificationTestRoutes(r.router,
		NewNotificationChannelHandler(service.NewNotificationChannelService(r.channels, r.cipher, r.senders)),
		NewNotificationRuleHandler(service.NewNotificationRuleService(r.rules, r.channels)))
	return r
}

func registerNotificationTestRoutes(router *gin.Engine, ch *NotificationChannelHandler, ru *NotificationRuleHandler) {
	router.GET(notifChannelsURL, ch.ListHandler)
	router.POST(notifChannelsURL, ch.CreateHandler)
	router.GET(notifChannelsURL+"/:id", ch.GetHandler)
	router.PUT(notifChannelsURL+"/:id", ch.UpdateHandler)
	router.DELETE(notifChannelsURL+"/:id", ch.DeleteHandler)
	router.POST(notifChannelsURL+"/:id/test", ch.TestHandler)
	router.GET(notifRulesURL, ru.ListHandler)
	router.POST(notifRulesURL, ru.CreateHandler)
	router.GET(notifRulesURL+"/:id", ru.GetHandler)
	router.PUT(notifRulesURL+"/:id", ru.UpdateHandler)
	router.DELETE(notifRulesURL+"/:id", ru.DeleteHandler)
}

func doNotificationRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func storedNotificationChannel() *dto.NotificationChannel {
	return &dto.NotificationChannel{
		ID: notifChannelID, OrganizationID: notifTestOrg, Name: "Domicilios", Type: dto.NotificationChannelDiscord,
		Enabled: true, Secret: []byte("sealed"), SecretHint: "…1234",
	}
}

func TestNotificationChannelHandler_Get_ShowsHintAndNoWebhookURL(t *testing.T) {
	r := newNotificationRouter(t)
	r.channels.EXPECT().FindByID(mock.Anything, notifChannelID).Return(storedNotificationChannel(), nil)

	rec := doNotificationRequest(r.router, http.MethodGet, notifChannelsURL+"/"+notifChannelID, "")

	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON(t, rec)
	settings, ok := body["settings"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "…1234", settings["webhook_url_hint"])
	assert.NotContains(t, rec.Body.String(), "webhook_url\"")
	assert.NotContains(t, rec.Body.String(), "sealed")
}

func TestNotificationChannelHandler_Create_InvalidWebhookNamesTheField(t *testing.T) {
	r := newNotificationRouter(t)

	rec := doNotificationRequest(r.router, http.MethodPost, notifChannelsURL,
		`{"name":"Domicilios","type":"discord","settings":{"webhook_url":"https://example.com/hook"}}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeJSON(t, rec)
	assert.Equal(t, "settings.webhook_url", body["field"])
	assert.Equal(t, "NOTIFICATION_CHANNEL_INVALID_FIELD", body["code"])
}

func TestNotificationChannelHandler_Create_BadBodyIs400(t *testing.T) {
	r := newNotificationRouter(t)
	rec := doNotificationRequest(r.router, http.MethodPost, notifChannelsURL, `{`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationChannelHandler_Create_Returns201(t *testing.T) {
	r := newNotificationRouter(t)
	r.channels.EXPECT().List(mock.Anything).Return(nil, nil)
	r.cipher.EXPECT().Encrypt(mock.Anything).Return([]byte("sealed"), nil)
	r.channels.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	rec := doNotificationRequest(r.router, http.MethodPost, notifChannelsURL,
		`{"name":"Domicilios","type":"discord","settings":{"webhook_url":"`+notifValidHook+`"}}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, true, decodeJSON(t, rec)["enabled"])
}

func TestNotificationChannelHandler_NotFoundIs404(t *testing.T) {
	r := newNotificationRouter(t)
	r.channels.EXPECT().FindByID(mock.Anything, notifChannelID).Return(nil, domainError.ErrNotificationChannelNotFound)

	rec := doNotificationRequest(r.router, http.MethodDelete, notifChannelsURL+"/"+notifChannelID, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotificationChannelHandler_Test(t *testing.T) {
	tests := []struct {
		name       string
		outcome    dto.SendOutcome
		wantStatus int
		wantDest   float64
	}{
		{"accepted", dto.SendOutcome{Status: dto.SendSent, StatusCode: 204}, http.StatusNoContent, 0},
		{"refused", dto.SendOutcome{Status: dto.SendRejected, StatusCode: 404}, http.StatusBadGateway, 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNotificationRouter(t)
			r.channels.EXPECT().FindByID(mock.Anything, notifChannelID).Return(storedNotificationChannel(), nil)
			r.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(r.sender, true)
			r.cipher.EXPECT().Decrypt([]byte("sealed")).Return([]byte(`{"discord":{"webhook_url":"`+notifValidHook+`"}}`), nil)
			r.sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything).Return(tt.outcome, nil)

			rec := doNotificationRequest(r.router, http.MethodPost, notifChannelsURL+"/"+notifChannelID+"/test", "")

			require.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusBadGateway {
				body := decodeJSON(t, rec)
				assert.Equal(t, "NOTIFICATION_CHANNEL_UNREACHABLE", body["code"])
				assert.Equal(t, tt.wantDest, body["destination_status"])
			}
		})
	}
}

func TestNotificationRuleHandler_Create_ReturnsSnakeCaseJSON(t *testing.T) {
	r := newNotificationRouter(t)
	r.channels.EXPECT().FindByID(mock.Anything, notifChannelID).Return(storedNotificationChannel(), nil)
	r.rules.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	rec := doNotificationRequest(r.router, http.MethodPost, notifRulesURL,
		`{"channel_id":"`+notifChannelID+`","event":"order.paid","conditions":{"order_types":["delivery"]}}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	body := decodeJSON(t, rec)
	assert.Equal(t, notifChannelID, body["channel_id"])
	assert.Equal(t, "order.paid", body["event"])
	assert.Equal(t, float64(60), body["expires_after_minutes"])
	assert.Equal(t, true, body["enabled"])
	assert.Equal(t, map[string]any{"order_types": []any{"delivery"}}, body["conditions"])
	assert.NotContains(t, body, "ChannelID")
}

func TestNotificationRuleHandler_Create_InvalidConditionNamesTheField(t *testing.T) {
	r := newNotificationRouter(t)

	rec := doNotificationRequest(r.router, http.MethodPost, notifRulesURL,
		`{"channel_id":"`+notifChannelID+`","event":"order.paid","conditions":{"min_party_size":6}}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeJSON(t, rec)
	assert.Equal(t, "conditions.min_party_size", body["field"])
	assert.Equal(t, "NOTIFICATION_RULE_INVALID_FIELD", body["code"])
}

func TestNotificationRuleHandler_List_FiltersByChannelAndMapsJSON(t *testing.T) {
	r := newNotificationRouter(t)
	r.rules.EXPECT().List(mock.Anything, notifChannelID).Return([]*dto.NotificationRule{{
		ID: notifRuleID, ChannelID: notifChannelID, Event: dto.NotificationEventOrderPaid, Enabled: true,
		ExpiresAfterMinutes: 60, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}, nil)

	rec := doNotificationRequest(r.router, http.MethodGet, notifRulesURL+"?channel_id="+notifChannelID, "")

	require.Equal(t, http.StatusOK, rec.Code)
	rules, ok := decodeJSON(t, rec)["notification_rules"].([]any)
	require.True(t, ok)
	require.Len(t, rules, 1)
	first, ok := rules[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, notifRuleID, first["id"])
}

func TestNotificationRuleHandler_NotFoundIs404(t *testing.T) {
	r := newNotificationRouter(t)
	r.rules.EXPECT().FindByID(mock.Anything, notifRuleID).Return(nil, domainError.ErrNotificationRuleNotFound)

	rec := doNotificationRequest(r.router, http.MethodGet, notifRulesURL+"/"+notifRuleID, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotificationHandlers_WithoutKeyRespond503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerNotificationTestRoutes(router, NewNotificationChannelHandler(nil), NewNotificationRuleHandler(nil))

	for _, path := range []string{notifChannelsURL, notifRulesURL} {
		rec := doNotificationRequest(router, http.MethodGet, path, "")
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, path)
		assert.Equal(t, "NOTIFICATIONS_DISABLED", decodeJSON(t, rec)["code"])
	}
	rec := doNotificationRequest(router, http.MethodPost, notifChannelsURL+"/"+notifChannelID+"/test", "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
