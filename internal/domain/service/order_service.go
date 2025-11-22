package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/samber/lo"
)

type OrderService struct {
	openBillRepo   ports.OpenBillRepository
	productRepo    ports.ProductRepository
	invoiceService *InvoiceService
	taxConfig      dto.TaxConfig
}

func NewOrderService(
	openBillRepo ports.OpenBillRepository,
	productRepo ports.ProductRepository,
	invoiceService *InvoiceService,
) *OrderService {
	return &OrderService{
		openBillRepo:   openBillRepo,
		productRepo:    productRepo,
		taxConfig:      dto.GetDefaultTaxConfig(),
		invoiceService: invoiceService,
	}
}

// CreateOrder creates a new open order with the specified products
// If productIDs is empty, creates an empty order
func (s *OrderService) CreateOrder(ctx context.Context, req *dto.CreateOrderRequest, user dto.UserDomain) (*dto.OpenBill, error) {
	var products []*dto.Product
	var totalAmount float64

	if len(req.Products) > 0 {
		var err error
		products, err = s.productRepo.FindByIDs(ctx, lo.Map(req.Products, func(item dto.OrderProductItem, _ int) string {
			return item.ProductID
		}))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
		}

		if len(products) != len(req.Products) {
			return nil, orderError.ErrProductNotFound
		}

		for _, product := range products {
			totalAmount += product.TotalPriceWithTaxes
		}
	}

	openBill := &dto.OpenBill{
		TemporalIdentifier: req.TemporalIdentifier,
		TotalAmount:        totalAmount,
		CreatedBy:          &user.ID,
		Descriptor:         req.Descriptor,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.openBillRepo.Create(ctx, openBill, req.Products); err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
	}

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
	existingBill, err := s.openBillRepo.FindByID(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	var products []*dto.Product
	var totalAmount float64

	if len(req.Products) > 0 {
		products, err = s.productRepo.FindByIDs(ctx, lo.Map(req.Products, func(item dto.OrderProductItem, _ int) string {
			return item.ProductID
		}))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}

		if len(products) != len(req.Products) {
			return nil, orderError.ErrProductNotFound
		}

		for i, product := range products {
			totalAmount += product.TotalPriceWithTaxes * float64(req.Products[i].Quantity)
		}
	}

	updatedBill := &dto.OpenBill{
		ID:                 existingBill.ID,
		TemporalIdentifier: existingBill.TemporalIdentifier,
		TotalAmount:        totalAmount,
		CreatedBy:          existingBill.CreatedBy,
		Descriptor:         existingBill.Descriptor,
		CreatedAt:          existingBill.CreatedAt,
		UpdatedAt:          time.Now(),
	}

	if err := s.openBillRepo.Update(ctx, openBillID, updatedBill, req.Products); err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
	}

	if len(products) > 0 {
		productDTOs := make([]dto.Product, len(products))
		for i, p := range products {
			productDTOs[i] = *p
		}
		updatedBill.Products = productDTOs
	}

	return updatedBill, nil
}

// PayOrder consolidates an open_bill into a bill
// Moves all information from open_bill to bill (except temporal_identifier)
// Only moves open_bill_products where deleted_at IS NULL to bill_products
func (s *OrderService) PayOrder(ctx context.Context, openBillID string) (*dto.Bill, error) {
	if _, err := s.openBillRepo.FindByID(ctx, openBillID); err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	bill, err := s.openBillRepo.PayOrder(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
	}

	return bill, nil
}
