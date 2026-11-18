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

type SupplierHandler struct {
	supplierService *service.SupplierService
}

func NewSupplierHandler(supplierService *service.SupplierService) *SupplierHandler {
	return &SupplierHandler{
		supplierService: supplierService,
	}
}

func (h *SupplierHandler) CreateSupplierHandler(c *gin.Context) {
	var req dto.CreateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	supplier, err := h.supplierService.CreateSupplier(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating supplier: %v", err)

		if errors.Is(err, domainError.ErrSupplierCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create supplier"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, supplier)
}

func (h *SupplierHandler) UpdateSupplierHandler(c *gin.Context) {
	supplierID := c.Param("id")
	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}

	var req dto.UpdateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	supplier, err := h.supplierService.UpdateSupplier(c.Request.Context(), supplierID, &req)
	if err != nil {
		log.Printf("Error updating supplier: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrSupplierUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update supplier"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, supplier)
}

func (h *SupplierHandler) DeleteSupplierHandler(c *gin.Context) {
	supplierID := c.Param("id")
	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}

	err := h.supplierService.DeleteSupplier(c.Request.Context(), supplierID)
	if err != nil {
		log.Printf("Error deleting supplier: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrSupplierDeleteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete supplier"})
			return
		}
		RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SupplierHandler) ListSuppliersHandler(c *gin.Context) {
	suppliers, err := h.supplierService.ListSuppliers(c.Request.Context())
	if err != nil {
		log.Printf("Error listing suppliers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list suppliers"})
		return
	}

	total := len(suppliers)
	response := dto.SupplierListResponse{
		Suppliers: suppliers,
		Total:     &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *SupplierHandler) GetSupplierByIDHandler(c *gin.Context) {
	supplierID := c.Param("id")
	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}

	supplier, err := h.supplierService.GetSupplierByID(c.Request.Context(), supplierID)
	if err != nil {
		log.Printf("Error getting supplier: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, supplier)
}

func (h *SupplierHandler) AddProductToSupplierHandler(c *gin.Context) {
	supplierID := c.Param("id")
	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}

	var req dto.CreateSupplierCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	catalog, err := h.supplierService.AddProductToSupplier(c.Request.Context(), supplierID, &req)
	if err != nil {
		log.Printf("Error adding product to supplier: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, domainError.ErrSupplierCatalogAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Product already exists in supplier catalog"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add product to supplier"})
		return
	}

	c.JSON(http.StatusCreated, catalog)
}

func (h *SupplierHandler) UpdateSupplierCatalogHandler(c *gin.Context) {
	supplierID := c.Param("id")
	productID := c.Param("product_id")

	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	var req dto.UpdateSupplierCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	catalog, err := h.supplierService.UpdateSupplierCatalog(c.Request.Context(), supplierID, productID, &req)
	if err != nil {
		log.Printf("Error updating supplier catalog: %v", err)

		if errors.Is(err, domainError.ErrSupplierCatalogNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier catalog entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update supplier catalog"})
		return
	}

	c.JSON(http.StatusOK, catalog)
}

func (h *SupplierHandler) RemoveProductFromSupplierHandler(c *gin.Context) {
	supplierID := c.Param("id")
	productID := c.Param("product_id")

	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	err := h.supplierService.RemoveProductFromSupplier(c.Request.Context(), supplierID, productID)
	if err != nil {
		log.Printf("Error removing product from supplier: %v", err)

		if errors.Is(err, domainError.ErrSupplierCatalogNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier catalog entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove product from supplier"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SupplierHandler) GetSupplierProductsHandler(c *gin.Context) {
	supplierID := c.Param("id")
	if supplierID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Supplier ID is required"})
		return
	}

	products, err := h.supplierService.GetSupplierProducts(c.Request.Context(), supplierID)
	if err != nil {
		log.Printf("Error getting supplier products: %v", err)

		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get supplier products"})
		return
	}

	total := len(products)
	response := dto.SupplierCatalogListResponse{
		Items: products,
		Total: &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *SupplierHandler) GetProductSuppliersHandler(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	suppliers, err := h.supplierService.GetProductSuppliers(c.Request.Context(), productID)
	if err != nil {
		log.Printf("Error getting product suppliers: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get product suppliers"})
		return
	}

	total := len(suppliers)
	response := dto.ProductSuppliersListResponse{
		Items: suppliers,
		Total: &total,
	}

	c.JSON(http.StatusOK, response)
}
