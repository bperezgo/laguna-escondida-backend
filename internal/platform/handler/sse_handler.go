package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/sse"

	"github.com/gin-gonic/gin"
)

type SSEHandler struct {
	hub                *sse.Hub
	openBillProductHub *sse.OpenBillProductHub
	orderService       *service.OrderService
}

func NewSSEHandler(hub *sse.Hub, openBillProductHub *sse.OpenBillProductHub, orderService *service.OrderService) *SSEHandler {
	return &SSEHandler{
		hub:                hub,
		openBillProductHub: openBillProductHub,
		orderService:       orderService,
	}
}

func (h *SSEHandler) StreamCommandsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	client := sse.NewClient(area)
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	ctx := c.Request.Context()

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := event.ToSSEFormat()
			if err != nil {
				return true
			}
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				return true
			}
			return true
		case <-ctx.Done():
			return false
		}
	})
}

func (h *SSEHandler) GetPendingOpenBillProductsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	ctx := c.Request.Context()
	products, err := h.orderService.GetPendingOpenBillProductsByArea(ctx, area)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pending open bill products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"products": products})
}

func (h *SSEHandler) StreamOpenBillProductsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	client := sse.NewOpenBillProductClient(area)
	h.openBillProductHub.Register(client)
	defer h.openBillProductHub.Unregister(client)

	ctx := c.Request.Context()

	pendingProducts, err := h.orderService.GetPendingOpenBillProductsByArea(ctx, area)
	if err == nil && len(pendingProducts) > 0 {
		for _, product := range pendingProducts {
			data, err := json.Marshal(product)
			if err != nil {
				continue
			}
			_, err = fmt.Fprintf(c.Writer, "event: open_bill_product.created\ndata: %s\n\n", data)
			if err != nil {
				continue
			}
			c.Writer.Flush()
		}
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := json.Marshal(event.Data)
			if err != nil {
				return true
			}
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				return true
			}
			return true
		case <-ctx.Done():
			return false
		}
	})
}
