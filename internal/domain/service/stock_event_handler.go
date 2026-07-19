package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/shopspring/decimal"
)

type StockEventHandler struct {
	stockRepo             ports.StockRepository
	productRepo           ports.ProductRepository
	productIngredientRepo ports.ProductIngredientRepository
	lockManager           *eventbus.ProductLockManager
	unitOfWork            ports.UnitOfWork
	outboxRepo            ports.SyncOutboxRepository
	syncIdentity          dto.SyncIdentity
	logger                *slog.Logger
}

func NewStockEventHandler(
	stockRepo ports.StockRepository,
	productRepo ports.ProductRepository,
	productIngredientRepo ports.ProductIngredientRepository,
	lockManager *eventbus.ProductLockManager,
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
	logger *slog.Logger,
) *StockEventHandler {
	return &StockEventHandler{
		stockRepo:             stockRepo,
		productRepo:           productRepo,
		productIngredientRepo: productIngredientRepo,
		lockManager:           lockManager,
		unitOfWork:            unitOfWork,
		outboxRepo:            outboxRepo,
		syncIdentity:          syncIdentity,
		logger:                logger,
	}
}

// HandleOrderCreated decreases stock when an order is created.
func (h *StockEventHandler) HandleOrderCreated(ctx context.Context, event dto.OrderCreatedEvent) error {
	h.logger.Info("handling order created event for stock",
		slog.String("open_bill_id", event.OpenBillID),
		slog.Int("product_count", len(event.Products)),
	)

	for _, item := range event.Products {
		if err := h.decreaseStockForProduct(ctx, item.ProductID, item.Quantity); err != nil {
			h.logger.Error("failed to decrease stock",
				slog.String("product_id", item.ProductID),
				slog.Int("quantity", item.Quantity),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// HandleOrderUpdated adjusts stock based on product changes between previous and current state.
func (h *StockEventHandler) HandleOrderUpdated(ctx context.Context, event dto.OrderUpdatedEvent) error {
	h.logger.Info("handling order updated event for stock",
		slog.String("open_bill_id", event.OpenBillID),
		slog.Int("previous_count", len(event.PreviousProducts)),
		slog.Int("current_count", len(event.CurrentProducts)),
	)

	previousQty := make(map[string]int)
	for _, p := range event.PreviousProducts {
		previousQty[p.ProductID] += p.Quantity
	}

	currentQty := make(map[string]int)
	for _, p := range event.CurrentProducts {
		currentQty[p.ProductID] += p.Quantity
	}

	allProductIDs := make(map[string]struct{})
	for id := range previousQty {
		allProductIDs[id] = struct{}{}
	}
	for id := range currentQty {
		allProductIDs[id] = struct{}{}
	}

	for productID := range allProductIDs {
		prevQty := previousQty[productID]
		currQty := currentQty[productID]
		diff := currQty - prevQty

		if diff > 0 {
			if err := h.decreaseStockForProduct(ctx, productID, diff); err != nil {
				h.logger.Error("failed to decrease stock on order update",
					slog.String("product_id", productID),
					slog.Int("diff", diff),
					slog.String("error", err.Error()),
				)
			}
		} else if diff < 0 {
			if err := h.increaseStockForProduct(ctx, productID, -diff); err != nil {
				h.logger.Error("failed to increase stock on order update",
					slog.String("product_id", productID),
					slog.Int("diff", -diff),
					slog.String("error", err.Error()),
				)
			}
		}
	}

	return nil
}

// HandleOrderDeleted restores stock when an order is deleted (voided). It is the mirror
// image of HandleOrderCreated: every line the order still held is credited back to
// inventory (composite products expand to their ingredients), reversing the decrement the
// order applied. Cancelling or completing a line never returns stock, so the lines present
// at deletion time are exactly what is owed back. The lines ride on the event, so no read
// of the now soft-deleted order is needed.
func (h *StockEventHandler) HandleOrderDeleted(ctx context.Context, event dto.OrderDeletedEvent) error {
	h.logger.Info("handling order deleted event for stock",
		slog.String("open_bill_id", event.OpenBillID),
		slog.Int("product_count", len(event.Products)),
	)

	for _, item := range event.Products {
		if err := h.increaseStockForProduct(ctx, item.ProductID, item.Quantity); err != nil {
			h.logger.Error("failed to restore stock on order deletion",
				slog.String("product_id", item.ProductID),
				slog.Int("quantity", item.Quantity),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// HandlePurchaseEntryCreated increases stock when products arrive from suppliers.
func (h *StockEventHandler) HandlePurchaseEntryCreated(ctx context.Context, event dto.PurchaseEntryCreatedEvent) error {
	h.logger.Info("handling purchase entry created event for stock",
		slog.String("purchase_entry_id", event.PurchaseEntryID),
		slog.Int("item_count", len(event.Items)),
	)

	for _, item := range event.Items {
		change := int(item.Quantity.IntPart())
		if err := h.updateStock(ctx, item.ProductID, change); err != nil {
			h.logger.Error("failed to increase stock from purchase entry",
				slog.String("product_id", item.ProductID),
				slog.Int("quantity", change),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

func (h *StockEventHandler) decreaseStockForProduct(ctx context.Context, productID string, quantity int) error {
	product, err := h.productRepo.FindByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	if product.ProductType == dto.ProductTypeComposite {
		ingredients, err := h.productIngredientRepo.FindByCompositeProductID(ctx, productID)
		if err != nil {
			return fmt.Errorf("failed to get ingredients: %w", err)
		}
		if len(ingredients) == 0 {
			h.logger.Warn("composite product has no ingredients",
				slog.String("product_id", productID),
			)
			return nil
		}
		for _, ingredient := range ingredients {
			totalQty := ingredient.Quantity.Mul(decimal.NewFromInt(int64(quantity)))
			if err := h.updateStock(ctx, ingredient.IngredientProductID, -int(totalQty.IntPart())); err != nil {
				return fmt.Errorf("failed to decrease ingredient stock: %w", err)
			}
		}

		return nil
	}

	if err := h.updateStock(ctx, productID, -quantity); err != nil {
		return fmt.Errorf("failed to decrease stock: %w", err)
	}

	return nil
}

func (h *StockEventHandler) increaseStockForProduct(ctx context.Context, productID string, quantity int) error {
	product, err := h.productRepo.FindByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	if product.ProductType == dto.ProductTypeComposite {
		ingredients, err := h.productIngredientRepo.FindByCompositeProductID(ctx, productID)
		if err != nil {
			return fmt.Errorf("failed to get ingredients: %w", err)
		}
		if len(ingredients) == 0 {
			h.logger.Warn("composite product has no ingredients",
				slog.String("product_id", productID),
			)
			return nil
		}
		for _, ingredient := range ingredients {
			totalQty := ingredient.Quantity.Mul(decimal.NewFromInt(int64(quantity)))
			if err := h.updateStock(ctx, ingredient.IngredientProductID, int(totalQty.IntPart())); err != nil {
				return fmt.Errorf("failed to increase ingredient stock: %w", err)
			}
		}

		return nil
	}

	if err := h.updateStock(ctx, productID, quantity); err != nil {
		return fmt.Errorf("failed to increase stock: %w", err)
	}

	return nil
}

func (h *StockEventHandler) updateStock(ctx context.Context, productID string, change int) error {
	h.lockManager.Lock(productID)
	defer h.lockManager.Unlock(productID)

	product, prodErr := h.productRepo.FindByID(ctx, productID)
	if prodErr != nil {
		return fmt.Errorf("product not found: %w", prodErr)
	}

	existingStock, findErr := h.stockRepo.FindByProductID(ctx, productID)

	// Persist the stock write, its historic record, and the sync-outbox row that replicates
	// it to the cloud in one transaction (Option A). The per-product lock above serializes
	// the read-modify-write so concurrent in-process events can't lose an update.
	return h.unitOfWork.Do(ctx, func(ctx context.Context) error {
		var snapshot *dto.Stock
		var operation dto.SyncOperation

		if findErr != nil {
			now := time.Now()
			stock := &dto.Stock{
				ProductID:     productID,
				Version:       product.Version,
				Amount:        change,
				UnitOfMeasure: product.UnitOfMeasure,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if createErr := h.stockRepo.Create(ctx, stock); createErr != nil {
				return fmt.Errorf("failed to create stock: %w", createErr)
			}
			snapshot = stock
			operation = dto.SyncOperationCreate

			h.logger.Info("created new stock entry",
				slog.String("product_id", productID),
				slog.Int("initial_amount", change),
				slog.String("unit_of_measure", string(product.UnitOfMeasure)),
			)
		} else {
			newAmount := existingStock.Amount + change
			if err := h.stockRepo.UpdateAmount(ctx, productID, newAmount); err != nil {
				return fmt.Errorf("failed to update stock: %w", err)
			}
			snapshot = &dto.Stock{
				ProductID:     existingStock.ProductID,
				Version:       existingStock.Version,
				Amount:        newAmount,
				UnitOfMeasure: existingStock.UnitOfMeasure,
				CreatedAt:     existingStock.CreatedAt,
				UpdatedAt:     time.Now(),
			}
			operation = dto.SyncOperationUpdate

			h.logger.Info("updated stock",
				slog.String("product_id", productID),
				slog.Int("previous_amount", existingStock.Amount),
				slog.Int("change", change),
				slog.Int("new_amount", newAmount),
			)
		}

		// Unlike before, a failed historic write now rolls back the stock write (and the
		// outbox row) instead of being logged and swallowed.
		if err := h.stockRepo.CreateHistoricRecord(ctx, &dto.HistoricStock{
			ProductID:     productID,
			UnitOfMeasure: product.UnitOfMeasure,
			Change:        change,
			CreatedAt:     time.Now(),
		}); err != nil {
			return fmt.Errorf("failed to create historic stock record: %w", err)
		}

		return appendStockOutbox(ctx, h.outboxRepo, h.syncIdentity.NodeID, snapshot, operation)
	})
}
