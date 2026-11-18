package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

type NotificationChannelRepository interface {
	Create(ctx context.Context, channel *dto.NotificationChannel) error
	Update(ctx context.Context, channel *dto.NotificationChannel) error
	FindByID(ctx context.Context, id string) (*dto.NotificationChannel, error)
	List(ctx context.Context) ([]*dto.NotificationChannel, error)
	Delete(ctx context.Context, id string) error
}

type NotificationRuleRepository interface {
	Create(ctx context.Context, rule *dto.NotificationRule) error
	Update(ctx context.Context, rule *dto.NotificationRule) error
	FindByID(ctx context.Context, id string) (*dto.NotificationRule, error)
	// List filters by channelID when it is non-empty.
	List(ctx context.Context, channelID string) ([]*dto.NotificationRule, error)
	Delete(ctx context.Context, id string) error
	// ListEnabledForEvent returns enabled rules of the event whose channel is enabled too.
	ListEnabledForEvent(ctx context.Context, event dto.NotificationEventType) ([]*dto.RuleWithChannel, error)
}

// StaffNotificationRepository is the notification rows, for the context's organization.
type StaffNotificationRepository interface {
	// Insert is idempotent on (rule, event): a repeat records nothing and returns inserted=false.
	Insert(ctx context.Context, notification *dto.StaffNotification) (inserted bool, err error)
	// ClaimDue leases up to limit pending notifications due at now, skipping rows another sender holds,
	// by moving their next attempt to leaseUntil and counting the attempt. Live is false when the rule
	// or channel was deleted or disabled.
	ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]*dto.ClaimedStaffNotification, error)
	MarkSent(ctx context.Context, id string, at time.Time) error
	// MarkFailed stores the outcome of a failed attempt: pending with its next attempt, or a final status.
	MarkFailed(ctx context.Context, id string, status dto.StaffNotificationStatus, nextAttemptAt time.Time, lastError string) error
}

// StaffNotificationSender delivers a message to one kind of chat channel.
type StaffNotificationSender interface {
	Send(ctx context.Context, secret dto.ChannelSecret, message dto.StaffMessage) (dto.SendOutcome, error)
}

// StaffNotificationSenders finds the sender for a channel type.
type StaffNotificationSenders interface {
	For(channelType dto.NotificationChannelType) (StaffNotificationSender, bool)
}

// SecretCipher encrypts channel secrets at rest; Decrypt only to send or test.
type SecretCipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}
