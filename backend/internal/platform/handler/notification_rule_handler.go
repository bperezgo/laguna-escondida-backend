package handler

import (
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type notificationRuleResponse struct {
	ID                  string                     `json:"id"`
	ChannelID           string                     `json:"channel_id"`
	Event               dto.NotificationEventType  `json:"event"`
	Conditions          dto.NotificationConditions `json:"conditions"`
	TitlePrefix         string                     `json:"title_prefix"`
	ExpiresAfterMinutes int                        `json:"expires_after_minutes"`
	Enabled             bool                       `json:"enabled"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
}

func newNotificationRuleResponse(r *dto.NotificationRule) notificationRuleResponse {
	return notificationRuleResponse{
		ID: r.ID, ChannelID: r.ChannelID, Event: r.Event, Conditions: r.Conditions, TitlePrefix: r.TitlePrefix,
		ExpiresAfterMinutes: r.ExpiresAfterMinutes, Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// NotificationRuleHandler holds a nil service while the deployment has no notification key.
type NotificationRuleHandler struct {
	rules *service.NotificationRuleService
}

func NewNotificationRuleHandler(rules *service.NotificationRuleService) *NotificationRuleHandler {
	return &NotificationRuleHandler{rules: rules}
}

func (h *NotificationRuleHandler) ListHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	rules, err := h.rules.List(c.Request.Context(), c.Query("channel_id"))
	if err != nil {
		respondNotificationError(c, "listing notification rules", err)
		return
	}
	out := make([]notificationRuleResponse, 0, len(rules))
	for _, r := range rules {
		out = append(out, newNotificationRuleResponse(r))
	}
	c.JSON(http.StatusOK, gin.H{"notification_rules": out})
}

func (h *NotificationRuleHandler) GetHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	rule, err := h.rules.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondNotificationError(c, "getting notification rule", err)
		return
	}
	c.JSON(http.StatusOK, newNotificationRuleResponse(rule))
}

func (h *NotificationRuleHandler) CreateHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	var req dto.CreateNotificationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	rule, err := h.rules.Create(c.Request.Context(), &req)
	if err != nil {
		respondNotificationError(c, "creating notification rule", err)
		return
	}
	c.JSON(http.StatusCreated, newNotificationRuleResponse(rule))
}

func (h *NotificationRuleHandler) UpdateHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	var req dto.UpdateNotificationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	rule, err := h.rules.Update(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondNotificationError(c, "updating notification rule", err)
		return
	}
	c.JSON(http.StatusOK, newNotificationRuleResponse(rule))
}

func (h *NotificationRuleHandler) DeleteHandler(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	if err := h.rules.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondNotificationError(c, "deleting notification rule", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notification rule deleted successfully"})
}

func (h *NotificationRuleHandler) disabled(c *gin.Context) bool {
	if h.rules != nil {
		return false
	}
	respondNotificationsDisabled(c)
	return true
}
