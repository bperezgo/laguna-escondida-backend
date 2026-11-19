package config

import (
	"fmt"
	"os"
	"strings"

	"laguna-escondida/backend/internal/domain/aggregate/notification_channel"
)

// LeadAlerts is where a new lead is announced; without a webhook leads are stored and nothing is sent.
type LeadAlerts struct {
	DiscordWebhookURL string
}

func (l LeadAlerts) Enabled() bool {
	return l.DiscordWebhookURL != ""
}

func loadLeadAlerts() (LeadAlerts, error) {
	raw := strings.TrimSpace(os.Getenv("LEAD_ALERT_DISCORD_WEBHOOK_URL"))
	if raw == "" {
		return LeadAlerts{}, nil
	}
	if err := notification_channel.ValidateDiscordWebhookURL(raw); err != nil {
		return LeadAlerts{}, fmt.Errorf("LEAD_ALERT_DISCORD_WEBHOOK_URL must be an https Discord webhook URL: %w", err)
	}
	return LeadAlerts{DiscordWebhookURL: raw}, nil
}
