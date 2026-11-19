package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	leadAlertBatch      = 50
	leadAlertLease      = 2 * time.Minute
	leadAlertFirstRetry = 15 * time.Second
	leadAlertMaxRetry   = 30 * time.Minute
	leadAlertGiveUp     = 24 * time.Hour
	leadAlertEmptyField = "—"
)

// LeadAlertService tells the platform operator about new leads, at least once. It has no organization:
// a lead belongs to none, so it can't ride the per-organization notification outbox.
type LeadAlertService struct {
	repo       ports.LeadRepository
	sender     ports.StaffNotificationSender
	webhookURL string
	logger     *slog.Logger
	now        func() time.Time
}

func NewLeadAlertService(repo ports.LeadRepository, sender ports.StaffNotificationSender, webhookURL string, logger *slog.Logger, now func() time.Time) *LeadAlertService {
	if now == nil {
		now = time.Now
	}
	return &LeadAlertService{repo: repo, sender: sender, webhookURL: webhookURL, logger: logger, now: now}
}

// DeliverDue sends the pending alerts that are due and returns how many were sent.
func (s *LeadAlertService) DeliverDue(ctx context.Context) (int, error) {
	now := s.now()
	claimed, err := s.repo.ClaimDueAlerts(ctx, now, now.Add(leadAlertLease), leadAlertBatch)
	if err != nil {
		return 0, fmt.Errorf("claim due lead alerts: %w", err)
	}
	sent := 0
	for _, lead := range claimed {
		ok, err := s.deliver(ctx, lead)
		if err != nil {
			return sent, err
		}
		if ok {
			sent++
		}
	}
	return sent, nil
}

func (s *LeadAlertService) deliver(ctx context.Context, lead *dto.Lead) (bool, error) {
	log := s.logger.With(slog.String("lead_id", lead.ID), slog.Int("attempts", lead.AlertAttempts))
	deadline := lead.CreatedAt.Add(leadAlertGiveUp)
	if !s.now().Before(deadline) {
		return false, s.giveUp(ctx, log, lead, "not sent within 24 hours")
	}

	secret := dto.ChannelSecret{
		Type:     dto.NotificationChannelDiscord,
		Settings: dto.ChannelSettings{Discord: &dto.DiscordSettings{WebhookURL: s.webhookURL}},
	}
	outcome, sendErr := s.sender.Send(ctx, secret, leadMessage(lead))
	now := s.now()
	if sendErr == nil && outcome.Status == dto.SendSent {
		return true, s.repo.MarkAlert(ctx, lead.ID, dto.LeadAlertSent, time.Time{}, "")
	}

	reason := fmt.Sprintf("HTTP %d", outcome.StatusCode)
	if sendErr != nil {
		reason = sendErr.Error()
	}
	next := now.Add(max(leadAlertRetryDelay(lead.AlertAttempts), outcome.RetryAfter))
	if !next.Before(deadline) {
		return false, s.giveUp(ctx, log, lead, reason)
	}
	log.WarnContext(ctx, "lead alert delivery failed; retrying", slog.String("reason", reason), slog.Time("next_attempt_at", next))
	return false, s.repo.MarkAlert(ctx, lead.ID, dto.LeadAlertPending, next, reason)
}

func (s *LeadAlertService) giveUp(ctx context.Context, log *slog.Logger, lead *dto.Lead, reason string) error {
	log.ErrorContext(ctx, "lead alert given up", slog.String("reason", reason))
	return s.repo.MarkAlert(ctx, lead.ID, dto.LeadAlertFailed, time.Time{}, reason)
}

func leadMessage(lead *dto.Lead) dto.StaffMessage {
	return dto.StaffMessage{
		Title: "Nuevo lead",
		Fields: []dto.MessageField{
			{Name: "Nombre", Value: orDash(lead.Name), Inline: true},
			{Name: "Restaurante", Value: orDash(lead.RestaurantName), Inline: true},
			{Name: "Teléfono", Value: orDash(lead.Phone), Inline: true},
			{Name: "utm_source", Value: orDash(lead.UTMSource), Inline: true},
			{Name: "utm_campaign", Value: orDash(lead.UTMCampaign), Inline: true},
		},
		At: lead.CreatedAt,
	}
}

func orDash(value string) string {
	if value == "" {
		return leadAlertEmptyField
	}
	return value
}

// leadAlertRetryDelay doubles from 15s per attempt made, capped at 30 minutes.
func leadAlertRetryDelay(attempts int) time.Duration {
	delay := leadAlertFirstRetry
	for i := 1; i < attempts && delay < leadAlertMaxRetry; i++ {
		delay *= 2
	}
	return min(delay, leadAlertMaxRetry)
}
