package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/sse"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// businessDayRange returns [start, end) of "today" in the restaurant's local time
// (America/Bogota, fixed UTC-5, no DST) as UTC instants, for timestamptz comparison.
func businessDayRange() (time.Time, time.Time) {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("America/Bogota", -5*60*60)
	}
	nowLocal := time.Now().In(loc)
	startLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	return startLocal.UTC(), startLocal.Add(24 * time.Hour).UTC()
}

type SSEHandler struct {
	hub                *sse.Hub
	openBillProductHub *sse.OpenBillProductHub
	orderService       *service.OrderService
	logger             *zap.Logger
}

func NewSSEHandler(hub *sse.Hub, openBillProductHub *sse.OpenBillProductHub, orderService *service.OrderService, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		hub:                hub,
		openBillProductHub: openBillProductHub,
		orderService:       orderService,
		logger:             logger,
	}
}

func (h *SSEHandler) StreamCommandsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	client := sse.NewClient(area)
	h.hub.Register(client)

	h.logger.Info("SSE connection established",
		zap.String("handler", "StreamCommands"),
		zap.String("area", area),
		zap.String("client_ip", c.ClientIP()),
	)

	defer func() {
		h.hub.Unregister(client)
		h.logger.Info("SSE connection closed",
			zap.String("handler", "StreamCommands"),
			zap.String("area", area),
			zap.String("client_ip", c.ClientIP()),
		)
	}()

	ctx := c.Request.Context()

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := event.ToSSEFormat()
			if err != nil {
				h.logger.Error("Failed to format SSE event",
					zap.String("handler", "StreamCommands"),
					zap.String("area", area),
					zap.String("event_type", event.Type),
					zap.Error(err),
				)
				return true
			}
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				h.logger.Error("Failed to write SSE event",
					zap.String("handler", "StreamCommands"),
					zap.String("area", area),
					zap.String("event_type", event.Type),
					zap.Error(err),
				)
				return true
			}
			h.logger.Debug("SSE event sent",
				zap.String("handler", "StreamCommands"),
				zap.String("area", area),
				zap.String("event_type", event.Type),
			)
			return true
		case <-ctx.Done():
			h.logger.Info("SSE connection context done",
				zap.String("handler", "StreamCommands"),
				zap.String("area", area),
			)
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

// GetCompletedOpenBillProductsHandler returns today's fully-completed comandas for an
// area (read-only "Comandas Listas" review view). "Today" = local business day (Bogota).
func (h *SSEHandler) GetCompletedOpenBillProductsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	from, to := businessDayRange()

	ctx := c.Request.Context()
	products, err := h.orderService.GetCompletedOpenBillProductsByArea(ctx, area, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get completed open bill products"})
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

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	client := sse.NewOpenBillProductClient(area)
	h.openBillProductHub.Register(client)

	h.logger.Info("SSE connection established",
		zap.String("handler", "StreamOpenBillProducts"),
		zap.String("area", area),
		zap.String("client_ip", c.ClientIP()),
	)

	defer func() {
		h.openBillProductHub.Unregister(client)
		h.logger.Info("SSE connection closed",
			zap.String("handler", "StreamOpenBillProducts"),
			zap.String("area", area),
			zap.String("client_ip", c.ClientIP()),
		)
	}()

	ctx := c.Request.Context()

	pendingProducts, err := h.orderService.GetPendingOpenBillProductsByArea(ctx, area)
	pendingSent := false

	c.Stream(func(w io.Writer) bool {
		if !pendingSent {
			pendingSent = true
			if err == nil && len(pendingProducts) > 0 {
				h.logger.Info("Sending pending products on connection",
					zap.String("handler", "StreamOpenBillProducts"),
					zap.String("area", area),
					zap.Int("count", len(pendingProducts)),
				)
				for _, product := range pendingProducts {
					data, err := json.Marshal(product)
					if err != nil {
						h.logger.Error("Failed to marshal pending product",
							zap.String("handler", "StreamOpenBillProducts"),
							zap.String("area", area),
							zap.Error(err),
						)
						continue
					}
					_, err = fmt.Fprintf(w, "event: open_bill_product.created\ndata: %s\n\n", data)
					if err != nil {
						h.logger.Error("Failed to write pending product",
							zap.String("handler", "StreamOpenBillProducts"),
							zap.String("area", area),
							zap.Error(err),
						)
						continue
					}
				}
			}
			return true
		}

		select {
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := json.Marshal(event.Data)
			if err != nil {
				h.logger.Error("Failed to marshal SSE event",
					zap.String("handler", "StreamOpenBillProducts"),
					zap.String("area", area),
					zap.String("event_type", event.Type),
					zap.Error(err),
				)
				return true
			}
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				h.logger.Error("Failed to write SSE event",
					zap.String("handler", "StreamOpenBillProducts"),
					zap.String("area", area),
					zap.String("event_type", event.Type),
					zap.Error(err),
				)
				return true
			}
			h.logger.Debug("SSE event sent",
				zap.String("handler", "StreamOpenBillProducts"),
				zap.String("area", area),
				zap.String("event_type", event.Type),
			)
			return true
		case <-ctx.Done():
			h.logger.Info("SSE connection context done",
				zap.String("handler", "StreamOpenBillProducts"),
				zap.String("area", area),
			)
			return false
		}
	})
}
