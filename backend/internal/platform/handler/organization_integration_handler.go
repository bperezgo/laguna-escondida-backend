package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
)

const integrationsUnavailableCode = "INTEGRATIONS_UNAVAILABLE"

type OrganizationIntegrationHandler struct {
	integrations *service.OrganizationIntegrationService
}

func NewOrganizationIntegrationHandler(integrations *service.OrganizationIntegrationService) *OrganizationIntegrationHandler {
	return &OrganizationIntegrationHandler{integrations: integrations}
}

// The platform routes name the organization in the path; the staff (Own*) routes act on the token's.
func (h *OrganizationIntegrationHandler) ListHandler(c *gin.Context)   { h.list(c, c.Param("id")) }
func (h *OrganizationIntegrationHandler) GetHandler(c *gin.Context)    { h.get(c, c.Param("id")) }
func (h *OrganizationIntegrationHandler) PutHandler(c *gin.Context)    { h.put(c, c.Param("id")) }
func (h *OrganizationIntegrationHandler) DeleteHandler(c *gin.Context) { h.delete(c, c.Param("id")) }
func (h *OrganizationIntegrationHandler) GenerateSecretHandler(c *gin.Context) {
	h.generateSecret(c, c.Param("id"))
}

func (h *OrganizationIntegrationHandler) OwnListHandler(c *gin.Context) {
	if orgID, ok := tokenOrganization(c); ok {
		h.list(c, orgID)
	}
}

func (h *OrganizationIntegrationHandler) OwnGetHandler(c *gin.Context) {
	if orgID, ok := tokenOrganization(c); ok {
		h.get(c, orgID)
	}
}

func (h *OrganizationIntegrationHandler) OwnPutHandler(c *gin.Context) {
	if orgID, ok := tokenOrganization(c); ok {
		h.put(c, orgID)
	}
}

func (h *OrganizationIntegrationHandler) OwnDeleteHandler(c *gin.Context) {
	if orgID, ok := tokenOrganization(c); ok {
		h.delete(c, orgID)
	}
}

func (h *OrganizationIntegrationHandler) OwnGenerateSecretHandler(c *gin.Context) {
	if orgID, ok := tokenOrganization(c); ok {
		h.generateSecret(c, orgID)
	}
}

func tokenOrganization(c *gin.Context) (string, bool) {
	orgID, err := tenant.RequireOrg(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return "", false
	}
	return orgID, true
}

func (h *OrganizationIntegrationHandler) list(c *gin.Context, orgID string) {
	list, err := h.integrations.List(c.Request.Context(), orgID)
	if err != nil {
		respondIntegrationError(c, "listing integrations", err)
		return
	}
	c.JSON(http.StatusOK, dto.OrganizationIntegrationListResponse{Integrations: list})
}

func (h *OrganizationIntegrationHandler) get(c *gin.Context, orgID string) {
	integration, err := h.integrations.Get(c.Request.Context(), orgID, c.Param("provider"))
	if err != nil {
		respondIntegrationError(c, "reading integration", err)
		return
	}
	c.JSON(http.StatusOK, integration)
}

func (h *OrganizationIntegrationHandler) put(c *gin.Context, orgID string) {
	var (
		integration *dto.OrganizationIntegration
		err         error
	)
	switch c.Param("provider") {
	case string(dto.IntegrationProviderWompi):
		var creds dto.WompiCredentials
		if !bindIntegrationBody(c, &creds) {
			return
		}
		integration, err = h.integrations.SetWompi(c.Request.Context(), orgID, creds)
	case string(dto.IntegrationProviderMarketingApp):
		var creds dto.MarketingAppCredentials
		if !bindIntegrationBody(c, &creds) {
			return
		}
		integration, err = h.integrations.SetMarketingApp(c.Request.Context(), orgID, creds)
	default:
		err = domainError.ErrIntegrationProviderUnknown
	}
	if err != nil {
		respondIntegrationError(c, "setting integration", err)
		return
	}
	c.JSON(http.StatusOK, integration)
}

func bindIntegrationBody[T dto.WompiCredentials | dto.MarketingAppCredentials](c *gin.Context, creds *T) bool {
	if err := c.ShouldBindJSON(creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return false
	}
	return true
}

// generateSecret answers with the new secret this once; no other response carries it.
func (h *OrganizationIntegrationHandler) generateSecret(c *gin.Context, orgID string) {
	generated, err := h.integrations.GenerateMarketingAppSecret(c.Request.Context(), orgID, c.Param("provider"))
	if err != nil {
		respondIntegrationError(c, "generating integration secret", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, generated)
}

func (h *OrganizationIntegrationHandler) delete(c *gin.Context, orgID string) {
	if err := h.integrations.Delete(c.Request.Context(), orgID, c.Param("provider")); err != nil {
		respondIntegrationError(c, "deleting integration", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// respondIntegrationError never echoes the request: validation errors name the field, not the value.
func respondIntegrationError(c *gin.Context, action string, err error) {
	var validation *domainError.IntegrationValidationError
	switch {
	case errors.As(err, &validation):
		body := gin.H{"error": validation.Error()}
		if validation.Field != "" {
			body["field"] = validation.Field
		}
		c.JSON(http.StatusBadRequest, body)
	case errors.Is(err, domainError.ErrIntegrationsUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Integrations are not configured for this deployment", "code": integrationsUnavailableCode,
		})
	case errors.Is(err, domainError.ErrOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
	case errors.Is(err, domainError.ErrIntegrationProviderUnknown):
		c.JSON(http.StatusNotFound, gin.H{"error": "Unknown integration provider"})
	case errors.Is(err, domainError.ErrIntegrationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Integration not found"})
	default:
		log.Printf("Error %s: %v", action, err)
		RespondError(c, err)
	}
}
