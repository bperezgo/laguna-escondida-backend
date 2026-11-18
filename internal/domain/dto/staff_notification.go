package dto

import "time"

type NotificationEventType string

const (
	NotificationEventOrderCreated         NotificationEventType = DomainEventOrderCreated
	NotificationEventOrderPaid            NotificationEventType = DomainEventOrderPaid
	NotificationEventReservationRequested NotificationEventType = DomainEventReservationRequested
	NotificationEventReservationConfirmed NotificationEventType = DomainEventReservationConfirmed
)

type NotificationChannelType string

const (
	NotificationChannelDiscord NotificationChannelType = "discord"
)

// NotificationConditions holds every condition any event supports; a rule may set only its event's.
type NotificationConditions struct {
	OrderTypes   []OrderType         `json:"order_types,omitempty"`
	Sources      []ReservationSource `json:"sources,omitempty"`
	ConfirmedBy  []ConfirmedBy       `json:"confirmed_by,omitempty"`
	MinPartySize *int                `json:"min_party_size,omitempty"`
}

// ChannelSettings holds one typed struct per channel type; only the one matching the type is set.
type ChannelSettings struct {
	Discord *DiscordSettings `json:"discord,omitempty"`
}

type DiscordSettings struct {
	WebhookURL string `json:"webhook_url"`
}

// ChannelSecret is a channel's decrypted settings, used only to send.
type ChannelSecret struct {
	Type     NotificationChannelType
	Settings ChannelSettings
}

type NotificationChannel struct {
	ID             string
	OrganizationID string
	Name           string
	Type           NotificationChannelType
	Enabled        bool
	// Secret is the encrypted settings; it never leaves the service layer.
	Secret     []byte
	SecretHint string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type NotificationRule struct {
	ID                  string
	OrganizationID      string
	ChannelID           string
	Event               NotificationEventType
	Conditions          NotificationConditions
	TitlePrefix         string
	ExpiresAfterMinutes int
	Enabled             bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// NotificationSubject is what a rule's conditions are matched against, built from the decoded event.
type NotificationSubject struct {
	Event       NotificationEventType
	OrderType   *OrderType
	Source      *ReservationSource
	ConfirmedBy *ConfirmedBy
	PartySize   *int
}

type MessageField struct {
	Name   string
	Value  string
	Inline bool
}

// StaffMessage is channel-neutral; Prefix goes first and unchanged so a role mention pings.
type StaffMessage struct {
	Prefix string
	Title  string
	Fields []MessageField
	At     time.Time
}

type SendStatus string

const (
	SendSent     SendStatus = "sent"
	SendRejected SendStatus = "rejected"
	SendRetry    SendStatus = "retry"
)

// SendOutcome is how the destination answered one attempt; StatusCode is 0 when no answer came.
type SendOutcome struct {
	Status     SendStatus
	StatusCode int
	RetryAfter time.Duration
}

type StaffNotificationStatus string

const (
	StaffNotificationPending  StaffNotificationStatus = "pending"
	StaffNotificationSent     StaffNotificationStatus = "sent"
	StaffNotificationRejected StaffNotificationStatus = "rejected"
	StaffNotificationExpired  StaffNotificationStatus = "expired"
	StaffNotificationDropped  StaffNotificationStatus = "dropped"
)

// StaffNotification is one row to send: one rule's message for one event.
type StaffNotification struct {
	ID             string
	OrganizationID string
	RuleID         string
	ChannelID      string
	EventID        string
	Message        StaffMessage
	Status         StaffNotificationStatus
	Attempts       int
	NextAttemptAt  time.Time
	ExpiresAt      time.Time
	LastError      *string
	CreatedAt      time.Time
	SentAt         *time.Time
}

// RuleWithChannel is an enabled rule with its enabled channel, as the consumer needs them.
type RuleWithChannel struct {
	Rule    NotificationRule
	Channel NotificationChannel
}

// ClaimedStaffNotification is a leased notification with what sending needs: whether its rule and
// channel still exist and are enabled, the channel type and its encrypted secret. Attempts includes the claim.
type ClaimedStaffNotification struct {
	Notification *StaffNotification
	Live         bool
	ChannelType  NotificationChannelType
	Secret       []byte
}
