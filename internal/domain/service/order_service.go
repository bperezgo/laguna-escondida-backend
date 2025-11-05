package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type OrderService struct {
	openBillRepo ports.OpenBillRepository
	productRepo  ports.ProductRepository
	taxConfig    dto.TaxConfig
}

func NewOrderService(
	openBillRepo ports.OpenBillRepository,
	productRepo ports.ProductRepository,
) *OrderService {
	return &OrderService{
		openBillRepo: openBillRepo,
		productRepo:  productRepo,
		taxConfig:    dto.GetDefaultTaxConfig(),
	}
}

// CreateOrder creates a new open order with the specified products
// If productIDs is empty, creates an empty order
func (s *OrderService) CreateOrder(ctx context.Context, req *dto.CreateOrderRequest) (*dto.OpenBill, error) {
	var products []*dto.Product
	var totalPrice float64

	// If products are provided, fetch and validate them
	if len(req.ProductIDs) > 0 {
		var err error
		products, err = s.productRepo.FindByIDs(ctx, req.ProductIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
		}

		// Validate that all products were found
		if len(products) != len(req.ProductIDs) {
			return nil, orderError.ErrProductNotFound
		}

		// Calculate total price from products
		for _, product := range products {
			totalPrice += product.Price
		}
	}

	// Calculate taxes and tip based on total_price
	// Note: All taxes are included in the prices, so we calculate based on total_price
	vat := totalPrice * s.taxConfig.VATPercent
	ico := totalPrice * s.taxConfig.ICOPercent
	tip := totalPrice * s.taxConfig.TipPercent

	// Generate temporal identifier (simple timestamp-based for now)
	temporalIdentifier := fmt.Sprintf("ORDER-%d", time.Now().UnixNano())

	openBill := &dto.OpenBill{
		TemporalIdentifier: temporalIdentifier,
		TotalPrice:         totalPrice,
		VAT:                vat,
		ICO:                ico,
		Tip:                tip,
		DocumentURL:        nil,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Convert product IDs to OrderProductItem format with default quantity of 1
	orderProducts := make([]dto.OrderProductItem, len(req.ProductIDs))
	for i, productID := range req.ProductIDs {
		orderProducts[i] = dto.OrderProductItem{
			ProductID: productID,
			Quantity:  1, // Default quantity for CreateOrder
		}
	}

	// Create the open bill in the repository
	if err := s.openBillRepo.Create(ctx, openBill, orderProducts); err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
	}

	// Populate products in the response
	if len(products) > 0 {
		productDTOs := make([]dto.Product, len(products))
		for i, p := range products {
			productDTOs[i] = *p
		}
		openBill.Products = productDTOs
	}

	return openBill, nil
}

// UpdateOrder updates an existing open order with new products and quantities
// If product is new, creates it with quantity
// If product exists with different quantity, updates the quantity
// If product is removed, soft deletes it (sets deleted_at)
func (s *OrderService) UpdateOrder(ctx context.Context, openBillID string, req *dto.UpdateOrderRequest) (*dto.OpenBill, error) {
	// Validate that the open bill exists
	existingBill, err := s.openBillRepo.FindByID(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	// If no products provided, treat as empty order (all products will be soft deleted)
	var products []*dto.Product
	var totalPrice float64
	var productIDs []string

	if len(req.Products) > 0 {
		// Extract product IDs from request
		productIDs = make([]string, len(req.Products))
		for i, item := range req.Products {
			productIDs[i] = item.ProductID
		}

		// Fetch and validate products
		products, err = s.productRepo.FindByIDs(ctx, productIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}

		// Validate that all products were found
		if len(products) != len(req.Products) {
			return nil, orderError.ErrProductNotFound
		}

		// Calculate total price from products with quantities
		for i, product := range products {
			totalPrice += product.Price * float64(req.Products[i].Quantity)
		}
	}

	// Calculate taxes and tip based on total_price
	vat := totalPrice * s.taxConfig.VATPercent
	ico := totalPrice * s.taxConfig.ICOPercent
	tip := totalPrice * s.taxConfig.TipPercent

	// Prepare updated open bill
	updatedBill := &dto.OpenBill{
		ID:                 existingBill.ID,
		TemporalIdentifier: existingBill.TemporalIdentifier,
		TotalPrice:         totalPrice,
		VAT:                vat,
		ICO:                ico,
		Tip:                tip,
		DocumentURL:        existingBill.DocumentURL,
		CreatedAt:          existingBill.CreatedAt,
		UpdatedAt:          time.Now(),
	}

	// Update the open bill in the repository
	if err := s.openBillRepo.Update(ctx, openBillID, updatedBill, req.Products); err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
	}

	// Populate products in the response
	if len(products) > 0 {
		productDTOs := make([]dto.Product, len(products))
		for i, p := range products {
			productDTOs[i] = *p
		}
		updatedBill.Products = productDTOs
	}

	return updatedBill, nil
}
