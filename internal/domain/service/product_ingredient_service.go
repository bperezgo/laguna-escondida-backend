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

	now := time.Now()
	ingredient := &dto.ProductIngredient{
		ID:                  uuid.New().String(),
		CompositeProductID:  compositeProductID,
		IngredientProductID: req.IngredientProductID,
		Quantity:            quantity,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.productIngredientRepo.Create(ctx, ingredient); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductIngredientCreationFailed, err)
	}

	return ingredient, nil
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

	existing.Quantity = quantity
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
