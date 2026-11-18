package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type ReservationHandler struct {
	reservations *service.ReservationService
}

func NewReservationHandler(reservations *service.ReservationService) *ReservationHandler {
	return &ReservationHandler{reservations: reservations}
}

func (h *ReservationHandler) ListHandler(c *gin.Context) {
	result, err := h.reservations.List(c.Request.Context(), dto.ReservationListQuery{
		Date: c.Query("date"), Status: c.Query("status"), Phone: c.Query("phone"), From: c.Query("from"),
	})
	if err != nil {
		respondCRMError(c, "listing reservations", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReservationHandler) GetHandler(c *gin.Context) {
	result, err := h.reservations.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondCRMError(c, "reading reservation", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateHandler: the route lets reservations:create or :manage through; a WhatsApp request needs
// the first, and a staff-entered (confirmed) reservation the second.
func (h *ReservationHandler) CreateHandler(c *gin.Context) {
	var req dto.CreateReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	required := permissions.ReservationsManage
	if req.Source == dto.ReservationSourceWhatsApp {
		required = permissions.ReservationsCreate
	}
	if !permissions.HasPermission(callerRoles(c), required) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions", "required": string(required)})
		return
	}
	result, err := h.reservations.Create(c.Request.Context(), &req)
	if err != nil {
		respondCRMError(c, "creating reservation", err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *ReservationHandler) ConfirmHandler(c *gin.Context) {
	result, err := h.reservations.Confirm(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondCRMError(c, "confirming reservation", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReservationHandler) DeclineHandler(c *gin.Context) {
	var req dto.DeclineReservationRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}
	result, err := h.reservations.Decline(c.Request.Context(), c.Param("id"), req.Reason)
	if err != nil {
		respondCRMError(c, "declining reservation", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CancelHandler cancels as staff with reservations:manage, otherwise for the guest whose phone is
// in the body, which only finds that guest's own reservations.
func (h *ReservationHandler) CancelHandler(c *gin.Context) {
	var req dto.CancelReservationRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}
	var (
		result *dto.Reservation
		err    error
	)
	if permissions.HasPermission(callerRoles(c), permissions.ReservationsManage) {
		result, err = h.reservations.CancelByStaff(c.Request.Context(), c.Param("id"))
	} else {
		if req.CustomerPhone == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer_phone is required", "code": "VALIDATION_ERROR", "field": "customer_phone"})
			return
		}
		result, err = h.reservations.CancelByCustomer(c.Request.Context(), c.Param("id"), *req.CustomerPhone)
	}
	if err != nil {
		respondCRMError(c, "cancelling reservation", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReservationHandler) NoShowHandler(c *gin.Context) {
	result, err := h.reservations.MarkNoShow(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondCRMError(c, "marking reservation no-show", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type ContactHandler struct {
	contacts *service.ContactService
}

func NewContactHandler(contacts *service.ContactService) *ContactHandler {
	return &ContactHandler{contacts: contacts}
}

func (h *ContactHandler) ListHandler(c *gin.Context) {
	limit, limitErr := optionalInt(c.Query("limit"))
	offset, offsetErr := optionalInt(c.Query("offset"))
	if limitErr != nil || offsetErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit and offset must be whole numbers"})
		return
	}
	result, err := h.contacts.List(c.Request.Context(), dto.ContactListFilter{Search: c.Query("search"), Limit: limit, Offset: offset})
	if err != nil {
		respondCRMError(c, "listing contacts", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ContactHandler) GetHandler(c *gin.Context) {
	result, err := h.contacts.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondCRMError(c, "reading contact", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type FeedbackHandler struct {
	feedback *service.FeedbackService
}

func NewFeedbackHandler(feedback *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{feedback: feedback}
}

func (h *FeedbackHandler) ListHandler(c *gin.Context) {
	needsAttention := false
	if raw := c.Query("needs_attention"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "needs_attention must be true or false"})
			return
		}
		needsAttention = parsed
	}
	result, err := h.feedback.List(c.Request.Context(), dto.FeedbackListQuery{From: c.Query("from"), To: c.Query("to"), NeedsAttention: needsAttention})
	if err != nil {
		respondCRMError(c, "listing feedback", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *FeedbackHandler) RecordHandler(c *gin.Context) {
	var req dto.RecordFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	result, err := h.feedback.Record(c.Request.Context(), &req)
	if err != nil {
		respondCRMError(c, "recording feedback", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func callerRoles(c *gin.Context) []int {
	roleIDs, _ := c.Get("role_ids")
	roles, _ := roleIDs.([]int)
	return roles
}

func optionalInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func respondCRMError(c *gin.Context, action string, err error) {
	var validation *domainError.ValidationError
	var transition *domainError.ReservationTransitionError
	switch {
	case errors.As(err, &validation):
		c.JSON(http.StatusBadRequest, gin.H{"error": validation.Error(), "code": "VALIDATION_ERROR", "field": validation.Field})
	case errors.As(err, &transition):
		c.JSON(http.StatusConflict, gin.H{"error": transition.Error(), "code": "INVALID_RESERVATION_TRANSITION", "from": transition.From, "to": transition.To})
	case errors.Is(err, domainError.ErrReservationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "reservation not found", "code": "RESERVATION_NOT_FOUND"})
	case errors.Is(err, domainError.ErrContactNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found", "code": "CONTACT_NOT_FOUND"})
	case errors.Is(err, domainError.ErrNoVisitAwaitingFeedback):
		c.JSON(http.StatusNotFound, gin.H{"error": "no visit awaiting feedback", "code": "NO_VISIT_AWAITING_FEEDBACK"})
	default:
		log.Printf("Error %s: %v", action, err)
		RespondError(c, err)
	}
}
