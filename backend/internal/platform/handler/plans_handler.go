package handler

import (
	"net/http"

	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type PlansHandler struct{}

func NewPlansHandler() *PlansHandler { return &PlansHandler{} }

func (h *PlansHandler) ListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, service.PlanPrices())
}
