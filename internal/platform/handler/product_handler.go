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

type ProductHandler struct {
	productService *service.ProductService
}

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

func (h *ProductHandler) CreateProductHandler(c *gin.Context) {
	var req dto.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	product, err := h.productService.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating product: %v", err)

		if errors.Is(err, domainError.ErrProductCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) UpdateProductHandler(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	var req dto.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	product, err := h.productService.UpdateProduct(c.Request.Context(), productID, &req)
	if err != nil {
		log.Printf("Error updating product: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) DeleteProductHandler(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.productService.DeleteProduct(c.Request.Context(), productID)
	if err != nil {
		log.Printf("Error deleting product: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductDeleteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ProductHandler) ListProductsHandler(c *gin.Context) {
	products, err := h.productService.ListProducts(c.Request.Context())
	if err != nil {
		log.Printf("Error listing products: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list products"})
		return
	}

	total := len(products)
	response := dto.ProductListResponse{
		Products: products,
		Total:    &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProductHandler) GetProductByIDHandler(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	product, err := h.productService.GetProductByID(c.Request.Context(), productID)
	if err != nil {
		log.Printf("Error getting product: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) ListCategoriesHandler(c *gin.Context) {
	categories, err := h.productService.ListCategories(c.Request.Context())
	if err != nil {
		log.Printf("Error listing categories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

func (h *ProductHandler) CreateProductResponsibilityHandler(c *gin.Context) {
	var req dto.CreateProductResponsibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	responsibility, err := h.productService.CreateProductResponsibility(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating product responsibility: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product responsibility"})
		return
	}

	c.JSON(http.StatusCreated, responsibility)
}
