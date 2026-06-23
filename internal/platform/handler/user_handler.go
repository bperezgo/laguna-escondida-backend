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
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, userWithRoles)
}

func (h *UserHandler) SignInHandler(c *gin.Context) {
	var req dto.SignInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	signInResponse, err := h.userService.SignIn(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error signing in: %v", err)

		if errors.Is(err, domainError.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, signInResponse)
}

func (h *UserHandler) GetCurrentUserHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
		return
	}

	currentUser, err := h.userService.GetCurrentUser(c.Request.Context(), userIDStr)
	if err != nil {
		log.Printf("Error getting current user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	c.JSON(http.StatusOK, currentUser)
}
