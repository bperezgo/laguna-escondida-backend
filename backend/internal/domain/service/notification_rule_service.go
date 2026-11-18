package service

import (
	"context"
	"errors"
	"fmt"

	rule "laguna-escondida/backend/internal/domain/aggregate/notification_rule"
	ruleError "laguna-escondida/backend/internal/domain/aggregate/notification_rule/error"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

type NotificationRuleService struct {
	rules    ports.NotificationRuleRepository
	channels ports.NotificationChannelRepository
}

func NewNotificationRuleService(rules ports.NotificationRuleRepository, channels ports.NotificationChannelRepository) *NotificationRuleService {
	return &NotificationRuleService{rules: rules, channels: channels}
}

// List filters by channelID when it is non-empty.
func (s *NotificationRuleService) List(ctx context.Context, channelID string) ([]*dto.NotificationRule, error) {
	rules, err := s.rules.List(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrNotificationRuleOperation, err)
	}
	return rules, nil
}

func (s *NotificationRuleService) Get(ctx context.Context, id string) (*dto.NotificationRule, error) {
	return s.rules.FindByID(ctx, id)
}

func (s *NotificationRuleService) Create(ctx context.Context, req *dto.CreateNotificationRuleRequest) (*dto.NotificationRule, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil, err
	}
	agg, err := rule.NewAggregateFromCreateRequest(orgID, req)
	if err != nil {
		return nil, err
	}
	if err := s.ensureChannel(ctx, agg.ChannelID()); err != nil {
		return nil, err
	}
	ruleDTO := agg.ToDTO()
	if err := s.rules.Create(ctx, ruleDTO); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrNotificationRuleOperation, err)
	}
	return ruleDTO, nil
}

func (s *NotificationRuleService) Update(ctx context.Context, id string, req *dto.UpdateNotificationRuleRequest) (*dto.NotificationRule, error) {
	existing, err := s.rules.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	agg := rule.NewAggregateFromDTO(existing)
	if err := agg.Update(req); err != nil {
		return nil, err
	}
	if req.ChannelID != nil {
		if err := s.ensureChannel(ctx, agg.ChannelID()); err != nil {
			return nil, err
		}
	}
	ruleDTO := agg.ToDTO()
	if err := s.rules.Update(ctx, ruleDTO); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrNotificationRuleOperation, err)
	}
	return ruleDTO, nil
}

func (s *NotificationRuleService) Delete(ctx context.Context, id string) error {
	if _, err := s.rules.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.rules.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrNotificationRuleOperation, err)
	}
	return nil
}

// ensureChannel is org-scoped by the repository, so another organization's channel reads as missing.
func (s *NotificationRuleService) ensureChannel(ctx context.Context, channelID string) error {
	if _, err := s.channels.FindByID(ctx, channelID); err != nil {
		if errors.Is(err, domainError.ErrNotificationChannelNotFound) {
			return ruleError.NewInvalidFieldError("channel_id", "channel not found")
		}
		return fmt.Errorf("%w: %w", domainError.ErrNotificationRuleOperation, err)
	}
	return nil
}
