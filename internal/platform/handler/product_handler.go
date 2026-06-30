package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

// parseProductTypes parses a comma-separated product_type query param
// (e.g. "SELLABLE,BOTH") into a slice of product types. Empty entries are
// dropped and surrounding whitespace is trimmed. Unknown values are passed
// through and simply match no products.
func parseProductTypes(raw string) []dto.ProductType {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	productTypes := make([]dto.ProductType, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		productTypes = append(productTypes, dto.ProductType(value))
	}

	return productTypes
}

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
		RespondError(c, err)
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
		RespondError(c, err)
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
		RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ProductHandler) ListProductsHandler(c *gin.Context) {
	filter := dto.ListProductsRequest{
		ProductTypes: parseProductTypes(c.Query("product_type")),
	}

	products, err := h.productService.ListProducts(c.Request.Context(), filter)
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
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) BulkCreateProductsHandler(c *gin.Context) {
	var req dto.BulkCreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	response, err := h.productService.BulkCreateProducts(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error bulk creating products: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductCreationFailed) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
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

func (h *ProductHandler) UpdateProductResponsibilityHandler(c *gin.Context) {
	responsibilityID := c.Param("id")
	if responsibilityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Responsibility ID is required"})
		return
	}

	var req dto.UpdateProductResponsibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	responsibility, err := h.productService.UpdateProductResponsibility(c.Request.Context(), responsibilityID, &req)
	if err != nil {
		log.Printf("Error updating product responsibility: %v", err)

		if errors.Is(err, domainError.ErrProductResponsibilityNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product responsibility not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductResponsibilityUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product responsibility"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, responsibility)
}

func (h *ProductHandler) DeleteProductResponsibilityHandler(c *gin.Context) {
	responsibilityID := c.Param("id")
	if responsibilityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Responsibility ID is required"})
		return
	}

	err := h.productService.DeleteProductResponsibility(c.Request.Context(), responsibilityID)
	if err != nil {
		log.Printf("Error deleting product responsibility: %v", err)

		if errors.Is(err, domainError.ErrProductResponsibilityDeleteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product responsibility"})
			return
		}
		RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ProductHandler) GetProductResponsibilityByIDHandler(c *gin.Context) {
	responsibilityID := c.Param("id")
	if responsibilityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Responsibility ID is required"})
		return
	}

	responsibility, err := h.productService.GetProductResponsibilityByID(c.Request.Context(), responsibilityID)
	if err != nil {
		log.Printf("Error getting product responsibility: %v", err)

		if errors.Is(err, domainError.ErrProductResponsibilityNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product responsibility not found"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, responsibility)
}
