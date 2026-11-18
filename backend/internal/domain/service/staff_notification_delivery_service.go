package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

const (
	staffDeliveryBatch   = 50
	staffDeliveryLease   = 2 * time.Minute
	staffFirstRetryDelay = 10 * time.Second
	staffMaxRetryDelay   = 5 * time.Minute
)

// StaffNotificationDeliveryService sends one organization's due notifications and stores what the
// destination answered. Delivery is at least once and in no particular order.
type StaffNotificationDeliveryService struct {
	notifications ports.StaffNotificationRepository
	senders       ports.StaffNotificationSenders
	cipher        ports.SecretCipher
	logger        *slog.Logger
	now           func() time.Time
}

func NewStaffNotificationDeliveryService(
	notifications ports.StaffNotificationRepository,
	senders ports.StaffNotificationSenders,
	cipher ports.SecretCipher,
	logger *slog.Logger,
	now func() time.Time,
) *StaffNotificationDeliveryService {
	if now == nil {
		now = time.Now
	}
	return &StaffNotificationDeliveryService{notifications: notifications, senders: senders, cipher: cipher, logger: logger, now: now}
}

// DeliverDue sends the context's organization's due notifications and returns how many were sent.
func (s *StaffNotificationDeliveryService) DeliverDue(ctx context.Context) (int, error) {
	if _, err := tenant.RequireOrg(ctx); err != nil {
		return 0, err
	}
	now := s.now()
	claimed, err := s.notifications.ClaimDue(ctx, now, now.Add(staffDeliveryLease), staffDeliveryBatch)
	if err != nil {
		return 0, fmt.Errorf("claim due staff notifications: %w", err)
	}
	sent := 0
	for _, c := range claimed {
		ok, err := s.deliver(ctx, c)
		if err != nil {
			return sent, err
		}
		if ok {
			sent++
		}
	}
	return sent, nil
}

func (s *StaffNotificationDeliveryService) deliver(ctx context.Context, c *dto.ClaimedStaffNotification) (bool, error) {
	n := c.Notification
	log := s.logger.With(
		slog.String("organization_id", n.OrganizationID), slog.String("channel_id", n.ChannelID),
		slog.String("rule_id", n.RuleID), slog.String("notification_id", n.ID), slog.Int("attempts", n.Attempts))
	now := s.now()

	if !c.Live {
		return false, s.notifications.MarkFailed(ctx, n.ID, dto.StaffNotificationDropped, now, "rule or channel was deleted or disabled")
	}
	if !now.Before(n.ExpiresAt) {
		log.WarnContext(ctx, "staff notification expired unsent")
		return false, s.notifications.MarkFailed(ctx, n.ID, dto.StaffNotificationExpired, now, "not sent before it expired")
	}

	secret, err := s.openSecret(c)
	if err != nil {
		log.ErrorContext(ctx, "staff notification secret unusable; it will not be retried", slog.Any("error", err))
		return false, s.notifications.MarkFailed(ctx, n.ID, dto.StaffNotificationRejected, now, err.Error())
	}
	sender, ok := s.senders.For(c.ChannelType)
	if !ok {
		log.ErrorContext(ctx, "no sender for channel type; it will not be retried", slog.String("type", string(c.ChannelType)))
		return false, s.notifications.MarkFailed(ctx, n.ID, dto.StaffNotificationRejected, now, "no sender for channel type")
	}

	outcome, sendErr := sender.Send(ctx, secret, n.Message)
	now = s.now()
	if sendErr == nil && outcome.Status == dto.SendSent {
		return true, s.notifications.MarkSent(ctx, n.ID, now)
	}
	if sendErr == nil && outcome.Status == dto.SendRejected {
		log.ErrorContext(ctx, "destination rejected a staff notification; it will not be retried", slog.Int("status", outcome.StatusCode))
		return false, s.notifications.MarkFailed(ctx, n.ID, dto.StaffNotificationRejected, now, fmt.Sprintf("HTTP %d", outcome.StatusCode))
	}

	reason := fmt.Sprintf("HTTP %d", outcome.StatusCode)
	if sendErr != nil {
		reason = sendErr.Error()
	}
	delay := max(staffRetryDelay(n.Attempts), outcome.RetryAfter)
	log.WarnContext(ctx, "staff notification delivery failed; retrying", slog.String("reason", reason))
	return false, s.notifications.MarkFailed(ctx, n.ID, dto.StaffNotificationPending, now.Add(delay), reason)
}

func (s *StaffNotificationDeliveryService) openSecret(c *dto.ClaimedStaffNotification) (dto.ChannelSecret, error) {
	plain, err := s.cipher.Decrypt(c.Secret)
	if err != nil {
		return dto.ChannelSecret{}, fmt.Errorf("decrypt channel secret: %w", err)
	}
	var settings dto.ChannelSettings
	if err := json.Unmarshal(plain, &settings); err != nil {
		return dto.ChannelSecret{}, fmt.Errorf("decode channel secret: %w", err)
	}
	return dto.ChannelSecret{Type: c.ChannelType, Settings: settings}, nil
}

// staffRetryDelay doubles from 10s per attempt made, capped at 5 minutes.
func staffRetryDelay(attempts int) time.Duration {
	delay := staffFirstRetryDelay
	for i := 1; i < attempts && delay < staffMaxRetryDelay; i++ {
		delay *= 2
	}
	return min(delay, staffMaxRetryDelay)
}
