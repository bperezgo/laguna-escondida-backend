package handler

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"slices"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService         *service.UserService
	organizationService *service.OrganizationService
	logger              *slog.Logger
	// billing is nil on an edge box, which has no billing.
	billing billingSummarizer
}

type billingSummarizer interface {
	Summary(ctx context.Context, orgID string) (*dto.BillingSummary, error)
}

// ShowBilling makes /auth/me carry, for an admin, what the subscription notice needs. Only the cloud calls it.
func (h *UserHandler) ShowBilling(billing billingSummarizer) {
	h.billing = billing
}

func NewUserHandler(userService *service.UserService, organizationService *service.OrganizationService, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		userService:         userService,
		organizationService: organizationService,
		logger:              logger,
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
		if errors.Is(err, domainError.ErrUserEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "Email is already in use"})
			return
		}
		if errors.Is(err, domainError.ErrEmailRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email is required for an admin, accountant, agent or courier"})
			return
		}
		if errors.Is(err, domainError.ErrOrganizationRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "organization is required"})
			return
		}
		if errors.Is(err, domainError.ErrOrganizationMismatch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "organization does not match this box's"})
			return
		}
		if errors.Is(err, domainError.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
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
	ctx := c.Request.Context()

	var req dto.SignInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WarnContext(ctx, "sign-in request decode failed", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	signInResponse, err := h.userService.SignIn(ctx, &req)
	if err != nil {
		if errors.Is(err, domainError.ErrTrialExpired) || errors.Is(err, domainError.ErrPaymentRequired) {
			h.logger.InfoContext(ctx, "sign-in refused: organization stopped", slog.String("email", req.Email), slog.String("reason", err.Error()))
			respondStopped(c, err)
			return
		}
		if errors.Is(err, domainError.ErrOrganizationPending) {
			h.logger.InfoContext(ctx, "sign-in refused: approval pending", slog.String("email", req.Email))
			respondPendingApproval(c, err)
			return
		}
		if errors.Is(err, domainError.ErrInvalidCredentials) {
			h.logger.WarnContext(ctx, "sign-in rejected",
				slog.String("email", req.Email),
				slog.String("username", req.Username),
				slog.String("reason", err.Error()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": invalidCredentialsMessage(h.userService.SignInMode())})
			return
		}
		if errors.Is(err, domainError.ErrSignInEmailRequired) || errors.Is(err, domainError.ErrSignInUsernameRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		h.logger.ErrorContext(ctx, "sign-in failed",
			slog.String("email", req.Email),
			slog.String("username", req.Username),
			slog.String("error", err.Error()),
		)
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, signInResponse)
}

// respondStopped answers an ended trial or a missed payment; an admin's body carries the setup
// token the recovery routes accept.
func respondStopped(c *gin.Context, err error) {
	body := trialExpiredBody()
	if errors.Is(err, domainError.ErrPaymentRequired) {
		body = paymentRequiredBody()
	}
	var stopped *domainError.StoppedError
	if errors.As(err, &stopped) && stopped.SetupToken != "" {
		body["setup_token"] = stopped.SetupToken
	}
	c.JSON(http.StatusForbidden, body)
}

func respondPendingApproval(c *gin.Context, err error) {
	var pending *domainError.PendingApprovalError
	if errors.As(err, &pending) && pending.SetupToken != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Approval pending", "code": "pending_approval", "setup_token": pending.SetupToken})
		return
	}
	c.JSON(http.StatusForbidden, pendingApprovalBody)
}

// Opaque per mode, so the response can't tell which emails, usernames or organizations exist.
func invalidCredentialsMessage(mode dto.SignInMode) string {
	if mode == dto.SignInModeCloud {
		return "Invalid email or password"
	}
	return "Invalid username or password"
}

func (h *UserHandler) SignInContextHandler(c *gin.Context) {
	ctx := c.Request.Context()
	signInContext, err := h.userService.SignInContext(ctx)
	if err != nil {
		h.logger.ErrorContext(ctx, "sign-in context failed", slog.String("error", err.Error()))
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, signInContext)
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

func (h *UserHandler) GeneratePasswordHandler(c *gin.Context) {
	generated, err := h.userService.GeneratePassword(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondUserError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, generated)
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
	case errors.Is(err, domainError.ErrUserEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Email is already in use"})
	case errors.Is(err, domainError.ErrEmailRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required for an admin, accountant, agent or courier"})
	case errors.Is(err, domainError.ErrUserHasEmail):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only a user without an email gets a generated password"})
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

	organization, err := h.organizationService.GetCurrent(c.Request.Context(), c.GetString("org_id"))
	if err != nil {
		log.Printf("Error getting current organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}
	currentUser.Organization = organization

	// Anyone else gets only what the trial's days-left notice uses: the organization's status and trial end.
	if h.billing != nil && slices.Contains(currentUser.Permissions, string(permissions.BillingManage)) {
		summary, err := h.billing.Summary(c.Request.Context(), c.GetString("org_id"))
		if err != nil {
			log.Printf("Error getting billing summary: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
			return
		}
		currentUser.Billing = summary
	}

	c.JSON(http.StatusOK, currentUser)
}
