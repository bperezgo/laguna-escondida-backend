package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductIngredientService struct {
	productIngredientRepo ports.ProductIngredientRepository
	productRepo           ports.ProductRepository
}

func NewProductIngredientService(
	productIngredientRepo ports.ProductIngredientRepository,
	productRepo ports.ProductRepository,
) *ProductIngredientService {
	return &ProductIngredientService{
		productIngredientRepo: productIngredientRepo,
		productRepo:           productRepo,
	}
}

func (s *ProductIngredientService) AddIngredient(ctx context.Context, compositeProductID string, req *dto.AddIngredientRequest) (*dto.ProductIngredient, error) {
	compositeProduct, err := s.productRepo.FindByID(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	if compositeProduct.ProductType != dto.ProductTypeComposite {
		return nil, domainError.ErrProductNotComposite
	}

	if compositeProductID == req.IngredientProductID {
		return nil, domainError.ErrIngredientCannotBeSelf
	}

	if _, findErr := s.productRepo.FindByID(ctx, req.IngredientProductID); findErr != nil {
		return nil, fmt.Errorf("%w: ingredient product not found: %w", domainError.ErrProductNotFound, findErr)
	}

	existingIngredients, err := s.productIngredientRepo.FindByCompositeProductID(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing ingredients: %w", err)
	}
	for _, existing := range existingIngredients {
		if existing.IngredientProductID == req.IngredientProductID {
			return nil, domainError.ErrProductIngredientAlreadyExists
		}
	}

	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrInvalidIngredientQuantity, err)
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("%w: quantity must be greater than zero", domainError.ErrInvalidIngredientQuantity)
	}

	cycle, err := s.wouldCreateCycle(ctx, compositeProductID, req.IngredientProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to check ingredient cycle: %w", err)
	}
	if cycle {
		return nil, domainError.ErrIngredientCycle
	}

	now := time.Now()
	ingredient := &dto.ProductIngredient{
		ID:                  uuid.Must(uuid.NewV7()).String(),
		CompositeProductID:  compositeProductID,
		IngredientProductID: req.IngredientProductID,
		DefaultQuantity:     quantity,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.productIngredientRepo.Create(ctx, ingredient); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientCreationFailed, err)
	}

	return ingredient, nil
}

// wouldCreateCycle reports whether adding the edge compositeProductID -> ingredientProductID
// would create a cycle in the recipe graph. It walks the prospective ingredient's existing
// subtree; if any path reaches compositeProductID, the new edge closes a loop. A visited set
// keeps the walk bounded even if the current graph already contains a cycle.
func (s *ProductIngredientService) wouldCreateCycle(ctx context.Context, compositeProductID, ingredientProductID string) (bool, error) {
	visited := make(map[string]struct{})

	var reaches func(productID string) (bool, error)
	reaches = func(productID string) (bool, error) {
		if productID == compositeProductID {
			return true, nil
		}
		if _, seen := visited[productID]; seen {
			return false, nil
		}
		visited[productID] = struct{}{}

		edges, err := s.productIngredientRepo.FindByCompositeProductID(ctx, productID)
		if err != nil {
			return false, err
		}
		for _, edge := range edges {
			found, err := reaches(edge.IngredientProductID)
			if err != nil {
				return false, err
			}
			if found {
				return true, nil
			}
		}
		return false, nil
	}

	return reaches(ingredientProductID)
}

func (s *ProductIngredientService) UpdateIngredient(ctx context.Context, compositeProductID, ingredientID string, req *dto.UpdateIngredientRequest) (*dto.ProductIngredient, error) {
	existing, err := s.productIngredientRepo.FindByID(ctx, ingredientID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrProductIngredientNotFound
		}
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientNotFound, err)
	}

	if existing.CompositeProductID != compositeProductID {
		return nil, domainError.ErrProductIngredientNotFound
	}

	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrInvalidIngredientQuantity, err)
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("%w: quantity must be greater than zero", domainError.ErrInvalidIngredientQuantity)
	}

	existing.DefaultQuantity = quantity
	existing.UpdatedAt = time.Now()

	if err := s.productIngredientRepo.Update(ctx, ingredientID, existing); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientUpdateFailed, err)
	}

	return existing, nil
}

func (s *ProductIngredientService) RemoveIngredient(ctx context.Context, compositeProductID, ingredientID string) error {
	existing, err := s.productIngredientRepo.FindByID(ctx, ingredientID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domainError.ErrProductIngredientNotFound
		}
		return fmt.Errorf("%w: %w", domainError.ErrProductIngredientNotFound, err)
	}

	if existing.CompositeProductID != compositeProductID {
		return domainError.ErrProductIngredientNotFound
	}

	if err := s.productIngredientRepo.Delete(ctx, ingredientID); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductIngredientDeleteFailed, err)
	}

	return nil
}

// ConfigureSideDish marks an existing composite ingredient as a customer-configurable side
// dish and sets its default/min/max quantities. Bounds must satisfy 0 <= min <= default <= max
// so a resolved selection always has a valid range.
func (s *ProductIngredientService) ConfigureSideDish(ctx context.Context, compositeProductID, ingredientID string, req *dto.ConfigureSideDishRequest) (*dto.ProductIngredient, error) {
	existing, err := s.productIngredientRepo.FindByID(ctx, ingredientID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrProductIngredientNotFound
		}
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientNotFound, err)
	}

	if existing.CompositeProductID != compositeProductID {
		return nil, domainError.ErrProductIngredientNotFound
	}

	if !validSideDishBounds(req.MinQuantity, req.DefaultQuantity, req.MaxQuantity) {
		return nil, domainError.ErrInvalidSideDishBounds
	}

	existing.IsSideDish = true
	existing.DefaultQuantity = decimal.NewFromInt(int64(req.DefaultQuantity))
	existing.MinQuantity = req.MinQuantity
	existing.MaxQuantity = req.MaxQuantity
	existing.UpdatedAt = time.Now()

	if err := s.productIngredientRepo.Update(ctx, ingredientID, existing); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientUpdateFailed, err)
	}

	return existing, nil
}

// ClearSideDish reverts an ingredient to a fixed (non-configurable) recipe row. Its
// DefaultQuantity is preserved as the exact amount always consumed.
func (s *ProductIngredientService) ClearSideDish(ctx context.Context, compositeProductID, ingredientID string) (*dto.ProductIngredient, error) {
	existing, err := s.productIngredientRepo.FindByID(ctx, ingredientID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrProductIngredientNotFound
		}
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientNotFound, err)
	}

	if existing.CompositeProductID != compositeProductID {
		return nil, domainError.ErrProductIngredientNotFound
	}

	existing.IsSideDish = false
	existing.MinQuantity = 0
	existing.MaxQuantity = 0
	existing.UpdatedAt = time.Now()

	if err := s.productIngredientRepo.Update(ctx, ingredientID, existing); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientUpdateFailed, err)
	}

	return existing, nil
}

// GetSideDishOptions returns the composite's side-dish options with each option's default,
// min, and max so a client can render the +/- controls and enforce bounds.
func (s *ProductIngredientService) GetSideDishOptions(ctx context.Context, compositeProductID string) ([]*dto.SideDishOption, error) {
	if _, err := s.productRepo.FindByID(ctx, compositeProductID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	ingredients, err := s.productIngredientRepo.FindByCompositeProductIDWithProducts(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to get side-dish options: %w", err)
	}

	options := make([]*dto.SideDishOption, 0, len(ingredients))
	for _, ingredient := range ingredients {
		if !ingredient.IsSideDish {
			continue
		}
		options = append(options, &dto.SideDishOption{
			IngredientProductID: ingredient.IngredientProductID,
			IngredientProduct:   ingredient.IngredientProduct,
			DefaultQuantity:     int(ingredient.DefaultQuantity.IntPart()),
			MinQuantity:         ingredient.MinQuantity,
			MaxQuantity:         ingredient.MaxQuantity,
		})
	}

	return options, nil
}

func validSideDishBounds(minQty, defaultQty, maxQty int) bool {
	return 0 <= minQty && minQty <= defaultQty && defaultQty <= maxQty
}

func (s *ProductIngredientService) GetIngredients(ctx context.Context, compositeProductID string) ([]*dto.ProductIngredientWithProduct, error) {
	if _, err := s.productRepo.FindByID(ctx, compositeProductID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	ingredients, err := s.productIngredientRepo.FindByCompositeProductIDWithProducts(ctx, compositeProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingredients: %w", err)
	}

	return ingredients, nil
}

func (s *ProductIngredientService) GetIngredientByID(ctx context.Context, ingredientID string) (*dto.ProductIngredient, error) {
	ingredient, err := s.productIngredientRepo.FindByID(ctx, ingredientID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrProductIngredientNotFound
		}
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientNotFound, err)
	}

	return ingredient, nil
}
