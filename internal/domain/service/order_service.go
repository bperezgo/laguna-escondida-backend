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

	// Create the open bill in the repository
	if err := s.openBillRepo.Create(ctx, openBill, req.ProductIDs); err != nil {
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
