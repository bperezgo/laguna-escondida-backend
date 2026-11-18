package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/gin-gonic/gin"
)

const notificationsDisabledCode = "NOTIFICATIONS_DISABLED"

// NotificationChannelHandler holds a nil service while the deployment has no notification key.
type NotificationChannelHandler struct {
	channels *service.NotificationChannelService
}

func NewNotificationChannelHandler(channels *service.NotificationChannelService) *NotificationChannelHandler {
	return &NotificationChannelHandler{channels: channels}
}

func (h *NotificationChannelHandler) ListHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	channels, err := h.channels.List(c.Request.Context())
	if err != nil {
		respondNotificationError(c, "listing notification channels", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"notification_channels": channels})
}

func (h *NotificationChannelHandler) GetHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	channel, err := h.channels.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondNotificationError(c, "getting notification channel", err)
		return
	}
	c.JSON(http.StatusOK, channel)
}

func (h *NotificationChannelHandler) CreateHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	var req dto.CreateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	channel, err := h.channels.Create(c.Request.Context(), &req)
	if err != nil {
		respondNotificationError(c, "creating notification channel", err)
		return
	}
	c.JSON(http.StatusCreated, channel)
}

func (h *NotificationChannelHandler) UpdateHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	var req dto.UpdateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	channel, err := h.channels.Update(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondNotificationError(c, "updating notification channel", err)
		return
	}
	c.JSON(http.StatusOK, channel)
}

func (h *NotificationChannelHandler) DeleteHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	if err := h.channels.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondNotificationError(c, "deleting notification channel", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notification channel deleted successfully"})
}

func (h *NotificationChannelHandler) TestHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	if err := h.channels.Test(c.Request.Context(), c.Param("id")); err != nil {
		respondNotificationError(c, "testing notification channel", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NotificationChannelHandler) disabled(c *gin.Context) bool {
	if h.channels != nil {
		return false
	}
	respondNotificationsDisabled(c)
	return true
}

func respondNotificationsDisabled(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "Notifications are not configured for this deployment",
		"code":  notificationsDisabledCode,
	})
}

func respondNotificationError(c *gin.Context, action string, err error) {
	log.Printf("Error %s: %v", action, err)
	var unreachable *domainError.NotificationChannelUnreachableError
	switch {
	case errors.As(err, &unreachable):
		body := gin.H{"error": "The destination refused the message or could not be reached", "code": "NOTIFICATION_CHANNEL_UNREACHABLE"}
		if unreachable.DestinationStatus != 0 {
			body["destination_status"] = unreachable.DestinationStatus
		}
		c.JSON(http.StatusBadGateway, body)
	case errors.Is(err, domainError.ErrNotificationChannelNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification channel not found"})
	case errors.Is(err, domainError.ErrNotificationRuleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification rule not found"})
	default:
		var baseErr *baseError.BaseError
		if errors.As(err, &baseErr) {
			body := gin.H{"error": baseErr.GetMessage(), "code": string(baseErr.GetCode())}
			if field, ok := baseErr.GetFieldValue().(string); ok && field != "" {
				body["field"] = field
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}
		RespondError(c, err)
	}
}
