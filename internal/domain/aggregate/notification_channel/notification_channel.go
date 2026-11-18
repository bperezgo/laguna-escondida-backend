package notification_channel

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	channelError "laguna-escondida/backend/internal/domain/aggregate/notification_channel/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

const (
	maxNameLength  = 80
	hintLength     = 4
	webhookPathPfx = "/api/webhooks/"
)

var allowedDiscordHosts = map[string]bool{"discord.com": true, "discordapp.com": true}

type Aggregate struct {
	id         string
	orgID      string
	name       string
	typ        dto.NotificationChannelType
	enabled    bool
	secret     []byte
	secretHint string
	// pending is the plaintext settings awaiting encryption; nil when the stored secret is kept.
	pending   *dto.ChannelSettings
	createdAt time.Time
	updatedAt time.Time
}

func NewAggregateFromCreateRequest(orgID string, req *dto.CreateNotificationChannelRequest) (*Aggregate, error) {
	name, err := validateName(req.Name)
	if err != nil {
		return nil, err
	}
	if req.Type != dto.NotificationChannelDiscord {
		return nil, channelError.NewInvalidFieldError("type", "must be one of: discord")
	}
	settings, hint, err := validateSettings(req.Type, &req.Settings)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	now := time.Now()
	return &Aggregate{
		id:         uuid.Must(uuid.NewV7()).String(),
		orgID:      orgID,
		name:       name,
		typ:        req.Type,
		enabled:    enabled,
		secretHint: hint,
		pending:    settings,
		createdAt:  now,
		updatedAt:  now,
	}, nil
}

func NewAggregateFromDTO(channel *dto.NotificationChannel) *Aggregate {
	return &Aggregate{
		id:         channel.ID,
		orgID:      channel.OrganizationID,
		name:       channel.Name,
		typ:        channel.Type,
		enabled:    channel.Enabled,
		secret:     channel.Secret,
		secretHint: channel.SecretHint,
		createdAt:  channel.CreatedAt,
		updatedAt:  channel.UpdatedAt,
	}
}

// Update keeps the stored secret unless the request carries settings.
func (a *Aggregate) Update(req *dto.UpdateNotificationChannelRequest) error {
	if req.Name != nil {
		name, err := validateName(*req.Name)
		if err != nil {
			return err
		}
		a.name = name
	}
	if req.Settings != nil && req.Settings.WebhookURL != nil {
		settings, hint, err := validateSettings(a.typ, req.Settings)
		if err != nil {
			return err
		}
		a.pending, a.secretHint = settings, hint
	}
	if req.Enabled != nil {
		a.enabled = *req.Enabled
	}
	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) Name() string { return a.name }

func (a *Aggregate) Type() dto.NotificationChannelType { return a.typ }

// PendingSettings is the plaintext to encrypt before saving, or nil when the secret is unchanged.
func (a *Aggregate) PendingSettings() *dto.ChannelSettings { return a.pending }

func (a *Aggregate) SetEncryptedSecret(secret []byte) {
	a.secret = secret
	a.pending = nil
}

func (a *Aggregate) ToDTO() *dto.NotificationChannel {
	return &dto.NotificationChannel{
		ID:             a.id,
		OrganizationID: a.orgID,
		Name:           a.name,
		Type:           a.typ,
		Enabled:        a.enabled,
		Secret:         a.secret,
		SecretHint:     a.secretHint,
		CreatedAt:      a.createdAt,
		UpdatedAt:      a.updatedAt,
	}
}

func (a *Aggregate) ToView() *dto.NotificationChannelView {
	return &dto.NotificationChannelView{
		ID:        a.id,
		Name:      a.name,
		Type:      a.typ,
		Enabled:   a.enabled,
		Settings:  dto.NotificationChannelSettingsView{WebhookURLHint: a.secretHint},
		CreatedAt: a.createdAt,
		UpdatedAt: a.updatedAt,
	}
}

// ValidateDiscordWebhookURL is also used by the sender, so a stored URL is re-checked before every send.
func ValidateDiscordWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !allowedDiscordHosts[strings.ToLower(u.Hostname())] ||
		u.Port() != "" || u.User != nil || !strings.HasPrefix(u.Path, webhookPathPfx) || len(u.Path) == len(webhookPathPfx) {
		return channelError.NewInvalidFieldError("settings.webhook_url", "must be an https Discord webhook URL (discord.com or discordapp.com, /api/webhooks/...)")
	}
	return nil
}

func validateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return "", channelError.NewInvalidFieldError("name", "is required and must be at most 80 characters")
	}
	return name, nil
}

func validateSettings(typ dto.NotificationChannelType, in *dto.ChannelSettingsInput) (*dto.ChannelSettings, string, error) {
	switch typ {
	case dto.NotificationChannelDiscord:
		if in.WebhookURL == nil {
			return nil, "", channelError.NewInvalidFieldError("settings.webhook_url", "is required")
		}
		if err := ValidateDiscordWebhookURL(*in.WebhookURL); err != nil {
			return nil, "", err
		}
		return &dto.ChannelSettings{Discord: &dto.DiscordSettings{WebhookURL: *in.WebhookURL}}, hint(*in.WebhookURL), nil
	default:
		return nil, "", channelError.NewInvalidFieldError("type", "must be one of: discord")
	}
}

func hint(secret string) string {
	runes := []rune(secret)
	if len(runes) > hintLength {
		runes = runes[len(runes)-hintLength:]
	}
	return "…" + string(runes)
}
