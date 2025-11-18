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

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) CreateUserHandler(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userWithRoles, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating user: %v", err)

		if errors.Is(err, domainError.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}
		if errors.Is(err, domainError.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "One or more roles not found"})
			return
		}
		if errors.Is(err, domainError.ErrInvalidRoleIDs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role IDs provided"})
			return
		}
		if errors.Is(err, domainError.ErrUserCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, userWithRoles)
}

