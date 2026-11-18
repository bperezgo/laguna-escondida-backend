package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	channel "laguna-escondida/backend/internal/domain/aggregate/notification_channel"
	channelError "laguna-escondida/backend/internal/domain/aggregate/notification_channel/error"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

type NotificationChannelService struct {
	repo    ports.NotificationChannelRepository
	cipher  ports.SecretCipher
	senders ports.StaffNotificationSenders
}

func NewNotificationChannelService(repo ports.NotificationChannelRepository, cipher ports.SecretCipher, senders ports.StaffNotificationSenders) *NotificationChannelService {
	return &NotificationChannelService{repo: repo, cipher: cipher, senders: senders}
}

func (s *NotificationChannelService) List(ctx context.Context) ([]*dto.NotificationChannelView, error) {
	channels, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrNotificationChannelOperation, err)
	}
	views := make([]*dto.NotificationChannelView, 0, len(channels))
	for _, c := range channels {
		views = append(views, channel.NewAggregateFromDTO(c).ToView())
	}
	return views, nil
}

func (s *NotificationChannelService) Get(ctx context.Context, id string) (*dto.NotificationChannelView, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return channel.NewAggregateFromDTO(existing).ToView(), nil
}

func (s *NotificationChannelService) Create(ctx context.Context, req *dto.CreateNotificationChannelRequest) (*dto.NotificationChannelView, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil, err
	}
	agg, err := channel.NewAggregateFromCreateRequest(orgID, req)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNameFree(ctx, agg.Name(), ""); err != nil {
		return nil, err
	}
	if err := s.seal(agg); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, agg.ToDTO()); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrNotificationChannelOperation, err)
	}
	return agg.ToView(), nil
}

func (s *NotificationChannelService) Update(ctx context.Context, id string, req *dto.UpdateNotificationChannelRequest) (*dto.NotificationChannelView, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	agg := channel.NewAggregateFromDTO(existing)
	if err := agg.Update(req); err != nil {
		return nil, err
	}
	if req.Name != nil {
		if err := s.ensureNameFree(ctx, agg.Name(), id); err != nil {
			return nil, err
		}
	}
	if err := s.seal(agg); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, agg.ToDTO()); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrNotificationChannelOperation, err)
	}
	return agg.ToView(), nil
}

func (s *NotificationChannelService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrNotificationChannelOperation, err)
	}
	return nil
}

// Test sends a sample message right away; nothing is stored or retried.
func (s *NotificationChannelService) Test(ctx context.Context, id string) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	sender, ok := s.senders.For(existing.Type)
	if !ok {
		return fmt.Errorf("%w: no sender for type %q", domainError.ErrNotificationChannelOperation, existing.Type)
	}
	plain, err := s.cipher.Decrypt(existing.Secret)
	if err != nil {
		return fmt.Errorf("%w: decrypting secret: %w", domainError.ErrNotificationChannelOperation, err)
	}
	var settings dto.ChannelSettings
	if unmarshalErr := json.Unmarshal(plain, &settings); unmarshalErr != nil {
		return fmt.Errorf("%w: decoding secret: %w", domainError.ErrNotificationChannelOperation, unmarshalErr)
	}
	message := dto.StaffMessage{
		Title:  "Mensaje de prueba",
		Fields: []dto.MessageField{{Name: "Canal", Value: existing.Name, Inline: true}},
		At:     time.Now(),
	}
	outcome, err := sender.Send(ctx, dto.ChannelSecret{Type: existing.Type, Settings: settings}, message)
	if err != nil {
		return &domainError.NotificationChannelUnreachableError{DestinationStatus: outcome.StatusCode}
	}
	if outcome.Status != dto.SendSent {
		return &domainError.NotificationChannelUnreachableError{DestinationStatus: outcome.StatusCode}
	}
	return nil
}

func (s *NotificationChannelService) ensureNameFree(ctx context.Context, name, exceptID string) error {
	channels, err := s.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrNotificationChannelOperation, err)
	}
	for _, c := range channels {
		if c.Name == name && c.ID != exceptID {
			return channelError.NewInvalidFieldError("name", "is already used by another channel")
		}
	}
	return nil
}

// seal encrypts the aggregate's pending settings, if any, into its stored secret.
func (s *NotificationChannelService) seal(agg *channel.Aggregate) error {
	pending := agg.PendingSettings()
	if pending == nil {
		return nil
	}
	plain, err := json.Marshal(pending)
	if err != nil {
		return fmt.Errorf("%w: encoding secret: %w", domainError.ErrNotificationChannelOperation, err)
	}
	encrypted, err := s.cipher.Encrypt(plain)
	if err != nil {
		return fmt.Errorf("%w: encrypting secret: %w", domainError.ErrNotificationChannelOperation, err)
	}
	agg.SetEncryptedSecret(encrypted)
	return nil
}
