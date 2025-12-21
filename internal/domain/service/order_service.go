package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/aggregate/customer"
	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type OrderService struct {
	openBillRepo   ports.OpenBillRepository
	productRepo    ports.ProductRepository
	billRepo       ports.BillRepository
	billOwnerRepo  ports.BillOwnerRepository
	invoiceService *InvoiceService
	unitOfWork     ports.UnitOfWork
	taxConfig      dto.TaxConfig
}

func NewOrderService(
	openBillRepo ports.OpenBillRepository,
	productRepo ports.ProductRepository,
	billRepo ports.BillRepository,
	billOwnerRepo ports.BillOwnerRepository,
	invoiceService *InvoiceService,
	unitOfWork ports.UnitOfWork,
) *OrderService {
	return &OrderService{
		openBillRepo:   openBillRepo,
		productRepo:    productRepo,
		taxConfig:      dto.GetDefaultTaxConfig(),
		billRepo:       billRepo,
		billOwnerRepo:  billOwnerRepo,
		invoiceService: invoiceService,
		unitOfWork:     unitOfWork,
	}
}

// CreateOrder creates a new open order with the specified products
// If productIDs is empty, creates an empty order
func (s *OrderService) CreateOrder(
	ctx context.Context,
	req *dto.CreateOrderRequest,
	user dto.UserDomain,
) (*dto.OpenBill, error) {
	var products []*dto.Product
	totalAmount := decimal.Zero

	if len(req.Products) > 0 {
		var err error
		uniqueProductIDs := lo.Uniq(lo.Map(req.Products, func(item dto.OrderProductItem, _ int) string {
			return item.ProductID
		}))

		products, err = s.productRepo.FindByIDs(ctx, uniqueProductIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
		}

		if len(products) != len(uniqueProductIDs) {
			return nil, orderError.ErrProductNotFound
		}

		productPriceMap := make(map[string]*dto.Product)
		for _, product := range products {
			productPriceMap[product.ID] = product
		}

		for _, item := range req.Products {
			if product, exists := productPriceMap[item.ProductID]; exists {
				totalAmount = totalAmount.Add(product.TotalPriceWithTaxes.Mul(decimal.NewFromInt(int64(item.Quantity))))
			}
		}
	}

	openBill := &dto.OpenBill{
		TemporalIdentifier: req.TemporalIdentifier,
		TotalAmount:        totalAmount,
		Descriptor:         req.Descriptor,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.openBillRepo.Create(ctx, openBill, req.Products, user.ID); err != nil {
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
	totalAmount := decimal.Zero

	if len(req.Products) > 0 {
		uniqueProductIDs := lo.Uniq(lo.Map(req.Products, func(item dto.OrderProductItem, _ int) string {
			return item.ProductID
		}))

		products, err = s.productRepo.FindByIDs(ctx, uniqueProductIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}

		if len(products) != len(uniqueProductIDs) {
			return nil, orderError.ErrProductNotFound
		}

		productPriceMap := make(map[string]*dto.Product)
		for _, product := range products {
			productPriceMap[product.ID] = product
		}

		for _, item := range req.Products {
			if product, exists := productPriceMap[item.ProductID]; exists {
				totalAmount = totalAmount.Add(product.TotalPriceWithTaxes.Mul(decimal.NewFromInt(int64(item.Quantity))))
			}
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
func (s *OrderService) PayOrder(ctx context.Context, payOrderCommand command.PayOrderCommand) error {
	return s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		openBillWithProducts, err := s.openBillRepo.FindByID(txCtx, payOrderCommand.OpenBillID)
		if err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
		}

		if payOrderCommand.Customer != nil {
			if err := s.createOrUpdateBillOwner(txCtx, payOrderCommand.Customer); err != nil {
				return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
			}
		}

		products, err := product.NewFromOpenBillProducts(openBillWithProducts.Products)
		if err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		productDTOs := lo.Map(products, func(product *product.Aggregate, _ int) *dto.Product {
			return product.ToDTO()
		})

		billAggregate, err := bill.NewBillFromOpenBillWithProducts(openBillWithProducts, payOrderCommand.PaymentCode, payOrderCommand.Customer)
		if err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		if err := s.openBillRepo.Delete(txCtx, payOrderCommand.OpenBillID); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		// It is better to leave this execution to the end, because internally it calls the invoice provider
		// And if it fails, it will be hard to compensate that operation.
		if err := s.billRepo.Create(txCtx, billAggregate, productDTOs); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		return nil
	})
}

func (s *OrderService) createOrUpdateBillOwner(ctx context.Context, customerDTO *dto.Customer) error {
	existingCustomer, err := s.billOwnerRepo.FindByID(ctx, customerDTO.DocumentNumber)
	if err != nil {
		if errors.Is(err, orderError.ErrBillOwnerNotFound) {
			customerAggregate, err := customer.NewCustomerFromDTO(customerDTO)
			if err != nil {
				return fmt.Errorf("failed to create customer aggregate: %w", err)
			}
			return s.billOwnerRepo.Create(ctx, customerAggregate)
		}
		return fmt.Errorf("failed to find bill owner: %w", err)
	}

	if err := existingCustomer.UpdateFromDTO(customerDTO); err != nil {
		return fmt.Errorf("failed to update customer from DTO: %w", err)
	}
	return s.billOwnerRepo.Update(ctx, existingCustomer)
}

// GetAllActiveOpenBills returns all open bills where deleted_at is NULL
func (s *OrderService) GetAllActiveOpenBills(ctx context.Context) (*dto.OpenBillListResponse, error) {
	openBills, err := s.openBillRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active open bills: %w", err)
	}

	total := len(openBills)
	return &dto.OpenBillListResponse{
		OpenBills: openBills,
		Total:     &total,
	}, nil
}

// GetOpenBillWithProducts returns a specific open bill with inner joins to open_bills_products and products
func (s *OrderService) GetOpenBillWithProducts(ctx context.Context, openBillID string) (*dto.OpenBillWithProducts, error) {
	openBill, err := s.openBillRepo.FindByIDWithProducts(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	return openBill, nil
}
