package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/notification_channel"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	discordTimeout   = 10 * time.Second
	discordEmbedBlue = 0x3498db
)

type DiscordWebhookClient struct {
	client      *Client
	validateURL func(string) error
}

func NewDiscordWebhookClient(client *Client) ports.StaffNotificationSender {
	return &DiscordWebhookClient{client: client, validateURL: notification_channel.ValidateDiscordWebhookURL}
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordEmbed struct {
	Title     string         `json:"title"`
	Color     int            `json:"color"`
	Fields    []discordField `json:"fields,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
}

type discordAllowedMentions struct {
	Parse []string `json:"parse"`
}

type discordMessage struct {
	Content         string                 `json:"content,omitempty"`
	Embeds          []discordEmbed         `json:"embeds"`
	AllowedMentions discordAllowedMentions `json:"allowed_mentions"`
}

// Send answers retry for anything that is not a definitive outcome, so the delivery service backs off.
func (c *DiscordWebhookClient) Send(ctx context.Context, secret dto.ChannelSecret, message dto.StaffMessage) (dto.SendOutcome, error) {
	if secret.Settings.Discord == nil {
		return dto.SendOutcome{Status: dto.SendRejected}, fmt.Errorf("channel has no discord settings")
	}
	webhookURL := secret.Settings.Discord.WebhookURL
	// The stored URL is re-checked on every send, so a tampered row can't turn this into an SSRF.
	if err := c.validateURL(webhookURL); err != nil {
		return dto.SendOutcome{Status: dto.SendRejected}, fmt.Errorf("stored webhook url: %w", err)
	}
	body, err := json.Marshal(buildDiscordMessage(message))
	if err != nil {
		return dto.SendOutcome{Status: dto.SendRetry}, fmt.Errorf("marshal discord message: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, discordTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return dto.SendOutcome{Status: dto.SendRetry}, fmt.Errorf("build discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return dto.SendOutcome{Status: dto.SendRetry}, fmt.Errorf("send discord message: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return discordOutcome(resp), nil
}

func buildDiscordMessage(m dto.StaffMessage) discordMessage {
	embed := discordEmbed{Title: m.Title, Color: discordEmbedBlue}
	for _, f := range m.Fields {
		embed.Fields = append(embed.Fields, discordField{Name: f.Name, Value: f.Value, Inline: f.Inline})
	}
	if !m.At.IsZero() {
		embed.Timestamp = m.At.UTC().Format(time.RFC3339)
	}
	return discordMessage{
		Content:         m.Prefix,
		Embeds:          []discordEmbed{embed},
		AllowedMentions: discordAllowedMentions{Parse: []string{"roles", "users"}},
	}
}

func discordOutcome(resp *http.Response) dto.SendOutcome {
	outcome := dto.SendOutcome{StatusCode: resp.StatusCode}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		outcome.Status = dto.SendSent
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound:
		outcome.Status = dto.SendRejected
	case resp.StatusCode == http.StatusTooManyRequests:
		outcome.Status = dto.SendRetry
		outcome.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	default:
		outcome.Status = dto.SendRetry
	}
	return outcome
}

// Discord sends fractional seconds, which the HTTP date form of Retry-After never does.
func parseRetryAfter(raw string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// StaffNotificationSenderRegistry finds the sender for a channel type.
type StaffNotificationSenderRegistry struct {
	senders map[dto.NotificationChannelType]ports.StaffNotificationSender
}

func NewStaffNotificationSenderRegistry(senders map[dto.NotificationChannelType]ports.StaffNotificationSender) ports.StaffNotificationSenders {
	return &StaffNotificationSenderRegistry{senders: senders}
}

func (r *StaffNotificationSenderRegistry) For(channelType dto.NotificationChannelType) (ports.StaffNotificationSender, bool) {
	sender, ok := r.senders[channelType]
	return sender, ok
}
