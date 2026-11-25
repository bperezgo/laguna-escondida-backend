package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	paymentStrategyField     = "strategy"
	paymentInstructionsField = "manual_instructions"
	paymentWindowField       = "manual_window_minutes"
)

// PaymentSettingsService reads and saves how the context organization's agent orders are paid.
type PaymentSettingsService struct {
	settings ports.PaymentSettingsRepository
}

func NewPaymentSettingsService(settings ports.PaymentSettingsRepository) *PaymentSettingsService {
	return &PaymentSettingsService{settings: settings}
}

// Get returns the saved settings, or wompi with the default window when none were saved.
func (s *PaymentSettingsService) Get(ctx context.Context) (*dto.PaymentSettings, error) {
	stored, found, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("read payment settings: %w", err)
	}
	if !found {
		return &dto.PaymentSettings{Strategy: dto.PaymentStrategyWompi, ManualWindowMinutes: dto.DefaultManualWindowMinutes}, nil
	}
	return stored, nil
}

// Save validates and replaces the three settings.
func (s *PaymentSettingsService) Save(ctx context.Context, req dto.PaymentSettingsRequest) (*dto.PaymentSettings, error) {
	settings, err := validatePaymentSettings(req)
	if err != nil {
		return nil, err
	}
	if err = s.settings.Upsert(ctx, settings); err != nil {
		return nil, fmt.Errorf("save payment settings: %w", err)
	}
	return settings, nil
}

func validatePaymentSettings(req dto.PaymentSettingsRequest) (*dto.PaymentSettings, error) {
	strategy := dto.PaymentStrategy(req.Strategy)
	if strategy != dto.PaymentStrategyManual && strategy != dto.PaymentStrategyWompi {
		return nil, &domainError.IntegrationValidationError{Field: paymentStrategyField, Reason: "must be manual or wompi"}
	}
	instructions := strings.TrimSpace(req.ManualInstructions)
	if strategy == dto.PaymentStrategyManual && instructions == "" {
		return nil, &domainError.IntegrationValidationError{Field: paymentInstructionsField, Reason: "is required for the manual strategy"}
	}
	if utf8.RuneCountInString(instructions) > dto.MaxManualInstructionsChars {
		return nil, &domainError.IntegrationValidationError{
			Field: paymentInstructionsField, Reason: fmt.Sprintf("must be at most %d characters", dto.MaxManualInstructionsChars),
		}
	}
	window := dto.DefaultManualWindowMinutes
	if req.ManualWindowMinutes != nil {
		window = *req.ManualWindowMinutes
	}
	if window < dto.MinManualWindowMinutes || window > dto.MaxManualWindowMinutes {
		return nil, &domainError.IntegrationValidationError{
			Field: paymentWindowField, Reason: fmt.Sprintf("must be a whole number from %d to %d", dto.MinManualWindowMinutes, dto.MaxManualWindowMinutes),
		}
	}
	return &dto.PaymentSettings{Strategy: strategy, ManualInstructions: instructions, ManualWindowMinutes: window}, nil
}
