package dto

import "time"

// ChannelSettingsInput is the writable settings of a channel; a nil field is "not provided".
type ChannelSettingsInput struct {
	WebhookURL *string `json:"webhook_url,omitempty"`
}

type CreateNotificationChannelRequest struct {
	Name     string                  `json:"name"`
	Type     NotificationChannelType `json:"type"`
	Enabled  *bool                   `json:"enabled,omitempty"`
	Settings ChannelSettingsInput    `json:"settings"`
}

// UpdateNotificationChannelRequest omits Settings to keep the stored secret.
type UpdateNotificationChannelRequest struct {
	Name     *string               `json:"name,omitempty"`
	Enabled  *bool                 `json:"enabled,omitempty"`
	Settings *ChannelSettingsInput `json:"settings,omitempty"`
}

type NotificationChannelSettingsView struct {
	WebhookURLHint string `json:"webhook_url_hint"`
}

// NotificationChannelView is what leaves the service layer: the hint, never the secret.
type NotificationChannelView struct {
	ID        string                          `json:"id"`
	Name      string                          `json:"name"`
	Type      NotificationChannelType         `json:"type"`
	Enabled   bool                            `json:"enabled"`
	Settings  NotificationChannelSettingsView `json:"settings"`
	CreatedAt time.Time                       `json:"created_at"`
	UpdatedAt time.Time                       `json:"updated_at"`
}

type CreateNotificationRuleRequest struct {
	ChannelID           string                 `json:"channel_id"`
	Event               NotificationEventType  `json:"event"`
	Conditions          NotificationConditions `json:"conditions"`
	TitlePrefix         string                 `json:"title_prefix"`
	ExpiresAfterMinutes *int                   `json:"expires_after_minutes,omitempty"`
	Enabled             *bool                  `json:"enabled,omitempty"`
}

type UpdateNotificationRuleRequest struct {
	ChannelID           *string                 `json:"channel_id,omitempty"`
	Conditions          *NotificationConditions `json:"conditions,omitempty"`
	TitlePrefix         *string                 `json:"title_prefix,omitempty"`
	ExpiresAfterMinutes *int                    `json:"expires_after_minutes,omitempty"`
	Enabled             *bool                   `json:"enabled,omitempty"`
}
