package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

const maxLeadBodyBytes = 16 << 10

type LeadHandler struct {
	leads *service.LeadService
}

func NewLeadHandler(leads *service.LeadService) *LeadHandler {
	return &LeadHandler{leads: leads}
}

// SubmitHandler never echoes a stored field or id: the caller is anonymous.
func (h *LeadHandler) SubmitHandler(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLeadBodyBytes)
	var req dto.SubmitLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request body too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := h.leads.Submit(c.Request.Context(), &req); err != nil {
		h.respondError(c, "submitting lead", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "received"})
}

func (h *LeadHandler) ListHandler(c *gin.Context) {
	filter := dto.ListLeadsFilter{Status: c.Query("status")}
	var err error
	if filter.Page, err = queryInt(c, "page"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page: must be a number", "field": "page"})
		return
	}
	if filter.PageSize, err = queryInt(c, "page_size"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page_size: must be a number", "field": "page_size"})
		return
	}
	page, err := h.leads.List(c.Request.Context(), filter)
	if err != nil {
		h.respondError(c, "listing leads", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *LeadHandler) GetHandler(c *gin.Context) {
	lead, err := h.leads.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, "getting lead", err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

func (h *LeadHandler) UpdateHandler(c *gin.Context) {
	var req dto.UpdateLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	lead, err := h.leads.Update(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.respondError(c, "updating lead", err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

func (h *LeadHandler) DeleteHandler(c *gin.Context) {
	if err := h.leads.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.respondError(c, "deleting lead", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *LeadHandler) respondError(c *gin.Context, action string, err error) {
	var validationErr *domainError.ValidationError
	switch {
	case errors.As(err, &validationErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error(), "field": validationErr.Field})
	case errors.Is(err, domainError.ErrLeadNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Lead not found"})
	case errors.Is(err, domainError.ErrLeadRateLimited):
		c.Header("Retry-After", strconv.Itoa(int(service.LeadCapRetryAfter.Seconds())))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
	default:
		log.Printf("Error %s: %v", action, err)
		RespondError(c, err)
	}
}

func queryInt(c *gin.Context, name string) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
