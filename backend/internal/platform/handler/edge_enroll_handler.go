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

type EdgeEnrollHandler struct {
	nodes *service.NodeService
}

func NewEdgeEnrollHandler(nodes *service.NodeService) *EdgeEnrollHandler {
	return &EdgeEnrollHandler{nodes: nodes}
}

func (h *EdgeEnrollHandler) EnrollHandler(c *gin.Context) {
	orgID, ok := staffOrganization(c)
	if !ok {
		return
	}
	var req dto.EnrollEdgeBoxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	enrolled, err := h.nodes.Enroll(c.Request.Context(), orgID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, enrolled)
}

func (h *EdgeEnrollHandler) respondError(c *gin.Context, err error) {
	respondEnrollError(c, err)
}

func respondEnrollError(c *gin.Context, err error) {
	var conflict *domainError.BoxAlreadyEnrolledError
	switch {
	case errors.Is(err, domainError.ErrNodeNameRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "field": "name"})
	case errors.As(err, &conflict):
		c.JSON(http.StatusConflict, gin.H{
			"error": conflict.Error(),
			"code":  "box_already_enrolled",
			"active_box": dto.ActiveBox{
				Name: conflict.Name, EnrolledAt: conflict.EnrolledAt, LastSeenAt: conflict.LastSeenAt,
			},
		})
	case errors.Is(err, domainError.ErrBoxAlreadyEnrolled):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "box_already_enrolled", "active_box": nil})
	default:
		log.Printf("Error enrolling box: %v", err)
		RespondError(c, err)
	}
}
