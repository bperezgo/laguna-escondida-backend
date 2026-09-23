package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type StockHandler struct {
	stockService          *service.StockService
	reconciliationService *service.StockReconciliationService
}

func NewStockHandler(
	stockService *service.StockService,
	reconciliationService *service.StockReconciliationService,
) *StockHandler {
	return &StockHandler{
		stockService:          stockService,
		reconciliationService: reconciliationService,
	}
}

// GetStockSyncFreshnessHandler reports when the cloud last folded a movement replicated from
// the restaurant. A batch count taken while that number is old will double-subtract whatever
// the restaurant has not yet sent, so the counting screen reads this to warn first.
func (h *StockHandler) GetStockSyncFreshnessHandler(c *gin.Context) {
	freshness, err := h.reconciliationService.Freshness(c.Request.Context())
	if err != nil {
		log.Printf("Error reading stock sync freshness: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read stock sync freshness"})
		return
	}

	c.JSON(http.StatusOK, freshness)
}

// WriteStockOpeningBalancesHandler is the cutover step that makes on-hand explainable: it
// writes the one movement per product that closes the gap between the amount the cloud
// adopted and the movements it actually holds. Safe to re-run — the second run finds no gap.
func (h *StockHandler) WriteStockOpeningBalancesHandler(c *gin.Context) {
	report, err := h.reconciliationService.WriteOpeningBalances(c.Request.Context())
	if err != nil {
		log.Printf("Error writing stock opening balances: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write stock opening balances"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *StockHandler) CreateStockHandler(c *gin.Context) {
	var req dto.CreateStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	stock, err := h.stockService.CreateStock(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating stock: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, domainError.ErrStockAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Stock already exists for this product"})
			return
		}
		if errors.Is(err, domainError.ErrStockCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create stock"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, stock)
}

func (h *StockHandler) AddOrDecreaseStockHandler(c *gin.Context) {
	productID := c.Param("product_id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	var req dto.AddOrDecreaseStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.ProductID = productID

	err := h.stockService.AddOrDecreaseStock(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error updating stock: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, domainError.ErrStockNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stock not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductVersionMismatch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product version mismatch"})
			return
		}
		if errors.Is(err, domainError.ErrStockUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stock"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *StockHandler) DeleteStockHandler(c *gin.Context) {
	productID := c.Param("product_id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.stockService.DeleteStock(c.Request.Context(), productID)
	if err != nil {
		log.Printf("Error deleting stock: %v", err)

		if errors.Is(err, domainError.ErrStockNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stock not found"})
			return
		}
		if errors.Is(err, domainError.ErrStockDeleteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete stock"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *StockHandler) GetAllStocksHandler(c *gin.Context) {
	stocks, err := h.stockService.GetAllStocks(c.Request.Context())
	if err != nil {
		log.Printf("Error getting all stocks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stocks"})
		return
	}

	total := len(stocks)
	response := dto.StockListResponse{
		Stocks: stocks,
		Total:  &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *StockHandler) BulkStockCreationOrUpdatingHandler(c *gin.Context) {
	var req dto.BulkStockCreationOrUpdatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := h.stockService.BulkStockCreationOrUpdating(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error bulk creating/updating stocks: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more products not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductVersionMismatch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product version mismatch"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}
