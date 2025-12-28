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
const CommandCancelledEventType = "command.cancelled"

const CommandItemCreatedEventType = "command_item.created"
const CommandItemCancelledEventType = "command_item.cancelled"
const CommandItemUpdatedEventType = "command_item.updated"

type CommandService struct {
	logger                 *zap.Logger
	commandRepo            ports.CommandRepository
	userRepo               ports.UserRepository
	sseNotifier            ports.SSENotifier
	commandItemSSENotifier ports.CommandItemSSENotifier
}

func NewCommandService(
	logger *zap.Logger,
	commandRepo ports.CommandRepository,
	userRepo ports.UserRepository,
	sseNotifier ports.SSENotifier,
	commandItemSSENotifier ports.CommandItemSSENotifier,
) *CommandService {
	return &CommandService{
		logger:                 logger,
		commandRepo:            commandRepo,
		userRepo:               userRepo,
		sseNotifier:            sseNotifier,
		commandItemSSENotifier: commandItemSSENotifier,
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

	userName := ""
	if createdBy != nil {
		userName = createdBy.Username
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

		for _, item := range items {
			itemSSE := &dto.CommandItemSSE{
				OpenBillProductID:  item.OpenBillProductID(),
				OpenBillID:         event.OpenBillID,
				ProductName:        item.ProductName(),
				Quantity:           item.Quantity(),
				Notes:              item.Notes(),
				TemporalIdentifier: event.TemporalIdentifier,
				Priority:           item.Priority(),
				CreatedAt:          now,
				Name:               userName,
			}
			if err := s.commandItemSSENotifier.NotifyArea(ctx, area, CommandItemCreatedEventType, itemSSE); err != nil {
				s.logger.Error("failed to notify command item created",
					zap.String("area", area),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

func (s *CommandService) GetPendingCommandsByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	return s.commandRepo.FindPendingByArea(ctx, area)
}

func (s *CommandService) GetPendingCommandItemsByArea(ctx context.Context, area string) ([]*dto.CommandItemSSE, error) {
	commands, err := s.commandRepo.FindPendingByArea(ctx, area)
	if err != nil {
		return nil, err
	}

	var items []*dto.CommandItemSSE
	for _, cmd := range commands {
		userName := ""
		if cmd.CreatedBy != nil {
			userName = cmd.CreatedBy.Username
		}

		for _, item := range cmd.Items {
			if item.Status != dto.CommandStatusCreated {
				continue
			}
			items = append(items, &dto.CommandItemSSE{
				OpenBillProductID:  item.OpenBillProductID,
				OpenBillID:         cmd.OpenBillID,
				ProductName:        item.ProductName,
				Quantity:           item.Quantity,
				Notes:              item.Notes,
				TemporalIdentifier: cmd.TemporalIdentifier,
				Priority:           item.Priority,
				CreatedAt:          cmd.CreatedAt,
				Name:               userName,
			})
		}
	}

	return items, nil
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

func (s *CommandService) HandleOrderUpdated(ctx context.Context, event dto.OrderUpdatedEvent) error {
	existingCommands, err := s.commandRepo.FindAllByOpenBillID(ctx, event.OpenBillID)
	if err != nil {
		s.logger.Error("failed to find commands for open bill", zap.String("open_bill_id", event.OpenBillID), zap.Error(err))
		return fmt.Errorf("failed to find commands for open bill: %w", err)
	}

	existingCommandsMap := make(map[string]*command.Aggregate)
	for _, cmd := range existingCommands {
		existingCommandsMap[cmd.OpenBillID()] = cmd
	}

	commandByArea := make(map[string]*command.Aggregate)
	for _, cmd := range existingCommands {
		commandByArea[cmd.Area()] = cmd
	}

	user, err := s.userRepo.FindByID(ctx, event.CreatedByID)
	if err != nil {
		s.logger.Error("failed to get user", zap.Error(err))
		return fmt.Errorf("failed to get user: %w", err)
	}

	createdBy := &dto.OpenBillCreator{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
	}

	previousMap := make(map[string]dto.OrderCreatedEventProduct)
	for _, p := range event.PreviousProducts {
		previousMap[p.OpenBillProductID] = p
	}

	currentMap := make(map[string]dto.OrderCreatedEventProduct)
	for _, p := range event.CurrentProducts {
		currentMap[p.OpenBillProductID] = p
	}

	s.cancelRemovedProducts(ctx, previousMap, currentMap, existingCommandsMap, event.OpenBillID, event.TemporalIdentifier, createdBy.Name)

	if err := s.createNewProducts(ctx, previousMap, currentMap, commandByArea, event.OpenBillID, event.TemporalIdentifier, createdBy); err != nil {
		return err
	}

	s.updateModifiedProducts(ctx, previousMap, currentMap, existingCommandsMap, event.OpenBillID, event.TemporalIdentifier, createdBy.Name)

	for _, cmd := range existingCommands {
		cmd.TryComplete()
		cmd.TryCancel()

		if err := s.commandRepo.Update(ctx, cmd); err != nil {
			s.logger.Error("failed to update command", zap.String("command_id", cmd.ID()), zap.Error(err))
		}

		if cmd.IsCompleted() || cmd.IsCancelled() {
			eventType := CommandCancelledEventType
			if cmd.IsCompleted() {
				eventType = "command.completed"
			}
			if err := s.sseNotifier.NotifyArea(ctx, cmd.Area(), eventType, cmd.ToDTO()); err != nil {
				s.logger.Error("failed to notify command state change", zap.Error(err))
			}
		}
	}

	return nil
}

func (s *CommandService) HandleOrderDeleted(ctx context.Context, event dto.OrderDeletedEvent) error {
	cmd, err := s.commandRepo.FindByOpenBillID(ctx, event.OpenBillID)
	if err != nil {
		return fmt.Errorf("failed to find command for open bill: %w", err)
	}

	if cmd.IsCancelled() || cmd.IsCompleted() {
		return nil
	}

	items := cmd.Items()

	if err := cmd.CancelAllItems(); err != nil {
		return fmt.Errorf("failed to cancel command items: %w", err)
	}

	if err := s.commandRepo.Update(ctx, cmd); err != nil {
		return fmt.Errorf("failed to update cancelled command: %w", err)
	}

	if err := s.sseNotifier.NotifyArea(ctx, cmd.Area(), CommandCancelledEventType, cmd.ToDTO()); err != nil {
		s.logger.Error("failed to notify area about cancelled command",
			zap.String("area", cmd.Area()),
			zap.Error(err),
		)
	}

	userName := ""
	if cmd.CreatedBy() != nil {
		userName = cmd.CreatedBy().Username
	}

	for _, item := range items {
		itemSSE := &dto.CommandItemSSE{
			OpenBillProductID:  item.OpenBillProductID(),
			OpenBillID:         cmd.OpenBillID(),
			ProductName:        item.ProductName(),
			Quantity:           item.Quantity(),
			Notes:              item.Notes(),
			TemporalIdentifier: cmd.TemporalIdentifier(),
			Priority:           item.Priority(),
			CreatedAt:          cmd.CreatedAt(),
			Name:               userName,
		}
		if err := s.commandItemSSENotifier.NotifyArea(ctx, cmd.Area(), CommandItemCancelledEventType, itemSSE); err != nil {
			s.logger.Error("failed to notify command item cancelled",
				zap.String("area", cmd.Area()),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (s *CommandService) cancelRemovedProducts(
	ctx context.Context,
	previousMap map[string]dto.OrderCreatedEventProduct,
	currentMap map[string]dto.OrderCreatedEventProduct,
	existingCommandsMap map[string]*command.Aggregate,
	openBillID string,
	temporalIdentifier string,
	createdByName string,
) {
	var productsToCancel []dto.OrderCreatedEventProduct
	for openBillProductID, product := range previousMap {
		if _, exists := currentMap[openBillProductID]; !exists {
			productsToCancel = append(productsToCancel, product)
		}
	}

	for _, productToCancel := range productsToCancel {
		cmd, exists := existingCommandsMap[openBillID]
		if !exists {
			continue
		}
		item := cmd.GetItemByOpenBillProductID(productToCancel.OpenBillProductID)
		if item == nil || !item.IsCreated() {
			continue
		}
		if err := cmd.CancelItemByOpenBillProductID(productToCancel.OpenBillProductID); err != nil {
			s.logger.Error("failed to cancel command item",
				zap.String("open_bill_product_id", productToCancel.OpenBillProductID),
				zap.Error(err),
			)
			continue
		}
		// TODO: Think of how to handle if an error occurred in the middle, the notification was sent, but the command was not updated correctly
		itemSSE := &dto.CommandItemSSE{
			OpenBillProductID:  item.OpenBillProductID(),
			OpenBillID:         openBillID,
			ProductName:        item.ProductName(),
			Quantity:           productToCancel.Quantity,
			Notes:              productToCancel.Notes,
			TemporalIdentifier: temporalIdentifier,
			Priority:           item.Priority(),
			CreatedAt:          cmd.CreatedAt(),
			Name:               createdByName,
		}
		if err := s.commandItemSSENotifier.NotifyArea(ctx, cmd.Area(), CommandItemCancelledEventType, itemSSE); err != nil {
			s.logger.Error("failed to notify command item cancelled", zap.Error(err))
		}
	}
}

func (s *CommandService) createNewProducts(
	ctx context.Context,
	previousMap map[string]dto.OrderCreatedEventProduct,
	currentMap map[string]dto.OrderCreatedEventProduct,
	commandByArea map[string]*command.Aggregate,
	openBillID string,
	temporalIdentifier string,
	createdBy *dto.OpenBillCreator,
) error {
	var productsToCreate []dto.OrderCreatedEventProduct
	for openBillProductID, product := range currentMap {
		if _, exists := previousMap[openBillProductID]; !exists {
			productsToCreate = append(productsToCreate, product)
		}
	}

	if len(productsToCreate) == 0 {
		return nil
	}

	productIDs := make([]string, len(productsToCreate))
	for i, p := range productsToCreate {
		productIDs[i] = p.ProductID
	}

	responsibilities, err := s.commandRepo.GetProductPreparationResponsibilities(ctx, productIDs)
	if err != nil {
		return fmt.Errorf("failed to get product preparation responsibilities: %w", err)
	}

	responsibilityMap := make(map[string]*dto.ProductPreparationResponsibilityWithProduct)
	for i := range responsibilities {
		responsibilityMap[responsibilities[i].ProductID] = &responsibilities[i]
	}

	newItemsByArea := make(map[string][]*command_item.Aggregate)
	now := time.Now()

	for _, p := range productsToCreate {
		responsibility, exists := responsibilityMap[p.ProductID]
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

		newItemsByArea[responsibility.Area] = append(newItemsByArea[responsibility.Area], item)
	}

	for area, items := range newItemsByArea {
		if cmd, exists := commandByArea[area]; exists {
			if err := cmd.AddItems(items); err != nil {
				s.logger.Error("failed to add items to command", zap.String("area", area), zap.Error(err))
				continue
			}

			for _, item := range items {
				itemSSE := &dto.CommandItemSSE{
					OpenBillProductID:  item.OpenBillProductID(),
					OpenBillID:         openBillID,
					ProductName:        item.ProductName(),
					Quantity:           item.Quantity(),
					Notes:              item.Notes(),
					TemporalIdentifier: temporalIdentifier,
					Priority:           item.Priority(),
					CreatedAt:          now,
					Name:               createdBy.Name,
				}
				if err := s.commandItemSSENotifier.NotifyArea(ctx, area, CommandItemCreatedEventType, itemSSE); err != nil {
					s.logger.Error("failed to notify command item created", zap.Error(err))
				}
			}
		} else {
			cmd, err := command.NewCommand(
				uuid.New().String(),
				openBillID,
				temporalIdentifier,
				createdBy,
				area,
				items,
				now,
				now,
			)
			if err != nil {
				s.logger.Error("failed to create command", zap.String("area", area), zap.Error(err))
				continue
			}

			if err := s.commandRepo.Create(ctx, cmd); err != nil {
				s.logger.Error("failed to save command", zap.String("area", area), zap.Error(err))
				continue
			}

			if err := s.sseNotifier.NotifyArea(ctx, area, CommandCreatedEventType, cmd.ToDTO()); err != nil {
				s.logger.Error("failed to notify command created", zap.Error(err))
			}

			for _, item := range items {
				itemSSE := &dto.CommandItemSSE{
					OpenBillProductID:  item.OpenBillProductID(),
					OpenBillID:         openBillID,
					ProductName:        item.ProductName(),
					Quantity:           item.Quantity(),
					Notes:              item.Notes(),
					TemporalIdentifier: temporalIdentifier,
					Priority:           item.Priority(),
					CreatedAt:          now,
					Name:               createdBy.Name,
				}
				if err := s.commandItemSSENotifier.NotifyArea(ctx, area, CommandItemCreatedEventType, itemSSE); err != nil {
					s.logger.Error("failed to notify command item created", zap.Error(err))
				}
			}

			commandByArea[area] = cmd
		}
	}

	return nil
}

func (s *CommandService) updateModifiedProducts(
	ctx context.Context,
	previousMap map[string]dto.OrderCreatedEventProduct,
	currentMap map[string]dto.OrderCreatedEventProduct,
	existingCommandsMap map[string]*command.Aggregate,
	openBillID string,
	temporalIdentifier string,
	createdByName string,
) {
	for openBillProductID, currentProduct := range currentMap {
		if _, exists := previousMap[openBillProductID]; !exists {
			continue
		}

		cmd, exists := existingCommandsMap[openBillID]
		if !exists {
			continue
		}

		if !cmd.HasItemChanged(openBillProductID, currentProduct.Quantity, currentProduct.Notes) {
			continue
		}

		item := cmd.GetItemByOpenBillProductID(openBillProductID)
		if item == nil || !item.IsCreated() {
			continue
		}

		if err := cmd.UpdateItemByOpenBillProductID(openBillProductID, currentProduct.Quantity, currentProduct.Notes); err != nil {
			s.logger.Error("failed to update command item",
				zap.String("open_bill_product_id", openBillProductID),
				zap.Error(err),
			)
			continue
		}

		itemSSE := &dto.CommandItemSSE{
			OpenBillProductID:  item.OpenBillProductID(),
			OpenBillID:         openBillID,
			ProductName:        item.ProductName(),
			Quantity:           currentProduct.Quantity,
			Notes:              currentProduct.Notes,
			TemporalIdentifier: temporalIdentifier,
			Priority:           item.Priority(),
			CreatedAt:          cmd.CreatedAt(),
			Name:               createdByName,
		}
		if err := s.commandItemSSENotifier.NotifyArea(ctx, cmd.Area(), CommandItemUpdatedEventType, itemSSE); err != nil {
			s.logger.Error("failed to notify command item updated", zap.Error(err))
		}
	}
}
