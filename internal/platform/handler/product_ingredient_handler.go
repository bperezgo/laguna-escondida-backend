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

type ProductIngredientHandler struct {
	productIngredientService *service.ProductIngredientService
}

func NewProductIngredientHandler(productIngredientService *service.ProductIngredientService) *ProductIngredientHandler {
	return &ProductIngredientHandler{
		productIngredientService: productIngredientService,
	}
}

func (h *ProductIngredientHandler) AddIngredientHandler(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	var req dto.AddIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ingredient, err := h.productIngredientService.AddIngredient(c.Request.Context(), productID, &req)
	if err != nil {
		log.Printf("Error adding ingredient: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductNotComposite) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product is not a composite product"})
			return
		}
		if errors.Is(err, domainError.ErrIngredientCannotBeSelf) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A product cannot be an ingredient of itself"})
			return
		}
		if errors.Is(err, domainError.ErrProductIngredientAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Ingredient already exists for this product"})
			return
		}
		if errors.Is(err, domainError.ErrInvalidIngredientQuantity) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domainError.ErrProductIngredientCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add ingredient"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, ingredient)
}

func (h *ProductIngredientHandler) UpdateIngredientHandler(c *gin.Context) {
	productID := c.Param("id")
	ingredientID := c.Param("ingredient_id")

	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}
	if ingredientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ingredient ID is required"})
		return
	}

	var req dto.UpdateIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ingredient, err := h.productIngredientService.UpdateIngredient(c.Request.Context(), productID, ingredientID, &req)
	if err != nil {
		log.Printf("Error updating ingredient: %v", err)

		if errors.Is(err, domainError.ErrProductIngredientNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ingredient not found"})
			return
		}
		if errors.Is(err, domainError.ErrInvalidIngredientQuantity) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domainError.ErrProductIngredientUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ingredient"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, ingredient)
}

func (h *ProductIngredientHandler) RemoveIngredientHandler(c *gin.Context) {
	productID := c.Param("id")
	ingredientID := c.Param("ingredient_id")

	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}
	if ingredientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ingredient ID is required"})
		return
	}

	err := h.productIngredientService.RemoveIngredient(c.Request.Context(), productID, ingredientID)
	if err != nil {
		log.Printf("Error removing ingredient: %v", err)

		if errors.Is(err, domainError.ErrProductIngredientNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ingredient not found"})
			return
		}
		if errors.Is(err, domainError.ErrProductIngredientDeleteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove ingredient"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ProductIngredientHandler) GetIngredientsHandler(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	ingredients, err := h.productIngredientService.GetIngredients(c.Request.Context(), productID)
	if err != nil {
		log.Printf("Error getting ingredients: %v", err)

		if errors.Is(err, domainError.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ingredients"})
		return
	}

	response := dto.ProductIngredientListResponse{
		Ingredients: ingredients,
	}

	c.JSON(http.StatusOK, response)
}
