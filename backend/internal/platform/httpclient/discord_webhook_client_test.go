package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discordTestSender(t *testing.T, handler http.HandlerFunc) (*DiscordWebhookClient, dto.ChannelSecret) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	sender := &DiscordWebhookClient{client: NewClient(slog.New(slog.NewTextHandler(io.Discard, nil)))}
	sender.validateURL = func(string) error { return nil }
	return sender, dto.ChannelSecret{
		Type:     dto.NotificationChannelDiscord,
		Settings: dto.ChannelSettings{Discord: &dto.DiscordSettings{WebhookURL: server.URL + "/api/webhooks/1/abc"}},
	}
}

func TestDiscordWebhookClient_Send_Body(t *testing.T) {
	var got map[string]any
	sender, secret := discordTestSender(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusNoContent)
	})

	outcome, err := sender.Send(context.Background(), secret, dto.StaffMessage{
		Prefix: "<@&123> Domicilio",
		Title:  "Pedido pagado",
		Fields: []dto.MessageField{{Name: "Total", Value: "$10.000", Inline: true}},
		At:     time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	assert.Equal(t, dto.SendOutcome{Status: dto.SendSent, StatusCode: http.StatusNoContent}, outcome)
	assert.Equal(t, "<@&123> Domicilio", got["content"], "the prefix is in content so the mention pings")
	assert.Equal(t, map[string]any{"parse": []any{"roles", "users"}}, got["allowed_mentions"])
	embeds, ok := got["embeds"].([]any)
	require.True(t, ok)
	require.Len(t, embeds, 1)
	embed, ok := embeds[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Pedido pagado", embed["title"])
	assert.Equal(t, "2026-11-18T15:00:00Z", embed["timestamp"])
	assert.Equal(t, []any{map[string]any{"name": "Total", "value": "$10.000", "inline": true}}, embed["fields"])
}

func TestDiscordWebhookClient_Send_Outcomes(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		retryAfter string
		want       dto.SendOutcome
	}{
		{"no content is sent", http.StatusNoContent, "", dto.SendOutcome{Status: dto.SendSent, StatusCode: 204}},
		{"not found is rejected", http.StatusNotFound, "", dto.SendOutcome{Status: dto.SendRejected, StatusCode: 404}},
		{"unauthorized is rejected", http.StatusUnauthorized, "", dto.SendOutcome{Status: dto.SendRejected, StatusCode: 401}},
		{"rate limit carries retry after", http.StatusTooManyRequests, "1.5", dto.SendOutcome{Status: dto.SendRetry, StatusCode: 429, RetryAfter: 1500 * time.Millisecond}},
		{"rate limit with garbage header", http.StatusTooManyRequests, "soon", dto.SendOutcome{Status: dto.SendRetry, StatusCode: 429}},
		{"server error retries", http.StatusBadGateway, "", dto.SendOutcome{Status: dto.SendRetry, StatusCode: 502}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, secret := discordTestSender(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				w.WriteHeader(tt.status)
			})
			outcome, err := sender.Send(context.Background(), secret, dto.StaffMessage{Title: "t"})
			require.NoError(t, err)
			assert.Equal(t, tt.want, outcome)
		})
	}
}

func TestDiscordWebhookClient_Send_RechecksTheAllowList(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(server.Close)
	sender := NewDiscordWebhookClient(NewClient(slog.New(slog.NewTextHandler(io.Discard, nil))))

	outcome, err := sender.Send(context.Background(), dto.ChannelSecret{
		Settings: dto.ChannelSettings{Discord: &dto.DiscordSettings{WebhookURL: server.URL + "/api/webhooks/1/abc"}},
	}, dto.StaffMessage{Title: "t"})

	require.Error(t, err)
	assert.Equal(t, dto.SendRejected, outcome.Status)
	assert.False(t, called, "a URL outside Discord is never called")
}

func TestDiscordWebhookClient_Send_NetworkErrorRetries(t *testing.T) {
	sender, secret := discordTestSender(t, func(http.ResponseWriter, *http.Request) {})
	secret.Settings.Discord.WebhookURL = "http://127.0.0.1:1/api/webhooks/1/abc"

	outcome, err := sender.Send(context.Background(), secret, dto.StaffMessage{Title: "t"})

	require.Error(t, err)
	assert.Equal(t, dto.SendRetry, outcome.Status)
}

func TestStaffNotificationSenderRegistry_For(t *testing.T) {
	sender := NewDiscordWebhookClient(NewClient(slog.New(slog.NewTextHandler(io.Discard, nil))))
	registry := NewStaffNotificationSenderRegistry(map[dto.NotificationChannelType]ports.StaffNotificationSender{dto.NotificationChannelDiscord: sender})

	found, ok := registry.For(dto.NotificationChannelDiscord)
	assert.True(t, ok)
	assert.Equal(t, sender, found)
	_, ok = registry.For("sms")
	assert.False(t, ok)
}
