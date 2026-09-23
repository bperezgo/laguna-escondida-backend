package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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
	emitter               ports.StockMovementEmitter
	logger                *slog.Logger
}

func NewStockEventHandler(
	stockRepo ports.StockRepository,
	productRepo ports.ProductRepository,
	productIngredientRepo ports.ProductIngredientRepository,
	lockManager *eventbus.ProductLockManager,
	unitOfWork ports.UnitOfWork,
	emitter ports.StockMovementEmitter,
	logger *slog.Logger,
) *StockEventHandler {
	return &StockEventHandler{
		stockRepo:             stockRepo,
		productRepo:           productRepo,
		productIngredientRepo: productIngredientRepo,
		lockManager:           lockManager,
		unitOfWork:            unitOfWork,
		emitter:               emitter,
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
		if err := h.decreaseStockForProduct(ctx, item.ProductID, item.Quantity, sideDishMap(item)); err != nil {
			h.logger.Error("failed to decrease stock",
				slog.String("product_id", item.ProductID),
				slog.Int("quantity", item.Quantity),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// sideDishMap indexes a line's resolved side-dish selections by ingredient product id, so
// expansion can override those ingredients' per-plate quantity with the amount actually served.
func sideDishMap(item dto.OrderCreatedEventProduct) map[string]int {
	if len(item.SideDishes) == 0 {
		return nil
	}
	m := make(map[string]int, len(item.SideDishes))
	for _, sd := range item.SideDishes {
		m[sd.IngredientProductID] = sd.Quantity
	}
	return m
}

// HandleOrderUpdated adjusts stock based on product changes between previous and current state.
func (h *StockEventHandler) HandleOrderUpdated(ctx context.Context, event dto.OrderUpdatedEvent) error {
	h.logger.Info("handling order updated event for stock",
		slog.String("open_bill_id", event.OpenBillID),
		slog.Int("previous_count", len(event.PreviousProducts)),
		slog.Int("current_count", len(event.CurrentProducts)),
	)

	// Diff previous vs current keyed by (product + resolved side-dish selection), not product
	// alone: editing only a side dish leaves the line quantity unchanged, so a product-only diff
	// would net to zero and miss the stock change (design D4). A changed side dish surfaces as a
	// restore of the previous key plus a consume of the current key.
	type lineConsumption struct {
		productID  string
		sideDishes map[string]int
		qty        int
	}
	byKey := make(map[string]*lineConsumption)
	accumulate := func(products []dto.OrderCreatedEventProduct, sign int) {
		for _, p := range products {
			key := consumptionKey(p)
			entry, ok := byKey[key]
			if !ok {
				entry = &lineConsumption{productID: p.ProductID, sideDishes: sideDishMap(p)}
				byKey[key] = entry
			}
			entry.qty += sign * p.Quantity
		}
	}
	accumulate(event.PreviousProducts, -1)
	accumulate(event.CurrentProducts, 1)

	for _, entry := range byKey {
		if entry.qty > 0 {
			if err := h.decreaseStockForProduct(ctx, entry.productID, entry.qty, entry.sideDishes); err != nil {
				h.logger.Error("failed to decrease stock on order update",
					slog.String("product_id", entry.productID),
					slog.Int("diff", entry.qty),
					slog.String("error", err.Error()),
				)
			}
		} else if entry.qty < 0 {
			if err := h.increaseStockForProduct(ctx, entry.productID, -entry.qty, entry.sideDishes); err != nil {
				h.logger.Error("failed to increase stock on order update",
					slog.String("product_id", entry.productID),
					slog.Int("diff", -entry.qty),
					slog.String("error", err.Error()),
				)
			}
		}
	}

	return nil
}

// consumptionKey canonically identifies a line's stock consumption: the product plus its
// resolved side-dish selection (sorted, so ordering never splits an otherwise-identical line).
// Lines with matching keys aggregate; a side-dish change yields a different key.
func consumptionKey(p dto.OrderCreatedEventProduct) string {
	if len(p.SideDishes) == 0 {
		return p.ProductID
	}
	pairs := make([]string, len(p.SideDishes))
	for i, sd := range p.SideDishes {
		pairs[i] = sd.IngredientProductID + ":" + strconv.Itoa(sd.Quantity)
	}
	sort.Strings(pairs)
	return p.ProductID + "|" + strings.Join(pairs, ",")
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
		if err := h.increaseStockForProduct(ctx, item.ProductID, item.Quantity, sideDishMap(item)); err != nil {
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
		if err := h.updateStock(ctx, item.ProductID, change, dto.StockMovementKindPurchase); err != nil {
			h.logger.Error("failed to increase stock from purchase entry",
				slog.String("product_id", item.ProductID),
				slog.Int("quantity", change),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// maxCompositeDepth bounds recipe expansion. It is a defensive backstop only: the visited-set
// cycle guard already terminates cyclic graphs; this caps pathologically deep (or cyclic, if
// the visited set were ever bypassed) recipes so a sale can never hang.
const maxCompositeDepth = 32

func (h *StockEventHandler) decreaseStockForProduct(ctx context.Context, productID string, quantity int, sideDishes map[string]int) error {
	return h.adjustStockForProduct(ctx, productID, decimal.NewFromInt(int64(quantity)), -1, sideDishes)
}

func (h *StockEventHandler) increaseStockForProduct(ctx context.Context, productID string, quantity int, sideDishes map[string]int) error {
	return h.adjustStockForProduct(ctx, productID, decimal.NewFromInt(int64(quantity)), 1, sideDishes)
}

// adjustStockForProduct applies a stock change for a single top-level order line. A COMPOSITE
// is expanded into its recipe (recursively, so composite-as-ingredient sub-recipes expand too)
// and only leaf ingredients touch a stock row; any other product type decrements/increments its
// own stock. sign is -1 to consume stock (sale/add) and +1 to restore it (void/remove).
// sideDishes overrides the per-plate quantity for the line's side-dish ingredients (design D4);
// it applies only to the ordered composite's direct ingredients (sub-recipes use their defaults).
func (h *StockEventHandler) adjustStockForProduct(ctx context.Context, productID string, quantity decimal.Decimal, sign int, sideDishes map[string]int) error {
	product, err := h.productRepo.FindByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	if product.ProductType == dto.ProductTypeComposite {
		return h.expandComposite(ctx, product, quantity, sign, make(map[string]struct{}), 0, sideDishes)
	}

	if err := h.updateStock(ctx, productID, sign*int(quantity.IntPart()), dto.StockMovementKindSale); err != nil {
		return fmt.Errorf("failed to adjust stock: %w", err)
	}

	return nil
}

// expandComposite walks a composite's recipe, multiplying quantity down the tree and writing
// stock only at leaf ingredients (a BOTH ingredient is a finished item, so it is consumed, not
// expanded). Quantity stays decimal along the path and is truncated to int only at the leaf
// write, so whole-unit recipes never compound rounding. path holds the composites currently on
// the expansion stack: revisiting one is a cycle, which is logged and skipped rather than failing
// the sale, so a cyclic graph always terminates.
func (h *StockEventHandler) expandComposite(ctx context.Context, composite *dto.Product, quantity decimal.Decimal, sign int, path map[string]struct{}, depth int, sideDishes map[string]int) error {
	if _, onPath := path[composite.ID]; onPath {
		h.logger.Warn("ingredient cycle detected while expanding composite; stopping expansion",
			slog.String("product_id", composite.ID),
		)
		return nil
	}
	if depth >= maxCompositeDepth {
		h.logger.Warn("composite recipe exceeds max depth; stopping expansion",
			slog.String("product_id", composite.ID),
			slog.Int("max_depth", maxCompositeDepth),
		)
		return nil
	}

	ingredients, err := h.productIngredientRepo.FindByCompositeProductID(ctx, composite.ID)
	if err != nil {
		return fmt.Errorf("failed to get ingredients: %w", err)
	}
	if len(ingredients) == 0 {
		h.logger.Warn("composite product has no ingredients",
			slog.String("product_id", composite.ID),
		)
		return nil
	}

	path[composite.ID] = struct{}{}
	defer delete(path, composite.ID)

	for _, ingredient := range ingredients {
		perUnit := ingredient.DefaultQuantity
		if selectedQty, ok := sideDishes[ingredient.IngredientProductID]; ok {
			perUnit = decimal.NewFromInt(int64(selectedQty))
		}
		childQty := perUnit.Mul(quantity)

		// A side dish set to 0 (or a default-0 offered alternative) consumes nothing: skip it
		// so it neither writes a no-op stock row nor emits a historic/outbox record.
		if childQty.IsZero() {
			continue
		}

		ingredientProduct, err := h.productRepo.FindByID(ctx, ingredient.IngredientProductID)
		if err != nil {
			return fmt.Errorf("ingredient product not found: %w", err)
		}

		if ingredientProduct.ProductType == dto.ProductTypeComposite {
			if err := h.expandComposite(ctx, ingredientProduct, childQty, sign, path, depth+1, nil); err != nil {
				return err
			}
			continue
		}

		if err := h.updateStockForProduct(ctx, ingredientProduct, sign*int(childQty.IntPart()), dto.StockMovementKindSale); err != nil {
			return fmt.Errorf("failed to adjust ingredient stock: %w", err)
		}
	}

	return nil
}

func (h *StockEventHandler) updateStock(ctx context.Context, productID string, change int, kind dto.StockMovementKind) error {
	product, prodErr := h.productRepo.FindByID(ctx, productID)
	if prodErr != nil {
		return fmt.Errorf("product not found: %w", prodErr)
	}

	return h.updateStockForProduct(ctx, product, change, kind)
}

// updateStockForProduct is updateStock for a product already loaded by the caller, so recipe
// expansion (which fetches each ingredient to decide expand-vs-write) does not fetch it twice.
func (h *StockEventHandler) updateStockForProduct(ctx context.Context, product *dto.Product, change int, kind dto.StockMovementKind) error {
	productID := product.ID

	h.lockManager.Lock(productID)
	defer h.lockManager.Unlock(productID)

	existingStock, findErr := h.stockRepo.FindByProductID(ctx, productID)

	// Persist the stock write and its movement — which the emitter queues for the node that
	// owns on-hand — in one transaction (Option A). The per-product lock above serializes
	// the read-modify-write so concurrent in-process events can't lose an update.
	return h.unitOfWork.Do(ctx, func(ctx context.Context) error {
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

		// Unlike before, a failed movement write now rolls back the stock write (and the
		// queued op) instead of being logged and swallowed.
		return recordMovement(ctx, h.stockRepo, h.emitter, &dto.HistoricStock{
			ProductID:     productID,
			UnitOfMeasure: product.UnitOfMeasure,
			Change:        change,
			Kind:          kind,
			CreatedAt:     time.Now(),
		})
	})
}
