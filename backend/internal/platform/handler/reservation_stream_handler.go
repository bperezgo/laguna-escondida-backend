package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/sse"

	"github.com/gin-gonic/gin"
)

type ReservationStreamHandler struct {
	hub       *sse.ReservationHub
	logger    *slog.Logger
	heartbeat time.Duration
}

func NewReservationStreamHandler(hub *sse.ReservationHub, logger *slog.Logger) *ReservationStreamHandler {
	return &ReservationStreamHandler{hub: hub, logger: logger, heartbeat: sseHeartbeatInterval}
}

// StreamHandler sends a `reservation` event with the whole reservation each time one of the
// caller's organization is created or changes state.
func (h *ReservationStreamHandler) StreamHandler(c *gin.Context) {
	ctx := c.Request.Context()
	organizationID, ok := tenant.OrgFrom(ctx)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No organization"})
		return
	}
	client := h.hub.Subscribe(organizationID)
	defer h.hub.Unsubscribe(client)

	// The first bytes open the stream for the browser, which reloads the day on every (re)connect.
	if _, err := fmt.Fprint(c.Writer, ": connected\n\n"); err != nil {
		return
	}
	c.Writer.Flush()

	heartbeat := time.NewTicker(h.heartbeat)
	defer heartbeat.Stop()
	c.Stream(func(w io.Writer) bool {
		select {
		case reservation, open := <-client.Events:
			if !open {
				return false
			}
			data, err := json.Marshal(reservation)
			if err != nil {
				h.logger.ErrorContext(ctx, "encoding a reservation for the stream", slog.String("reservation_id", reservation.ID), slog.Any("error", err))
				return true
			}
			_, err = fmt.Fprintf(w, "event: reservation\ndata: %s\n\n", data)
			return err == nil
		case <-heartbeat.C:
			_, err := fmt.Fprint(w, ": heartbeat\n\n")
			return err == nil
		case <-ctx.Done():
			return false
		}
	})
}
