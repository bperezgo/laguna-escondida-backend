package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/command"
	"laguna-escondida/backend/internal/domain/aggregate/command_item"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const CommandCreatedEventType = "command.created"

type CommandService struct {
	logger      *zap.Logger
	commandRepo ports.CommandRepository
	userRepo    ports.UserRepository
	sseNotifier ports.SSENotifier
}

func NewCommandService(
	logger *zap.Logger,
	commandRepo ports.CommandRepository,
	userRepo ports.UserRepository,
	sseNotifier ports.SSENotifier,
) *CommandService {
	return &CommandService{
		logger:      logger,
		commandRepo: commandRepo,
		userRepo:    userRepo,
		sseNotifier: sseNotifier,
	}
}

func (s *CommandService) HandleOrderCreated(ctx context.Context, event dto.OrderCreatedEvent) error {
	if len(event.Products) == 0 {
		return nil
	}

	productIDs := make([]string, len(event.Products))
	for i, p := range event.Products {
		productIDs[i] = p.ProductID
	}

	responsibilities, err := s.commandRepo.GetProductPreparationResponsibilities(ctx, productIDs)
	if err != nil {
		return fmt.Errorf("failed to get product preparation responsibilities: %w", err)
	}

	if len(responsibilities) == 0 {
		return nil
	}

	var createdBy *dto.OpenBillCreator
	if event.CreatedByID != "" {
		user, err := s.userRepo.FindByID(ctx, event.CreatedByID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}
		createdBy = &dto.OpenBillCreator{
			ID:       user.ID,
			Username: user.Username,
		}
	}

	responsibilityMap := make(map[string]*dto.ProductPreparationResponsibilityWithProduct)
	for i := range responsibilities {
		responsibilityMap[responsibilities[i].ProductID] = &responsibilities[i]
	}

	areaItemsMap := make(map[string][]*command_item.Aggregate)
	for _, p := range event.Products {
		responsibility, exists := responsibilityMap[p.ProductID]
		// This will remove all the products without an assigned area, we will just create the command for the products with an assigned area
		if !exists {
			continue
		}

		item, err := command_item.NewCommandItem(
			uuid.New().String(),
			p.OpenBillProductID,
			p.ProductID,
			responsibility.ProductName,
			p.Quantity,
			p.Notes,
			responsibility.Priority,
		)
		if err != nil {
			s.logger.Error("failed to create command item", zap.Error(err))
			continue
		}

		areaItemsMap[responsibility.Area] = append(areaItemsMap[responsibility.Area], item)
	}

	now := time.Now()
	for area, items := range areaItemsMap {
		cmd, err := command.NewCommand(
			uuid.New().String(),
			event.OpenBillID,
			event.TemporalIdentifier,
			createdBy,
			area,
			items,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to create command for area %s: %w", area, err)
		}

		if err := s.commandRepo.Create(ctx, cmd); err != nil {
			return fmt.Errorf("failed to create command for area %s: %w", area, err)
		}

		if err := s.sseNotifier.NotifyArea(ctx, area, CommandCreatedEventType, cmd.ToDTO()); err != nil {
			return fmt.Errorf("failed to notify area %s: %w", area, err)
		}
	}

	return nil
}

func (s *CommandService) GetPendingCommandsByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	return s.commandRepo.FindPendingByArea(ctx, area)
}

func (s *CommandService) CompleteCommand(ctx context.Context, id string) (*dto.Command, error) {
	cmd, err := s.commandRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := cmd.CompleteAllItems(); err != nil {
		return nil, err
	}

	if err := s.commandRepo.Update(ctx, cmd); err != nil {
		return nil, err
	}

	return cmd.ToDTO(), nil
}

func (s *CommandService) CompleteCommandItem(ctx context.Context, itemID string) (*dto.Command, error) {
	cmd, err := s.commandRepo.FindByItemID(ctx, itemID)
	if err != nil {
		return nil, err
	}

	if err := cmd.CompleteItem(itemID); err != nil {
		return nil, err
	}

	if _, err := cmd.TryComplete(); err != nil {
		return nil, err
	}

	if err := s.commandRepo.Update(ctx, cmd); err != nil {
		return nil, err
	}

	return cmd.ToDTO(), nil
}
