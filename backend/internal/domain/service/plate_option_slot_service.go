package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type PlateOptionSlotService struct {
	slotRepo    ports.PlateOptionSlotRepository
	productRepo ports.ProductRepository
}

func NewPlateOptionSlotService(
	slotRepo ports.PlateOptionSlotRepository,
	productRepo ports.ProductRepository,
) *PlateOptionSlotService {
	return &PlateOptionSlotService{
		slotRepo:    slotRepo,
		productRepo: productRepo,
	}
}

func (s *PlateOptionSlotService) Get(ctx context.Context, compositeProductID string) ([]dto.PlateOptionSlot, error) {
	if _, err := s.productRepo.FindByID(ctx, compositeProductID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}
	slots, err := s.slotRepo.FindByCompositeProductID(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plate option slots: %w", err)
	}
	return slots, nil
}

// Replace makes req the plate's whole slot list. It validates the full document before
// writing, so a rejected request leaves the previous configuration untouched.
func (s *PlateOptionSlotService) Replace(ctx context.Context, compositeProductID string, req *dto.ReplacePlateOptionSlotsRequest) ([]dto.PlateOptionSlot, error) {
	plate, err := s.productRepo.FindByID(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}
	if plate.ProductType != dto.ProductTypeComposite {
		return nil, domainError.ErrProductNotComposite
	}

	if err = validateSlotInputs(req.Slots); err != nil {
		return nil, err
	}

	if err = s.ensureIngredientsExist(ctx, compositeProductID, req.Slots); err != nil {
		return nil, err
	}

	existing, err := s.slotRepo.FindByCompositeProductID(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrPlateOptionSlotsUpdateFailed, err)
	}

	slots := buildSlots(compositeProductID, req.Slots, existing, time.Now())
	if err = s.slotRepo.ReplaceForCompositeProduct(ctx, compositeProductID, slots); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrPlateOptionSlotsUpdateFailed, err)
	}

	saved, err := s.slotRepo.FindByCompositeProductID(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plate option slots: %w", err)
	}
	return saved, nil
}

func validateSlotInputs(slots []dto.PlateOptionSlotInput) error {
	slotNames := make(map[string]struct{}, len(slots))
	slotIDs := make(map[string]struct{}, len(slots))
	choiceIDs := make(map[string]struct{})
	for i, slot := range slots {
		name := strings.TrimSpace(slot.Name)
		if name == "" {
			return invalidSlots(fmt.Sprintf("slot %d has no name", i+1))
		}
		if _, dup := slotNames[strings.ToLower(name)]; dup {
			return invalidSlots(fmt.Sprintf("slot %q appears twice", name))
		}
		slotNames[strings.ToLower(name)] = struct{}{}
		if err := claimID(slotIDs, slot.ID, "slot"); err != nil {
			return err
		}

		if len(slot.Choices) == 0 {
			return invalidSlots(fmt.Sprintf("slot %q has no choices", name))
		}
		defaults := 0
		choiceNames := make(map[string]struct{}, len(slot.Choices))
		for _, choice := range slot.Choices {
			choiceName := strings.TrimSpace(choice.Name)
			if choiceName == "" {
				return invalidSlots(fmt.Sprintf("slot %q has a choice with no name", name))
			}
			if _, dup := choiceNames[strings.ToLower(choiceName)]; dup {
				return invalidSlots(fmt.Sprintf("slot %q has the choice %q twice", name, choiceName))
			}
			choiceNames[strings.ToLower(choiceName)] = struct{}{}
			if err := claimID(choiceIDs, choice.ID, "choice"); err != nil {
				return err
			}
			if choice.IsDefault {
				defaults++
			}
			if err := validateBundle(name, choiceName, choice.Items); err != nil {
				return err
			}
		}
		if defaults != 1 {
			return invalidSlots(fmt.Sprintf("slot %q must have exactly one default choice, got %d", name, defaults))
		}
	}
	return nil
}

func validateBundle(slotName, choiceName string, items []dto.PlateOptionChoiceItemInput) error {
	ingredients := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, err := uuid.Parse(item.IngredientProductID); err != nil {
			return invalidSlots(fmt.Sprintf("choice %q of slot %q has an invalid ingredient id", choiceName, slotName))
		}
		if item.Quantity < 1 {
			return invalidSlots(fmt.Sprintf("choice %q of slot %q needs a quantity of at least 1 per ingredient", choiceName, slotName))
		}
		if _, dup := ingredients[item.IngredientProductID]; dup {
			return invalidSlots(fmt.Sprintf("choice %q of slot %q lists an ingredient twice", choiceName, slotName))
		}
		ingredients[item.IngredientProductID] = struct{}{}
	}
	return nil
}

func claimID(seen map[string]struct{}, id, kind string) error {
	if id == "" {
		return nil
	}
	if _, dup := seen[id]; dup {
		return invalidSlots(fmt.Sprintf("%s id %s is sent twice", kind, id))
	}
	seen[id] = struct{}{}
	return nil
}

func invalidSlots(reason string) error {
	return fmt.Errorf("%w: %s", domainError.ErrInvalidPlateOptionSlots, reason)
}

// ensureIngredientsExist resolves every bundle ingredient through the organization-scoped
// product lookup, so another organization's product fails exactly like a missing one.
func (s *PlateOptionSlotService) ensureIngredientsExist(ctx context.Context, compositeProductID string, slots []dto.PlateOptionSlotInput) error {
	var ids []string
	for _, slot := range slots {
		for _, choice := range slot.Choices {
			for _, item := range choice.Items {
				ids = append(ids, item.IngredientProductID)
			}
		}
	}
	ids = lo.Uniq(ids)
	if len(ids) == 0 {
		return nil
	}
	if lo.Contains(ids, compositeProductID) {
		return domainError.ErrIngredientCannotBeSelf
	}
	products, err := s.productRepo.FindByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrPlateOptionSlotsUpdateFailed, err)
	}
	if len(products) != len(ids) {
		return fmt.Errorf("%w: %w", domainError.ErrProductNotFound, domainError.ErrBundleIngredientNotFound)
	}
	return nil
}

// buildSlots assigns ids and positions: a slot or choice keeps its id when it already belongs
// to this plate, and an item keeps its id when its choice already holds that ingredient.
func buildSlots(compositeProductID string, inputs []dto.PlateOptionSlotInput, existing []dto.PlateOptionSlot, now time.Time) []dto.PlateOptionSlot {
	slotCreated := make(map[string]time.Time)
	choiceCreated := make(map[string]time.Time)
	itemByChoiceIngredient := make(map[string]dto.PlateOptionChoiceItem)
	for _, slot := range existing {
		slotCreated[slot.ID] = slot.CreatedAt
		for _, choice := range slot.Choices {
			choiceCreated[choice.ID] = choice.CreatedAt
			for _, item := range choice.Items {
				itemByChoiceIngredient[choice.ID+"|"+item.IngredientProductID] = item
			}
		}
	}

	keepOrNew := func(id string, created map[string]time.Time) (string, time.Time) {
		if at, ok := created[id]; ok {
			return id, at
		}
		return uuid.Must(uuid.NewV7()).String(), now
	}

	slots := make([]dto.PlateOptionSlot, 0, len(inputs))
	for i, in := range inputs {
		slotID, slotCreatedAt := keepOrNew(in.ID, slotCreated)
		choices := make([]dto.PlateOptionChoice, 0, len(in.Choices))
		for j, c := range in.Choices {
			choiceID, choiceCreatedAt := keepOrNew(c.ID, choiceCreated)
			items := make([]dto.PlateOptionChoiceItem, 0, len(c.Items))
			for _, it := range c.Items {
				item := dto.PlateOptionChoiceItem{
					ID:                  uuid.Must(uuid.NewV7()).String(),
					IngredientProductID: it.IngredientProductID,
					Quantity:            it.Quantity,
					CreatedAt:           now,
					UpdatedAt:           now,
				}
				if prev, ok := itemByChoiceIngredient[choiceID+"|"+it.IngredientProductID]; ok {
					item.ID, item.CreatedAt = prev.ID, prev.CreatedAt
				}
				items = append(items, item)
			}
			choices = append(choices, dto.PlateOptionChoice{
				ID:        choiceID,
				Name:      strings.TrimSpace(c.Name),
				Position:  j,
				IsDefault: c.IsDefault,
				Items:     items,
				CreatedAt: choiceCreatedAt,
				UpdatedAt: now,
			})
		}
		slots = append(slots, dto.PlateOptionSlot{
			ID:                 slotID,
			CompositeProductID: compositeProductID,
			Name:               strings.TrimSpace(in.Name),
			Position:           i,
			Choices:            choices,
			CreatedAt:          slotCreatedAt,
			UpdatedAt:          now,
		})
	}
	return slots
}
