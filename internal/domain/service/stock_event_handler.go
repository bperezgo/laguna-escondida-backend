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
	logger                *slog.Logger
}

func NewStockEventHandler(
	stockRepo ports.StockRepository,
	productRepo ports.ProductRepository,
	productIngredientRepo ports.ProductIngredientRepository,
	lockManager *eventbus.ProductLockManager,
	logger *slog.Logger,
) *StockEventHandler {
	return &StockEventHandler{
		stockRepo:             stockRepo,
		productRepo:           productRepo,
		productIngredientRepo: productIngredientRepo,
		lockManager:           lockManager,
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

// HandleOrderDeleted reverses stock changes when an order is deleted.
// Note: This requires the deleted order's products to be available.
// For now, we log a warning as OrderDeletedEvent may not contain product details.
func (h *StockEventHandler) HandleOrderDeleted(ctx context.Context, event dto.OrderDeletedEvent) error {
	h.logger.Warn("order deleted event received - stock reversal requires product info",
		slog.String("open_bill_id", event.OpenBillID),
	)

	// TODO: To properly reverse stock on deletion, we need either:
	// 1. Include products in OrderDeletedEvent before deletion
	// 2. Query from soft-deleted records
	// 3. Store order snapshots

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

	existingStock, err := h.stockRepo.FindByProductID(ctx, productID)
	if err != nil {
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

		h.logger.Info("updated stock",
			slog.String("product_id", productID),
			slog.Int("previous_amount", existingStock.Amount),
			slog.Int("change", change),
			slog.Int("new_amount", newAmount),
		)
	}

	if err := h.stockRepo.CreateHistoricRecord(ctx, &dto.HistoricStock{
		ProductID:     productID,
		UnitOfMeasure: product.UnitOfMeasure,
		Change:        change,
		CreatedAt:     time.Now(),
	}); err != nil {
		h.logger.Error("failed to create historic stock record",
			slog.String("product_id", productID),
			slog.Int("change", change),
			slog.String("error", err.Error()),
		)
	}

	return nil
}
