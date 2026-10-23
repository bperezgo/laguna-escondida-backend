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

type OrganizationHandler struct {
	orgService *service.OrganizationService
}

func NewOrganizationHandler(orgService *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{orgService: orgService}
}

// Slug is bound only to reject it: without the field Gin would drop it and the caller would think it changed.
type updateOrganizationBody struct {
	dto.UpdateOrganizationRequest
	Slug *string `json:"slug"`
}

func (h *OrganizationHandler) CreateHandler(c *gin.Context) {
	var req dto.CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	org, err := h.orgService.Create(c.Request.Context(), &req)
	if err != nil {
		h.respondError(c, "creating organization", err)
		return
	}
	c.JSON(http.StatusCreated, org)
}

func (h *OrganizationHandler) ListHandler(c *gin.Context) {
	orgs, err := h.orgService.List(c.Request.Context())
	if err != nil {
		h.respondError(c, "listing organizations", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"organizations": orgs})
}

func (h *OrganizationHandler) GetHandler(c *gin.Context) {
	org, err := h.orgService.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, "getting organization", err)
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *OrganizationHandler) UpdateHandler(c *gin.Context) {
	var body updateOrganizationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if body.Slug != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slug: cannot be changed", "field": "slug"})
		return
	}
	org, err := h.orgService.Update(c.Request.Context(), c.Param("id"), &body.UpdateOrganizationRequest)
	if err != nil {
		h.respondError(c, "updating organization", err)
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *OrganizationHandler) SetEInvoicingHandler(c *gin.Context) {
	var req dto.SetEInvoicingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	settings, err := h.orgService.SetEInvoicing(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.respondError(c, "setting organization e-invoicing", err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *OrganizationHandler) respondError(c *gin.Context, action string, err error) {
	var validationErr *domainError.OrganizationValidationError
	switch {
	case errors.As(err, &validationErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error(), "field": validationErr.Field})
	case errors.Is(err, domainError.ErrOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
	case errors.Is(err, domainError.ErrOrganizationSlugTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Organization slug is already taken", "field": "slug"})
	default:
		log.Printf("Error %s: %v", action, err)
		RespondError(c, err)
	}
}
