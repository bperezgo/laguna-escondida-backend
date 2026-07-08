package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

type ProductService struct {
	productRepo         ports.ProductRepository
	supplierRepo        ports.SupplierRepository
	supplierCatalogRepo ports.SupplierCatalogRepository
	unitOfWork          ports.UnitOfWork
}

func NewProductService(
	productRepo ports.ProductRepository,
	supplierRepo ports.SupplierRepository,
	supplierCatalogRepo ports.SupplierCatalogRepository,
	unitOfWork ports.UnitOfWork,
) *ProductService {
	return &ProductService{
		productRepo:         productRepo,
		supplierRepo:        supplierRepo,
		supplierCatalogRepo: supplierCatalogRepo,
		unitOfWork:          unitOfWork,
	}
}

// CreateProduct creates a new product with version = 1, optionally creating its
// preparation responsibility (area + priority) in the same transaction.
func (s *ProductService) CreateProduct(ctx context.Context, req *dto.CreateProductRequest) (*dto.Product, error) {
	if err := validateResponsibilityInput(req.PreparationResponsibility.Value); err != nil {
		return nil, err
	}

	aggregate, err := product.NewAggregateFromCreateProductRequest(req)
	if err != nil {
		return nil, err
	}
	productDTO := aggregate.ToDTO()

	var responsibility *dto.ProductPreparationResponsibility
	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.productRepo.Create(ctx, aggregate); err != nil {
			return err
		}
		if input := req.PreparationResponsibility.Value; input != nil {
			created, err := s.productRepo.CreatePreparationResponsibility(ctx, productDTO.ID, input.Area, input.Priority)
			if err != nil {
				return err
			}
			responsibility = created
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	productDTO.PreparationResponsibility = responsibility
	return productDTO, nil
}

// UpdateProduct updates an existing product, keeping version = 1, and reconciles
// its preparation responsibility in the same transaction based on the request's
// preparation_responsibility field (absent = unchanged, null = removed, object =
// created/updated).
func (s *ProductService) UpdateProduct(ctx context.Context, id string, req *dto.UpdateProductRequest) (*dto.Product, error) {
	if err := validateResponsibilityInput(req.PreparationResponsibility.Value); err != nil {
		return nil, err
	}

	existing, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	currentProduct, err := product.NewAggregateFromDTO(existing)
	if err != nil {
		return nil, err
	}

	newProduct, err := currentProduct.Update(req)
	if err != nil {
		return nil, err
	}
	productDTO := newProduct.ToDTO()

	var responsibility *dto.ProductPreparationResponsibility
	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.productRepo.Update(ctx, id, newProduct); err != nil {
			return err
		}

		existingMap, err := s.productRepo.FindPreparationResponsibilitiesByProductIDs(ctx, []string{id})
		if err != nil {
			return err
		}
		current := existingMap[id]

		// Field omitted: leave the responsibility as it is.
		if !req.PreparationResponsibility.Set {
			responsibility = current
			return nil
		}

		input := req.PreparationResponsibility.Value
		switch {
		case input != nil && current != nil:
			updated, err := s.productRepo.UpdatePreparationResponsibility(ctx, current.ID, input.Area, input.Priority)
			if err != nil {
				return err
			}
			responsibility = updated
		case input != nil && current == nil:
			created, err := s.productRepo.CreatePreparationResponsibility(ctx, id, input.Area, input.Priority)
			if err != nil {
				return err
			}
			responsibility = created
		case input == nil && current != nil:
			if err := s.productRepo.DeletePreparationResponsibility(ctx, current.ID); err != nil {
				return err
			}
			responsibility = nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	productDTO.PreparationResponsibility = responsibility
	return productDTO, nil
}

// validateResponsibilityInput guards the embedded responsibility payload. The
// frontend already validates these; this is defense-in-depth for direct callers.
func validateResponsibilityInput(input *dto.ProductResponsibilityInput) error {
	if input == nil {
		return nil
	}
	if input.Area == "" {
		return fmt.Errorf("preparation responsibility area is required")
	}
	if input.Priority < 0 {
		return fmt.Errorf("preparation responsibility priority must be greater than or equal to 0")
	}
	return nil
}

// DeleteProduct soft deletes a product
func (s *ProductService) DeleteProduct(ctx context.Context, id string) error {
	// Check if product exists
	if _, err := s.productRepo.FindByID(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	if err := s.productRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductDeleteFailed, err)
	}

	return nil
}

// ListProducts returns all non-deleted products, optionally filtered by product type
func (s *ProductService) ListProducts(ctx context.Context, filter dto.ListProductsRequest) ([]*dto.Product, error) {
	products, err := s.productRepo.FindAll(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	if err := s.attachResponsibilities(ctx, products); err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	return products, nil
}

// GetProductByID returns a product by its ID
func (s *ProductService) GetProductByID(ctx context.Context, id string) (*dto.Product, error) {
	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	if err := s.attachResponsibilities(ctx, []*dto.Product{product}); err != nil {
		return nil, fmt.Errorf("failed to load product responsibility: %w", err)
	}

	return product, nil
}

// attachResponsibilities enriches the given products with their single preparation
// responsibility (area + priority) using one batched query. Products without a
// responsibility are left untouched (nil).
func (s *ProductService) attachResponsibilities(ctx context.Context, products []*dto.Product) error {
	if len(products) == 0 {
		return nil
	}

	ids := make([]string, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}

	respMap, err := s.productRepo.FindPreparationResponsibilitiesByProductIDs(ctx, ids)
	if err != nil {
		return err
	}

	for _, p := range products {
		if resp, ok := respMap[p.ID]; ok {
			p.PreparationResponsibility = resp
		}
	}

	return nil
}

// CreateProductResponsibility assigns a preparation responsibility (area) to a product
func (s *ProductService) CreateProductResponsibility(ctx context.Context, req *dto.CreateProductResponsibilityRequest) (*dto.ProductPreparationResponsibility, error) {
	product, err := s.productRepo.FindByName(ctx, req.ProductName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	responsibility, err := s.productRepo.CreatePreparationResponsibility(ctx, product.ID, req.Area, req.Priority)
	if err != nil {
		return nil, fmt.Errorf("failed to create product responsibility: %w", err)
	}

	return responsibility, nil
}

// UpdateProductResponsibility updates an existing product responsibility
func (s *ProductService) UpdateProductResponsibility(ctx context.Context, id string, req *dto.UpdateProductResponsibilityRequest) (*dto.ProductPreparationResponsibility, error) {
	responsibility, err := s.productRepo.UpdatePreparationResponsibility(ctx, id, req.Area, req.Priority)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductResponsibilityUpdateFailed, err)
	}

	if responsibility == nil {
		return nil, domainError.ErrProductResponsibilityNotFound
	}

	return responsibility, nil
}

// DeleteProductResponsibility deletes a product responsibility
func (s *ProductService) DeleteProductResponsibility(ctx context.Context, id string) error {
	err := s.productRepo.DeletePreparationResponsibility(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductResponsibilityDeleteFailed, err)
	}

	return nil
}

// GetProductResponsibilityByID retrieves a product responsibility by its ID
func (s *ProductService) GetProductResponsibilityByID(ctx context.Context, id string) (*dto.ProductPreparationResponsibility, error) {
	responsibility, err := s.productRepo.FindPreparationResponsibilityByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductResponsibilityNotFound, err)
	}

	return responsibility, nil
}

// BulkCreateProducts creates multiple products, optionally linking them to a supplier
func (s *ProductService) BulkCreateProducts(ctx context.Context, req *dto.BulkCreateProductRequest) (*dto.BulkCreateProductResponse, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("%w: items cannot be empty", domainError.ErrProductCreationFailed)
	}

	// Validate supplier exists if provided
	if req.SupplierID != nil {
		if _, err := s.supplierRepo.FindByID(ctx, *req.SupplierID); err != nil {
			return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
		}
	}

	// Extract all SKUs and check for duplicates in database
	skus := make([]string, len(req.Items))
	for i, item := range req.Items {
		skus[i] = item.SKU
	}

	existingProducts, err := s.productRepo.FindBySKUs(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductCreationFailed, err)
	}

	existingSKUs := make(map[string]bool, len(existingProducts))
	for _, p := range existingProducts {
		existingSKUs[p.SKU] = true
	}

	response := &dto.BulkCreateProductResponse{
		Created: make([]*dto.Product, 0),
		Errors:  make([]dto.BulkCreateProductError, 0),
	}

	for i, item := range req.Items {
		if existingSKUs[item.SKU] {
			response.Errors = append(response.Errors, dto.BulkCreateProductError{
				Index:   i,
				SKU:     item.SKU,
				Name:    item.Name,
				Message: "product with this SKU already exists",
			})
			continue
		}

		createReq := &dto.CreateProductRequest{
			Name:                item.Name,
			Category:            item.Category,
			ProductType:         item.ProductType,
			UnitOfMeasure:       item.UnitOfMeasure,
			VAT:                 item.VAT,
			ICO:                 item.ICO,
			TaxesFormat:         item.TaxesFormat,
			Description:         item.Description,
			SKU:                 item.SKU,
			TotalPriceWithTaxes: item.TotalPriceWithTaxes,
		}

		aggregate, err := product.NewAggregateFromCreateProductRequest(createReq)
		if err != nil {
			response.Errors = append(response.Errors, dto.BulkCreateProductError{
				Index:   i,
				SKU:     item.SKU,
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}

		if err := s.productRepo.Create(ctx, aggregate); err != nil {
			response.Errors = append(response.Errors, dto.BulkCreateProductError{
				Index:   i,
				SKU:     item.SKU,
				Name:    item.Name,
				Message: fmt.Sprintf("failed to save: %s", err.Error()),
			})
			continue
		}

		productDTO := aggregate.ToDTO()
		response.Created = append(response.Created, productDTO)

		// Link to supplier catalog if supplier ID is provided
		if req.SupplierID != nil {
			now := time.Now()
			catalog := &dto.SupplierCatalog{
				ID:          uuid.Must(uuid.NewV7()).String(),
				SupplierID:  *req.SupplierID,
				ProductID:   productDTO.ID,
				SupplierSKU: item.SupplierSKU,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			if err := s.supplierCatalogRepo.Create(ctx, catalog); err != nil {
				response.Errors = append(response.Errors, dto.BulkCreateProductError{
					Index:   i,
					SKU:     item.SKU,
					Name:    item.Name,
					Message: fmt.Sprintf("product created but failed to link to supplier: %s", err.Error()),
				})
			}
		}
	}

	return response, nil
}

// ListCategories returns all unique product categories
func (s *ProductService) ListCategories(ctx context.Context) ([]string, error) {
	categories, err := s.productRepo.FindAllCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	return categories, nil
}
