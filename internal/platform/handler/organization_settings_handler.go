package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type OrganizationSettingsHandler struct {
	settingsService *service.OrganizationSettingsService
}

func NewOrganizationSettingsHandler(settingsService *service.OrganizationSettingsService) *OrganizationSettingsHandler {
	return &OrganizationSettingsHandler{settingsService: settingsService}
}

func (h *OrganizationSettingsHandler) GetSettingsHandler(c *gin.Context) {
	settings, err := h.settingsService.Get(c.Request.Context())
	if err != nil {
		h.respondError(c, "reading organization settings", err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// PatchSettingsHandler checks permissions per field: the route only requires ordering:read, and
// a body that touches the pause and the hours needs both grants or changes nothing.
func (h *OrganizationSettingsHandler) PatchSettingsHandler(c *gin.Context) {
	var req dto.UpdateOrganizationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	roleIDs, _ := c.Get("role_ids")
	roles, _ := roleIDs.([]int)
	for _, required := range h.settingsService.RequiredPermissions(&req) {
		if !permissions.HasPermission(roles, required) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions", "required": string(required)})
			return
		}
	}

	settings, err := h.settingsService.Update(c.Request.Context(), &req)
	if err != nil {
		h.respondError(c, "updating organization settings", err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *OrganizationSettingsHandler) respondError(c *gin.Context, action string, err error) {
	log.Printf("Error %s: %v", action, err)
	var validation *domainError.OrderingSettingsValidationError
	if errors.As(err, &validation) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": validation.Error(), "code": "INVALID_ORDERING_SETTINGS", "field": validation.Field,
		})
		return
	}
	if errors.Is(err, domainError.ErrOrganizationNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}
	RespondError(c, err)
}
