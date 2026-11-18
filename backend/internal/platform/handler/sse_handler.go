package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/sse"

	"github.com/gin-gonic/gin"
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

// sseHeartbeatInterval is how often each stream emits an SSE comment line to keep
// bytes flowing on an otherwise idle connection. It must stay well under the Next.js
// proxy's undici bodyTimeout (300s), otherwise a quiet stream is mistaken for a dead
// one and torn down every ~5 minutes. 15s leaves a wide safety margin.
const sseHeartbeatInterval = 15 * time.Second

type SSEHandler struct {
	hub                *sse.Hub
	openBillProductHub *sse.OpenBillProductHub
	orderService       *service.OrderService
	logger             *slog.Logger
}

func NewSSEHandler(hub *sse.Hub, openBillProductHub *sse.OpenBillProductHub, orderService *service.OrderService, logger *slog.Logger) *SSEHandler {
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

	h.logger.InfoContext(c.Request.Context(), "SSE connection established",
		slog.String("handler", "StreamCommands"),
		slog.String("area", area),
		slog.String("client_ip", c.ClientIP()),
	)

	defer func() {
		h.hub.Unregister(client)
		h.logger.InfoContext(c.Request.Context(), "SSE connection closed",
			slog.String("handler", "StreamCommands"),
			slog.String("area", area),
			slog.String("client_ip", c.ClientIP()),
		)
	}()

	ctx := c.Request.Context()

	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := event.ToSSEFormat()
			if err != nil {
				h.logger.ErrorContext(ctx, "Failed to format SSE event",
					slog.String("handler", "StreamCommands"),
					slog.String("area", area),
					slog.String("event_type", event.Type),
					slog.Any("error", err),
				)
				return true
			}
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				h.logger.ErrorContext(ctx, "Failed to write SSE event",
					slog.String("handler", "StreamCommands"),
					slog.String("area", area),
					slog.String("event_type", event.Type),
					slog.Any("error", err),
				)
				return true
			}
			h.logger.DebugContext(ctx, "SSE event sent",
				slog.String("handler", "StreamCommands"),
				slog.String("area", area),
				slog.String("event_type", event.Type),
			)
			return true
		case <-heartbeat.C:
			// SSE comment line (starts with ':') — ignored by EventSource clients,
			// but keeps bytes flowing so the Next.js proxy / intermediaries don't
			// time out an idle stream. A write failure means the client is gone.
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				h.logger.InfoContext(ctx, "SSE heartbeat write failed; closing stream",
					slog.String("handler", "StreamCommands"),
					slog.String("area", area),
					slog.Any("error", err),
				)
				return false
			}
			return true
		case <-ctx.Done():
			h.logger.InfoContext(ctx, "SSE connection context done",
				slog.String("handler", "StreamCommands"),
				slog.String("area", area),
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

	h.logger.InfoContext(c.Request.Context(), "SSE connection established",
		slog.String("handler", "StreamOpenBillProducts"),
		slog.String("area", area),
		slog.String("client_ip", c.ClientIP()),
	)

	defer func() {
		h.openBillProductHub.Unregister(client)
		h.logger.InfoContext(c.Request.Context(), "SSE connection closed",
			slog.String("handler", "StreamOpenBillProducts"),
			slog.String("area", area),
			slog.String("client_ip", c.ClientIP()),
		)
	}()

	ctx := c.Request.Context()

	pendingProducts, err := h.orderService.GetPendingOpenBillProductsByArea(ctx, area)
	pendingSent := false

	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	c.Stream(func(w io.Writer) bool {
		if !pendingSent {
			pendingSent = true
			if err == nil && len(pendingProducts) > 0 {
				h.logger.InfoContext(ctx, "Sending pending products on connection",
					slog.String("handler", "StreamOpenBillProducts"),
					slog.String("area", area),
					slog.Int("count", len(pendingProducts)),
				)
				for _, product := range pendingProducts {
					data, err := json.Marshal(product)
					if err != nil {
						h.logger.ErrorContext(ctx, "Failed to marshal pending product",
							slog.String("handler", "StreamOpenBillProducts"),
							slog.String("area", area),
							slog.Any("error", err),
						)
						continue
					}
					_, err = fmt.Fprintf(w, "event: open_bill_product.created\ndata: %s\n\n", data)
					if err != nil {
						h.logger.ErrorContext(ctx, "Failed to write pending product",
							slog.String("handler", "StreamOpenBillProducts"),
							slog.String("area", area),
							slog.Any("error", err),
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
				h.logger.ErrorContext(ctx, "Failed to marshal SSE event",
					slog.String("handler", "StreamOpenBillProducts"),
					slog.String("area", area),
					slog.String("event_type", event.Type),
					slog.Any("error", err),
				)
				return true
			}
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				h.logger.ErrorContext(ctx, "Failed to write SSE event",
					slog.String("handler", "StreamOpenBillProducts"),
					slog.String("area", area),
					slog.String("event_type", event.Type),
					slog.Any("error", err),
				)
				return true
			}
			h.logger.DebugContext(ctx, "SSE event sent",
				slog.String("handler", "StreamOpenBillProducts"),
				slog.String("area", area),
				slog.String("event_type", event.Type),
			)
			return true
		case <-heartbeat.C:
			// SSE comment line (starts with ':') — ignored by EventSource clients,
			// but keeps bytes flowing so the Next.js proxy / intermediaries don't
			// time out an idle stream. A write failure means the client is gone.
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				h.logger.InfoContext(ctx, "SSE heartbeat write failed; closing stream",
					slog.String("handler", "StreamOpenBillProducts"),
					slog.String("area", area),
					slog.Any("error", err),
				)
				return false
			}
			return true
		case <-ctx.Done():
			h.logger.InfoContext(ctx, "SSE connection context done",
				slog.String("handler", "StreamOpenBillProducts"),
				slog.String("area", area),
			)
			return false
		}
	})
}
