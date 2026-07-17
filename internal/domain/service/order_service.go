package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/aggregate/customer"
	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	pkgports "laguna-escondida/backend/pkg/domain/ports"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const OpenBillProductCreatedEventType = "open_bill_product.created"
const OpenBillProductCancelledEventType = "open_bill_product.cancelled"
const OpenBillProductUpdatedEventType = "open_bill_product.updated"

type OrderService struct {
	logger                      *zap.Logger
	openBillRepo                ports.OpenBillRepository
	productRepo                 ports.ProductRepository
	billRepo                    ports.BillRepository
	pendingInvoiceRepo          ports.PendingInvoiceRepository
	pendingInvoiceInitialStatus dto.PendingInvoiceStatus
	billOwnerRepo               ports.BillOwnerRepository
	unitOfWork                  ports.UnitOfWork
	eventBus                    pkgports.EventBus
	userRepo                    ports.UserRepository
	openBillProductSSENotifier  ports.OpenBillProductSSENotifier
	outboxRepo                  ports.SyncOutboxRepository
	syncIdentity                dto.SyncIdentity
	taxConfig                   dto.TaxConfig
}

func NewOrderService(
	openBillRepo ports.OpenBillRepository,
	productRepo ports.ProductRepository,
	billRepo ports.BillRepository,
	pendingInvoiceRepo ports.PendingInvoiceRepository,
	pendingInvoiceInitialStatus dto.PendingInvoiceStatus,
	billOwnerRepo ports.BillOwnerRepository,
	unitOfWork ports.UnitOfWork,
	eventBus pkgports.EventBus,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
) *OrderService {
	return &OrderService{
		openBillRepo:                openBillRepo,
		productRepo:                 productRepo,
		taxConfig:                   dto.GetDefaultTaxConfig(),
		billRepo:                    billRepo,
		pendingInvoiceRepo:          pendingInvoiceRepo,
		pendingInvoiceInitialStatus: pendingInvoiceInitialStatus,
		billOwnerRepo:               billOwnerRepo,
		unitOfWork:                  unitOfWork,
		eventBus:                    eventBus,
		outboxRepo:                  outboxRepo,
		syncIdentity:                syncIdentity,
	}
}

func NewOrderServiceWithSSE(
	logger *zap.Logger,
	openBillRepo ports.OpenBillRepository,
	productRepo ports.ProductRepository,
	billRepo ports.BillRepository,
	pendingInvoiceRepo ports.PendingInvoiceRepository,
	pendingInvoiceInitialStatus dto.PendingInvoiceStatus,
	billOwnerRepo ports.BillOwnerRepository,
	unitOfWork ports.UnitOfWork,
	eventBus pkgports.EventBus,
	userRepo ports.UserRepository,
	openBillProductSSENotifier ports.OpenBillProductSSENotifier,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
) *OrderService {
	return &OrderService{
		logger:                      logger,
		openBillRepo:                openBillRepo,
		productRepo:                 productRepo,
		taxConfig:                   dto.GetDefaultTaxConfig(),
		billRepo:                    billRepo,
		pendingInvoiceRepo:          pendingInvoiceRepo,
		pendingInvoiceInitialStatus: pendingInvoiceInitialStatus,
		billOwnerRepo:               billOwnerRepo,
		unitOfWork:                  unitOfWork,
		eventBus:                    eventBus,
		userRepo:                    userRepo,
		openBillProductSSENotifier:  openBillProductSSENotifier,
		outboxRepo:                  outboxRepo,
		syncIdentity:                syncIdentity,
	}
}

// CreateOrder creates a new open order with the specified products
// If productIDs is empty, creates an empty order
func (s *OrderService) CreateOrder(
	ctx context.Context,
	req *dto.CreateOrderRequest,
	user dto.UserDomain,
) (*dto.OpenBill, error) {
	// Reject a duplicate before doing any work: two active orders must never share a
	// temporal identifier. "Active" excludes completed/cancelled/deleted orders, so a
	// finalized order's identifier can be reused.
	duplicateActive, checkErr := s.openBillRepo.ExistsActiveByTemporalIdentifier(ctx, req.TemporalIdentifier)
	if checkErr != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, checkErr)
	}
	if duplicateActive {
		return nil, orderError.ErrDuplicateTemporalIdentifier
	}

	var products []*dto.Product
	totalAmount := decimal.Zero
	var openBillProducts []*openBill.OpenBillProduct

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

		responsibilities, err := s.openBillRepo.GetProductPreparationResponsibilities(ctx, uniqueProductIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
		}

		responsibilityMap := make(map[string]*dto.ProductPreparationResponsibilityWithProduct)
		for i := range responsibilities {
			responsibilityMap[responsibilities[i].ProductID] = &responsibilities[i]
		}

		productPriceMap := make(map[string]*dto.Product)
		for _, product := range products {
			productPriceMap[product.ID] = product
		}

		for _, item := range req.Products {
			if product, exists := productPriceMap[item.ProductID]; exists {
				totalAmount = totalAmount.Add(product.TotalPriceWithTaxes.Mul(decimal.NewFromInt(int64(item.Quantity))))
			}

			var area *string
			priority := 0
			if resp, exists := responsibilityMap[item.ProductID]; exists {
				area = &resp.Area
				priority = resp.Priority
			}

			openBillProduct, err := openBill.NewOpenBillProduct(
				item.OpenBillProductID,
				item.ProductID,
				item.Quantity,
				item.Notes,
				area,
				priority,
				user.ID,
			)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
			}
			openBillProducts = append(openBillProducts, openBillProduct)
		}
	}

	openBillAggregate, err := openBill.NewAggregate(
		req.OpenBillID,
		req.TemporalIdentifier,
		req.Descriptor,
		totalAmount,
		openBillProducts,
		user.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
	}

	openBillDTO := openBillAggregate.ToDTO()
	openBillDTO.CreatedByID = user.ID

	if len(products) > 0 {
		productDTOs := make([]dto.Product, len(products))
		for i, p := range products {
			productDTOs[i] = *p
		}
		openBillDTO.Products = productDTOs
	}

	// Persist the order and its sync-outbox row in one transaction (Option A): the
	// business change and the row that replicates it to peers commit or roll back
	// together, so a created order can never be lost from the sync log.
	if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.openBillRepo.Create(ctx, openBillAggregate); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
		}
		if err := s.appendOpenBillOutbox(ctx, openBillDTO, user.ID, openBillSyncProducts(openBillAggregate), dto.SyncOperationCreate); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderCreationFailed, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Publish event for SSE notifications after the commit, so subscribers only ever
	// react to an order that is durably persisted.
	if len(req.Products) > 0 {
		event := dto.NewOrderCreatedEvent(
			req.OpenBillID,
			req.TemporalIdentifier,
			user.ID,
			openBillDTO.CreatedAt,
			req.Products,
		)
		if err := s.eventBus.Publish(ctx, event); err != nil {
			s.logger.Error("failed to publish order created event", zap.Error(err))
		}
	}

	return openBillDTO, nil
}

// openBillSyncProducts snapshots an aggregate's line items — including each product's
// kitchen status/area/priority — for an open_bill sync payload, so status transitions
// (complete/cancel/in_progress) replicate to peers, not just header/quantity changes.
func openBillSyncProducts(aggregate *openBill.Aggregate) []dto.OpenBillSyncProduct {
	products := aggregate.Products()
	items := make([]dto.OpenBillSyncProduct, 0, len(products))
	for _, p := range products {
		items = append(items, dto.OpenBillSyncProduct{
			OpenBillProductID: p.ID(),
			ProductID:         p.ProductID(),
			Quantity:          p.Quantity(),
			Notes:             p.Notes(),
			Status:            p.Status(),
			Area:              p.Area(),
			Priority:          p.Priority(),
			CreatedBy:         p.CreatedByID(),
		})
	}
	return items
}

// appendOpenBillOutbox writes one sync_outbox row describing an open_bill change.
// It must be called inside a UnitOfWork transaction so the row commits atomically
// with the business change (Option A). The payload is a full snapshot of the order.
func (s *OrderService) appendOpenBillOutbox(
	ctx context.Context,
	openBillDTO *dto.OpenBill,
	createdByID string,
	products []dto.OpenBillSyncProduct,
	operation dto.SyncOperation,
) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate open_bill outbox op_id: %w", err)
	}

	payload := dto.OpenBillSyncPayload{
		ID:                 openBillDTO.ID,
		TemporalIdentifier: openBillDTO.TemporalIdentifier,
		Descriptor:         openBillDTO.Descriptor,
		TotalAmount:        openBillDTO.TotalAmount,
		Status:             openBillDTO.Status,
		CreatedByID:        createdByID,
		Products:           products,
		CreatedAt:          openBillDTO.CreatedAt,
		UpdatedAt:          openBillDTO.UpdatedAt,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal open_bill sync payload: %w", err)
	}

	return s.outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: s.syncIdentity.NodeID,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     openBillDTO.ID,
		Operation:    operation,
		Payload:      payloadBytes,
	})
}

// appendOpenBillDeleteOutbox writes a delete (tombstone) sync_outbox row for an
// open_bill. It must be called inside a UnitOfWork transaction (Option A).
func (s *OrderService) appendOpenBillDeleteOutbox(ctx context.Context, openBillID string) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate open_bill outbox op_id: %w", err)
	}

	payloadBytes, err := json.Marshal(dto.SyncTombstone{ID: openBillID})
	if err != nil {
		return fmt.Errorf("marshal open_bill tombstone: %w", err)
	}

	return s.outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: s.syncIdentity.NodeID,
		EntityType:   dto.SyncEntityOpenBill,
		EntityID:     openBillID,
		Operation:    dto.SyncOperationDelete,
		Payload:      payloadBytes,
	})
}

// UpdateOrder updates an existing open order with new products and quantities
// If product is new, creates it with quantity
// If product exists with different quantity, updates the quantity
// If product is removed, soft deletes it (sets deleted_at)
func (s *OrderService) UpdateOrder(ctx context.Context, openBillID string, req *dto.UpdateOrderRequest, user dto.UserDomain) (*dto.OpenBill, error) {
	existingOpenBill, err := s.openBillRepo.FindByIDWithProducts(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	existingBillAggregate, err := s.openBillRepo.FindAggregateByID(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	var products []*dto.Product
	totalAmount := decimal.Zero
	var openBillProducts []*openBill.OpenBillProduct

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

		responsibilities, err := s.openBillRepo.GetProductPreparationResponsibilities(ctx, uniqueProductIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}

		responsibilityMap := make(map[string]*dto.ProductPreparationResponsibilityWithProduct)
		for i := range responsibilities {
			responsibilityMap[responsibilities[i].ProductID] = &responsibilities[i]
		}

		productPriceMap := make(map[string]*dto.Product)
		for _, p := range products {
			productPriceMap[p.ID] = p
		}

		existingProductsByID := make(map[string]*openBill.OpenBillProduct)
		for _, p := range existingBillAggregate.Products() {
			existingProductsByID[p.ID()] = p
		}

		for _, item := range req.Products {
			if p, exists := productPriceMap[item.ProductID]; exists {
				totalAmount = totalAmount.Add(p.TotalPriceWithTaxes.Mul(decimal.NewFromInt(int64(item.Quantity))))
			}

			var area *string
			priority := 0
			if resp, exists := responsibilityMap[item.ProductID]; exists {
				area = &resp.Area
				priority = resp.Priority
			}

			var openBillProduct *openBill.OpenBillProduct
			if existing, exists := existingProductsByID[item.OpenBillProductID]; exists {
				openBillProduct, err = openBill.NewOpenBillProductFromRepository(
					item.OpenBillProductID,
					item.ProductID,
					item.Quantity,
					item.Notes,
					existing.Status(),
					area,
					priority,
					existing.CreatedByID(),
				)
			} else {
				openBillProduct, err = openBill.NewOpenBillProduct(
					item.OpenBillProductID,
					item.ProductID,
					item.Quantity,
					item.Notes,
					area,
					priority,
					user.ID,
				)
			}
			if err != nil {
				return nil, fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
			}
			openBillProducts = append(openBillProducts, openBillProduct)
		}
	}

	existingBillAggregate.UpdateProducts(openBillProducts, totalAmount)
	existingBillAggregate.UpdateInfo(req.TemporalIdentifier, req.Descriptor)

	updatedBill := existingBillAggregate.ToDTO()
	updatedBill.CreatedByID = existingOpenBill.CreatedBy.ID
	if len(products) > 0 {
		productDTOs := make([]dto.Product, len(products))
		for i, p := range products {
			productDTOs[i] = *p
		}
		updatedBill.Products = productDTOs
	}

	// Persist the update and its sync-outbox row in one transaction (Option A).
	if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.openBillRepo.Update(ctx, existingBillAggregate); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}
		if err := s.appendOpenBillOutbox(ctx, updatedBill, existingOpenBill.CreatedBy.ID, openBillSyncProducts(existingBillAggregate), dto.SyncOperationUpdate); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	event := dto.NewOrderUpdatedEvent(
		openBillID,
		existingBillAggregate.TemporalIdentifier(),
		existingOpenBill.CreatedBy.ID,
		existingOpenBill.Products,
		req.Products,
	)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish order updated event", zap.Error(err))
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
			if err = s.createOrUpdateBillOwner(txCtx, payOrderCommand.Customer); err != nil {
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

		if err = s.openBillRepo.Delete(txCtx, payOrderCommand.OpenBillID); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		// Persist the finalized bill. The pending invoice is constructed here in the service
		// (where business decisions belong) and persisted separately in the same transaction.
		if err = s.billRepo.Create(txCtx, billAggregate, productDTOs); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		// Replicate the pay outcome to the cloud (Option A, same transaction): the open_bill
		// is gone (tombstone) and a finalized bill now exists. The bill snapshot is the same
		// deterministic DTO the repository persisted. The bill's CUFE syncs later via an update
		// outbox row written when the submitter issues the invoice.
		if err := s.appendOpenBillDeleteOutbox(txCtx, payOrderCommand.OpenBillID); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}
		billDTO := billAggregate.ToDTO()
		if err := s.appendBillCreateOutbox(txCtx, billDTO); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
		}

		// Cash payments do not require an electronic invoice from the fiscal provider,
		// so we skip creating a pending_invoice row and its outbox entry.
		// TODO: remove this exception once cash invoicing is required.
		if payOrderCommand.PaymentCode != dto.ElectronicInvoicePaymentCodeCash {
			pendingInvoiceID, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("%w: generate pending invoice id: %w", orderError.ErrOrderPaymentFailed, err)
			}
			pendingInvoice := &dto.PendingInvoice{
				ID:          pendingInvoiceID.String(),
				BillID:      billDTO.ID,
				PaymentCode: payOrderCommand.PaymentCode,
				Status:      s.pendingInvoiceInitialStatus,
			}
			if err = s.pendingInvoiceRepo.Create(txCtx, pendingInvoice); err != nil {
				return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
			}
			if err := s.appendPendingInvoiceCreateOutbox(txCtx, pendingInvoice); err != nil {
				return fmt.Errorf("%w: %w", orderError.ErrOrderPaymentFailed, err)
			}
		}

		return nil
	})
}

// appendPendingInvoiceCreateOutbox writes one sync_outbox row so the cloud can create a
// pending_invoice and submit the invoice on behalf of this edge node. Only the payment code
// is included — consecutive and request_payload are assigned by the cloud at submission time.
// This runs in both modes; in cloud mode there is no push loop so the row is never sent.
func (s *OrderService) appendPendingInvoiceCreateOutbox(ctx context.Context, p *dto.PendingInvoice) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate pending invoice outbox op_id: %w", err)
	}

	payload, err := json.Marshal(dto.PendingInvoiceSyncPayload{
		ID:          p.ID,
		BillID:      p.BillID,
		PaymentCode: p.PaymentCode,
	})
	if err != nil {
		return fmt.Errorf("marshal pending invoice sync payload: %w", err)
	}

	return s.outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: s.syncIdentity.NodeID,
		EntityType:   dto.SyncEntityPendingInvoice,
		EntityID:     p.BillID,
		Operation:    dto.SyncOperationCreate,
		Payload:      payload,
	})
}

// appendBillCreateOutbox writes one sync_outbox row replicating a finalized bill to the
// cloud. It must be called inside a UnitOfWork transaction (Option A). CUFE/Tascode are nil
// here — they are filled in by a later update outbox row once the invoice is submitted.
func (s *OrderService) appendBillCreateOutbox(ctx context.Context, billDTO *dto.Bill) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate bill outbox op_id: %w", err)
	}

	items := make([]dto.BillSyncProduct, 0, len(billDTO.Products))
	for _, p := range billDTO.Products {
		items = append(items, dto.BillSyncProduct{
			ProductID: p.ProductID,
			Quantity:  p.Quantity,
		})
	}

	payload := dto.BillSyncPayload{
		ID:             billDTO.ID,
		Customer:       billDTO.Customer,
		TotalAmount:    billDTO.TotalAmount,
		DiscountAmount: billDTO.DiscountAmount,
		VAT:            billDTO.VAT,
		ICO:            billDTO.ICO,
		Tip:            billDTO.Tip,
		DocumentURL:    billDTO.DocumentURL,
		Products:       items,
		CreatedAt:      billDTO.CreatedAt,
		UpdatedAt:      billDTO.UpdatedAt,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal bill sync payload: %w", err)
	}

	return s.outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: s.syncIdentity.NodeID,
		EntityType:   dto.SyncEntityBill,
		EntityID:     billDTO.ID,
		Operation:    dto.SyncOperationCreate,
		Payload:      payloadBytes,
	})
}

func (s *OrderService) createOrUpdateBillOwner(ctx context.Context, customerDTO *dto.Customer) error {
	existingCustomer, err := s.billOwnerRepo.FindByID(ctx, customerDTO.DocumentNumber)
	if err != nil {
		if errors.Is(err, orderError.ErrBillOwnerNotFound) {
			var customerAggregate *customer.Aggregate
			customerAggregate, err = customer.NewCustomerFromDTO(customerDTO)
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

// GetClosedOpenBills returns the soft-deleted (closed) open bills created within
// [from, to) — the read-only "Órdenes cerradas hoy" list. It reuses the open-bill list
// shape so the frontend can render closed orders with the same card as active ones.
func (s *OrderService) GetClosedOpenBills(ctx context.Context, from, to time.Time) (*dto.OpenBillListResponse, error) {
	openBills, err := s.openBillRepo.FindDeletedByCreatedAtBetween(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get closed open bills: %w", err)
	}

	total := len(openBills)
	return &dto.OpenBillListResponse{
		OpenBills: openBills,
		Total:     &total,
	}, nil
}

// GetClosedOpenBillWithProducts returns a single closed (soft-deleted) open bill with
// its products, for the closed-order detail view and cuenta reprint.
func (s *OrderService) GetClosedOpenBillWithProducts(ctx context.Context, openBillID string) (*dto.OpenBillWithProducts, error) {
	openBill, err := s.openBillRepo.FindByIDIncludingDeletedWithProducts(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	return openBill, nil
}

// GetOpenBillWithProducts returns a specific open bill with inner joins to open_bills_products and products
func (s *OrderService) GetOpenBillWithProducts(ctx context.Context, openBillID string) (*dto.OpenBillWithProducts, error) {
	openBill, err := s.openBillRepo.FindByIDWithProducts(ctx, openBillID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	return openBill, nil
}

// DeleteOrder soft deletes an open order by setting deleted_at
func (s *OrderService) DeleteOrder(ctx context.Context, openBillID string) error {
	_, err := s.openBillRepo.FindByID(ctx, openBillID)
	if err != nil {
		return fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	// Soft-delete the order and append its tombstone outbox row in one transaction.
	if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.openBillRepo.Delete(ctx, openBillID); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderDeletionFailed, err)
		}
		if err := s.appendOpenBillDeleteOutbox(ctx, openBillID); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderDeletionFailed, err)
		}
		return nil
	}); err != nil {
		return err
	}

	// Publish event for SSE notifications after the commit.
	event := dto.NewOrderDeletedEvent(openBillID)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish order deleted event", zap.Error(err))
	}

	return nil
}

// persistAndSyncStatus persists an open_bill product-status transition and replicates
// it to peers in one transaction (Option A): the local status write and the sync_outbox
// snapshot commit together, so kitchen completions/cancellations never stay local.
func (s *OrderService) persistAndSyncStatus(ctx context.Context, aggregate *openBill.Aggregate) error {
	openBillDTO := aggregate.ToDTO()
	openBillDTO.CreatedByID = aggregate.CreatedByID()

	return s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.openBillRepo.UpdateProductStatus(ctx, aggregate); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}
		if err := s.appendOpenBillOutbox(ctx, openBillDTO, aggregate.CreatedByID(), openBillSyncProducts(aggregate), dto.SyncOperationUpdate); err != nil {
			return fmt.Errorf("%w: %w", orderError.ErrOrderUpdateFailed, err)
		}
		return nil
	})
}

// CompleteOpenBillProduct marks a product as completed and updates open_bill status if all products are finalized
func (s *OrderService) CompleteOpenBillProduct(ctx context.Context, openBillID, openBillProductID string) error {
	aggregate, err := s.openBillRepo.FindAggregateByID(ctx, openBillID)
	if err != nil {
		return fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	if err := aggregate.CompleteProduct(openBillProductID); err != nil {
		return err
	}

	if _, err := aggregate.TryComplete(); err != nil {
		return err
	}

	if _, err := aggregate.TryCancel(); err != nil {
		return err
	}

	return s.persistAndSyncStatus(ctx, aggregate)
}

// UncompleteOpenBillProduct reverts a completed product back to "created" (undo a
// kitchen strike-through) and reopens the bill header if it had been auto-completed.
func (s *OrderService) UncompleteOpenBillProduct(ctx context.Context, openBillID, openBillProductID string) error {
	aggregate, err := s.openBillRepo.FindAggregateByID(ctx, openBillID)
	if err != nil {
		return fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	if err := aggregate.UncompleteProduct(openBillProductID); err != nil {
		return err
	}

	return s.persistAndSyncStatus(ctx, aggregate)
}

// SetOpenBillProductInProgress marks a product as in_progress (fails if product is cancelled)
func (s *OrderService) SetOpenBillProductInProgress(ctx context.Context, openBillID, openBillProductID string) error {
	aggregate, err := s.openBillRepo.FindAggregateByID(ctx, openBillID)
	if err != nil {
		return fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	if err := aggregate.SetProductInProgress(openBillProductID); err != nil {
		return err
	}

	return s.persistAndSyncStatus(ctx, aggregate)
}

// CancelOpenBillProduct marks a product as cancelled and updates open_bill status if all products are cancelled
func (s *OrderService) CancelOpenBillProduct(ctx context.Context, openBillID, openBillProductID string) error {
	aggregate, err := s.openBillRepo.FindAggregateByID(ctx, openBillID)
	if err != nil {
		return fmt.Errorf("%w: %w", orderError.ErrOrderNotFound, err)
	}

	if err := aggregate.CancelProduct(openBillProductID); err != nil {
		return err
	}

	if _, err := aggregate.TryComplete(); err != nil {
		return err
	}

	if _, err := aggregate.TryCancel(); err != nil {
		return err
	}

	return s.persistAndSyncStatus(ctx, aggregate)
}

// HandleOrderCreatedSSE notifies frontend via SSE when products with preparation areas are created
func (s *OrderService) HandleOrderCreatedSSE(ctx context.Context, event dto.OrderCreatedEvent) error {
	s.logger.Info("HandleOrderCreatedSSE called",
		zap.String("open_bill_id", event.OpenBillID),
		zap.String("temporal_identifier", event.TemporalIdentifier),
		zap.Int("product_count", len(event.Products)),
	)

	if len(event.Products) == 0 {
		s.logger.Info("HandleOrderCreatedSSE: no products to process")
		return nil
	}

	productIDs := make([]string, len(event.Products))
	for i, p := range event.Products {
		productIDs[i] = p.ProductID
	}

	s.logger.Debug("Fetching product preparation responsibilities", zap.Strings("product_ids", productIDs))
	responsibilities, err := s.openBillRepo.GetProductPreparationResponsibilities(ctx, productIDs)
	if err != nil {
		s.logger.Error("Failed to get product preparation responsibilities", zap.Error(err))
		return fmt.Errorf("failed to get product preparation responsibilities: %w", err)
	}

	s.logger.Info("Product preparation responsibilities retrieved",
		zap.Int("responsibility_count", len(responsibilities)),
	)

	if len(responsibilities) == 0 {
		s.logger.Warn("No preparation responsibilities found for products")
		return nil
	}

	var createdByName string
	if event.CreatedByID != "" {
		user, err := s.userRepo.FindByID(ctx, event.CreatedByID)
		if err != nil {
			s.logger.Error("failed to get user", zap.Error(err))
		} else {
			createdByName = user.Name
		}
	}

	responsibilityMap := make(map[string]*dto.ProductPreparationResponsibilityWithProduct)
	for i := range responsibilities {
		responsibilityMap[responsibilities[i].ProductID] = &responsibilities[i]
	}

	notifiedCount := 0
	for _, p := range event.Products {
		responsibility, exists := responsibilityMap[p.ProductID]
		if !exists {
			s.logger.Debug("No responsibility found for product", zap.String("product_id", p.ProductID))
			continue
		}

		sseData := &dto.OpenBillProductSSE{
			OpenBillProductID:  p.OpenBillProductID,
			OpenBillID:         event.OpenBillID,
			ProductName:        responsibility.ProductName,
			Quantity:           p.Quantity,
			Notes:              p.Notes,
			Area:               responsibility.Area,
			Status:             string(dto.CommandStatusCreated),
			TemporalIdentifier: event.TemporalIdentifier,
			Priority:           responsibility.Priority,
			// Stamp the live payload with the order's real creation instant so the
			// kitchen countdown starts correctly. Without this it defaults to Go's
			// zero time and the line shows "¡URGENTE!" until a refresh.
			CreatedAt:     event.CreatedAt,
			CreatedByName: createdByName,
		}

		s.logger.Info("Notifying SSE clients",
			zap.String("area", responsibility.Area),
			zap.String("event_type", OpenBillProductCreatedEventType),
			zap.String("product_name", responsibility.ProductName),
			zap.Int("quantity", p.Quantity),
		)

		if err := s.openBillProductSSENotifier.NotifyArea(ctx, responsibility.Area, OpenBillProductCreatedEventType, sseData); err != nil {
			s.logger.Error("failed to notify open bill product created", zap.Error(err))
		} else {
			notifiedCount++
			s.logger.Debug("SSE notification sent successfully",
				zap.String("area", responsibility.Area),
				zap.String("product_name", responsibility.ProductName),
			)
		}

	}

	s.logger.Info("HandleOrderCreatedSSE completed",
		zap.Int("products_notified", notifiedCount),
		zap.Int("total_products", len(event.Products)),
	)

	return nil
}

// HandleOrderUpdatedSSE notifies frontend via SSE when order products are updated
func (s *OrderService) HandleOrderUpdatedSSE(ctx context.Context, event dto.OrderUpdatedEvent) error {
	var createdByName string
	if event.CreatedByID != "" {
		user, err := s.userRepo.FindByID(ctx, event.CreatedByID)
		if err != nil {
			s.logger.Error("failed to get user", zap.Error(err))
		} else {
			createdByName = user.Name
		}
	}

	previousMap := make(map[string]dto.OrderCreatedEventProduct)
	for _, p := range event.PreviousProducts {
		previousMap[p.OpenBillProductID] = p
	}

	currentMap := make(map[string]dto.OrderCreatedEventProduct)
	for _, p := range event.CurrentProducts {
		currentMap[p.OpenBillProductID] = p
	}

	// Get responsibilities for all products
	allProductIDs := make([]string, 0)
	for _, p := range event.PreviousProducts {
		allProductIDs = append(allProductIDs, p.ProductID)
	}
	for _, p := range event.CurrentProducts {
		allProductIDs = append(allProductIDs, p.ProductID)
	}
	allProductIDs = lo.Uniq(allProductIDs)

	var responsibilityMap map[string]*dto.ProductPreparationResponsibilityWithProduct
	if len(allProductIDs) > 0 {
		responsibilities, err := s.openBillRepo.GetProductPreparationResponsibilities(ctx, allProductIDs)
		if err != nil {
			s.logger.Error("failed to get product preparation responsibilities", zap.Error(err))
		} else {
			responsibilityMap = make(map[string]*dto.ProductPreparationResponsibilityWithProduct)
			for i := range responsibilities {
				responsibilityMap[responsibilities[i].ProductID] = &responsibilities[i]
			}
		}
	}

	// Stamp each live SSE payload with the line's real created_at from the DB.
	// OrderUpdatedEvent doesn't carry per-line timestamps, so without this the
	// payload's CreatedAt defaults to Go's zero time (0001-01-01); the kitchen
	// countdown then reads a huge elapsed and every added/modified line renders as
	// "¡URGENTE!" until a refresh reloads the snapshot. Reading created_at here (the
	// same column FindPendingByArea returns) keeps the live payload in agreement
	// with the refresh path. Mirrors the create-path fix in HandleOrderCreatedSSE.
	createdAtMap := make(map[string]time.Time)
	if openBillWithProducts, err := s.openBillRepo.FindByIDWithProducts(ctx, event.OpenBillID); err != nil {
		s.logger.Error("failed to load open bill for created_at stamping",
			zap.String("open_bill_id", event.OpenBillID), zap.Error(err))
	} else {
		for _, p := range openBillWithProducts.Products {
			createdAtMap[p.OpenBillProductID] = p.CreatedAt
		}
	}

	// Cancel removed products
	for openBillProductID, product := range previousMap {
		if _, exists := currentMap[openBillProductID]; !exists {
			responsibility, hasArea := responsibilityMap[product.ProductID]
			if !hasArea {
				continue
			}

			sseData := &dto.OpenBillProductSSE{
				OpenBillProductID:  openBillProductID,
				OpenBillID:         event.OpenBillID,
				ProductName:        responsibility.ProductName,
				Quantity:           product.Quantity,
				Notes:              product.Notes,
				Area:               responsibility.Area,
				Status:             string(dto.CommandStatusCancelled),
				TemporalIdentifier: event.TemporalIdentifier,
				Priority:           responsibility.Priority,
				CreatedByName:      createdByName,
			}

			if err := s.openBillProductSSENotifier.NotifyArea(ctx, responsibility.Area, OpenBillProductCancelledEventType, sseData); err != nil {
				s.logger.Error("failed to notify open bill product cancelled", zap.Error(err))
			}

		}
	}

	// Create new products and update modified products
	for openBillProductID, currentProduct := range currentMap {
		responsibility, hasArea := responsibilityMap[currentProduct.ProductID]
		if !hasArea {
			continue
		}

		previousProduct, existed := previousMap[openBillProductID]
		if !existed {
			// New product
			sseData := &dto.OpenBillProductSSE{
				OpenBillProductID:  openBillProductID,
				OpenBillID:         event.OpenBillID,
				ProductName:        responsibility.ProductName,
				Quantity:           currentProduct.Quantity,
				Notes:              currentProduct.Notes,
				Area:               responsibility.Area,
				Status:             string(dto.CommandStatusCreated),
				TemporalIdentifier: event.TemporalIdentifier,
				Priority:           responsibility.Priority,
				CreatedAt:          createdAtMap[openBillProductID],
				CreatedByName:      createdByName,
			}

			if err := s.openBillProductSSENotifier.NotifyArea(ctx, responsibility.Area, OpenBillProductCreatedEventType, sseData); err != nil {
				s.logger.Error("failed to notify open bill product created", zap.Error(err))
			}
		} else {
			// Check if modified
			quantityChanged := previousProduct.Quantity != currentProduct.Quantity
			notesChanged := (previousProduct.Notes == nil && currentProduct.Notes != nil) ||
				(previousProduct.Notes != nil && currentProduct.Notes == nil) ||
				(previousProduct.Notes != nil && currentProduct.Notes != nil && *previousProduct.Notes != *currentProduct.Notes)

			if quantityChanged || notesChanged {
				sseData := &dto.OpenBillProductSSE{
					OpenBillProductID:  openBillProductID,
					OpenBillID:         event.OpenBillID,
					ProductName:        responsibility.ProductName,
					Quantity:           currentProduct.Quantity,
					Notes:              currentProduct.Notes,
					Area:               responsibility.Area,
					Status:             string(dto.CommandStatusCreated),
					TemporalIdentifier: event.TemporalIdentifier,
					Priority:           responsibility.Priority,
					CreatedAt:          createdAtMap[openBillProductID],
					CreatedByName:      createdByName,
				}

				if err := s.openBillProductSSENotifier.NotifyArea(ctx, responsibility.Area, OpenBillProductUpdatedEventType, sseData); err != nil {
					s.logger.Error("failed to notify open bill product updated", zap.Error(err))
				}
			}
		}
	}

	return nil
}

// HandleOrderDeletedSSE notifies frontend via SSE when an order is deleted
func (s *OrderService) HandleOrderDeletedSSE(ctx context.Context, event dto.OrderDeletedEvent) error {
	openBill, err := s.openBillRepo.FindByIDWithProducts(ctx, event.OpenBillID)
	if err != nil {
		s.logger.Error("failed to find open bill for deletion event", zap.String("open_bill_id", event.OpenBillID), zap.Error(err))
		return nil
	}

	for _, product := range openBill.Products {
		if product.Area == nil || *product.Area == "" {
			continue
		}

		sseData := &dto.OpenBillProductSSE{
			OpenBillProductID:  product.OpenBillProductID,
			OpenBillID:         event.OpenBillID,
			ProductName:        product.Product.Name,
			Quantity:           product.Quantity,
			Notes:              product.Notes,
			Area:               *product.Area,
			Status:             string(dto.CommandStatusCancelled),
			TemporalIdentifier: openBill.TemporalIdentifier,
			Priority:           product.Priority,
			CreatedByName:      openBill.CreatedBy.Name,
		}

		if err := s.openBillProductSSENotifier.NotifyArea(ctx, *product.Area, OpenBillProductCancelledEventType, sseData); err != nil {
			s.logger.Error("failed to notify open bill product cancelled",
				zap.String("area", *product.Area),
				zap.Error(err),
			)
		}
	}

	return nil
}

// GetPendingOpenBillProductsByArea returns all pending open bill products for a specific area (for initial SSE connection)
func (s *OrderService) GetPendingOpenBillProductsByArea(ctx context.Context, area string) ([]*dto.OpenBillProductSSE, error) {
	return s.openBillRepo.FindPendingByArea(ctx, area)
}

// GetCompletedOpenBillProductsByArea returns the completed lines of comandas that
// are fully done for an area, completed within [from, to) (for the "Comandas Listas"
// review view).
func (s *OrderService) GetCompletedOpenBillProductsByArea(ctx context.Context, area string, from, to time.Time) ([]*dto.OpenBillProductSSE, error) {
	return s.openBillRepo.FindCompletedByAreaBetween(ctx, area, from, to)
}
