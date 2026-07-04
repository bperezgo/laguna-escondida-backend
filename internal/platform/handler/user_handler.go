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

func (h *UserHandler) ListUsersHandler(c *gin.Context) {
	users, err := h.userService.ListUsers(c.Request.Context())
	if err != nil {
		log.Printf("Error listing users: %v", err)
		h.respondUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, users)
}

func (h *UserHandler) GetUserHandler(c *gin.Context) {
	userWithRoles, err := h.userService.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		log.Printf("Error getting user: %v", err)
		h.respondUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, userWithRoles)
}

func (h *UserHandler) UpdateUserHandler(c *gin.Context) {
	actingUserID, ok := getActingUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userWithRoles, err := h.userService.UpdateUser(c.Request.Context(), actingUserID, c.Param("id"), &req)
	if err != nil {
		log.Printf("Error updating user: %v", err)
		h.respondUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, userWithRoles)
}

func (h *UserHandler) ResetPasswordHandler(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.userService.ResetPassword(c.Request.Context(), c.Param("id"), req.Password); err != nil {
		log.Printf("Error resetting password: %v", err)
		h.respondUserError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) DeleteUserHandler(c *gin.Context) {
	actingUserID, ok := getActingUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if err := h.userService.DeleteUser(c.Request.Context(), actingUserID, c.Param("id")); err != nil {
		log.Printf("Error deleting user: %v", err)
		h.respondUserError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) ListRolesHandler(c *gin.Context) {
	roles, err := h.userService.ListRoles(c.Request.Context())
	if err != nil {
		log.Printf("Error listing roles: %v", err)
		h.respondUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, roles)
}

// respondUserError maps user-domain sentinel errors to HTTP status codes,
// falling back to RespondError for aggregate validation / unknown errors.
func (h *UserHandler) respondUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainError.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
	case errors.Is(err, domainError.ErrUserAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
	case errors.Is(err, domainError.ErrRoleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "One or more roles not found"})
	case errors.Is(err, domainError.ErrInvalidRoleIDs):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role IDs provided"})
	case errors.Is(err, domainError.ErrCannotDeleteSelf):
		c.JSON(http.StatusConflict, gin.H{"error": "You cannot delete your own user"})
	case errors.Is(err, domainError.ErrCannotDeactivateSelf):
		c.JSON(http.StatusConflict, gin.H{"error": "You cannot deactivate your own user"})
	case errors.Is(err, domainError.ErrCannotRemoveLastAdmin):
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot remove the last active admin"})
	case errors.Is(err, domainError.ErrUserCreationFailed):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
	default:
		RespondError(c, err)
	}
}

func getActingUserID(c *gin.Context) (string, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
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
