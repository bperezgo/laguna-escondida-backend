package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const CommandCreatedEventType = "command.created"

type CommandService struct {
	commandRepo ports.CommandRepository
	productRepo ports.ProductRepository
	userRepo    ports.UserRepository
	sseNotifier ports.SSENotifier
}

func NewCommandService(
	commandRepo ports.CommandRepository,
	productRepo ports.ProductRepository,
	userRepo ports.UserRepository,
	sseNotifier ports.SSENotifier,
) *CommandService {
	return &CommandService{
		commandRepo: commandRepo,
		productRepo: productRepo,
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

	products, err := s.productRepo.FindByIDs(ctx, productIDs)
	if err != nil {
		return fmt.Errorf("failed to get products: %w", err)
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

	productMap := make(map[string]*dto.Product)
	for _, p := range products {
		productMap[p.ID] = p
	}

	productQuantityMap := make(map[string]int)
	productNotesMap := make(map[string]*string)
	for _, p := range event.Products {
		productQuantityMap[p.ProductID] = p.Quantity
		productNotesMap[p.ProductID] = p.Notes
	}

	areaProductMap := make(map[string][]string)
	for _, r := range responsibilities {
		areaProductMap[r.Area] = append(areaProductMap[r.Area], r.ProductID)
	}

	now := time.Now()
	for area, areaProductIDs := range areaProductMap {
		items := make([]dto.CommandItem, 0, len(areaProductIDs))
		for _, productID := range areaProductIDs {
			product, exists := productMap[productID]
			if !exists {
				continue
			}
			items = append(items, dto.CommandItem{
				ProductID:   productID,
				ProductName: product.Name,
				Quantity:    productQuantityMap[productID],
				Notes:       productNotesMap[productID],
			})
		}

		if len(items) == 0 {
			continue
		}

		command := &dto.Command{
			OpenBillID:         event.OpenBillID,
			TemporalIdentifier: event.TemporalIdentifier,
			CreatedBy:          createdBy,
			Area:               area,
			Status:             dto.CommandStatusPending,
			Items:              items,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		if err := s.commandRepo.Create(ctx, command); err != nil {
			return fmt.Errorf("failed to create command for area %s: %w", area, err)
		}

		if err := s.sseNotifier.NotifyArea(ctx, area, CommandCreatedEventType, command); err != nil {
			return fmt.Errorf("failed to notify area %s: %w", area, err)
		}
	}

	return nil
}

func (s *CommandService) GetPendingCommandsByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	return s.commandRepo.FindPendingByArea(ctx, area)
}
